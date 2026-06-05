package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	"golang.org/x/sync/errgroup"

	"github.com/xbpk3t/docs-alfred/service/wiki"
)

// errNotRetriable is returned when a URL was processed but did not produce a
// valid classification (e.g. AI couldn't assign a topic). The entry is flushed
// from the inbox so it is not retried.
var errNotRetriable = errors.New("not retriable")

type urlResult struct {
	err       error
	url       string
	lineIndex int
	success   bool
}

type inboxConfig struct {
	concurrency   int
	perURLTimeout time.Duration
	maxRetries    uint
}

func runInbox(cfg *Config) error {
	wikiRoot := resolveWikiRoot(cfg)

	if _, err := os.Stat(wikiRoot); os.IsNotExist(err) {
		return fmt.Errorf("wiki root not found: %s", wikiRoot)
	}

	inboxPath := filepath.Join(wikiRoot, "inbox.md")
	if _, err := os.Stat(inboxPath); os.IsNotExist(err) {
		return fmt.Errorf("inbox file not found: %s", inboxPath)
	}

	aiCfg := newAIConfig(cfg)
	classifier := wiki.NewClassifier(aiCfg, wikiRoot, cfg.Wiki.GhTopicsURL)
	fetcher := wiki.NewFetcher()

	slog.Info("Processing wiki inbox", "path", inboxPath)

	entries, err := wiki.ParseInbox(inboxPath)
	if err != nil {
		return fmt.Errorf("parse inbox: %w", err)
	}

	if len(entries) == 0 {
		slog.Info("No URLs found in inbox")

		return nil
	}

	slog.Info("Found URLs in inbox", "count", len(entries))

	ic := resolveInboxConfig(cfg)
	results := runInboxEntries(cfg, wikiRoot, classifier, fetcher, entries, ic)

	return flushInboxResults(inboxPath, entries, results)
}

func resolveInboxConfig(cfg *Config) inboxConfig {
	ic := inboxConfig{
		concurrency:   cfg.Wiki.Concurrency,
		perURLTimeout: time.Duration(cfg.Wiki.PerURLTimeout) * time.Second,
		maxRetries:    uint(cfg.Wiki.MaxRetries),
	}
	if ic.concurrency <= 0 {
		ic.concurrency = 5
	}
	if ic.perURLTimeout <= 0 {
		ic.perURLTimeout = 3 * time.Minute
	}
	if ic.maxRetries <= 0 {
		ic.maxRetries = 3
	}

	slog.Info("Inbox processing config",
		"concurrency", ic.concurrency,
		"perURLTimeout", ic.perURLTimeout,
		"maxRetries", ic.maxRetries,
	)

	return ic
}

func runInboxEntries(
	cfg *Config, wikiRoot string,
	classifier *wiki.Classifier, fetcher *wiki.Fetcher,
	entries []wiki.InboxEntry, ic inboxConfig,
) []*urlResult {
	results := make([]*urlResult, len(entries))
	var mu sync.Mutex

	g, ctx := errgroup.WithContext(context.Background())
	g.SetLimit(ic.concurrency)

	for i, entry := range entries {
		g.Go(func() error {
			result := processEntryWithRetry(ctx, cfg, wikiRoot, classifier, fetcher, entry, ic)

			mu.Lock()
			results[i] = result
			mu.Unlock()

			return nil // don't crash the whole group for one URL
		})
	}

	_ = g.Wait()

	return results
}

func processEntryWithRetry(
	ctx context.Context, cfg *Config, wikiRoot string,
	classifier *wiki.Classifier, fetcher *wiki.Fetcher,
	entry wiki.InboxEntry, ic inboxConfig,
) *urlResult {
	urlCtx, cancel := context.WithTimeout(ctx, ic.perURLTimeout)
	defer cancel()

	err := retry.Do(
		func() error {
			_, retryErr := processSingleURL(urlCtx, cfg, wikiRoot, classifier, fetcher, &entry)

			return retryErr
		},
		retry.Context(urlCtx),
		retry.Attempts(ic.maxRetries),
		retry.Delay(5*time.Second),
		retry.DelayType(retry.BackOffDelay),
		retry.LastErrorOnly(true),
		retry.RetryIf(func(err error) bool {
			return !errors.Is(err, errNotRetriable)
		}),
		retry.OnRetry(func(n uint, retryErr error) {
			slog.Warn("Retrying processing URL",
				"url", entry.URL, "attempt", n+1, "error", retryErr)
		}),
	)

	if err != nil && !errors.Is(err, errNotRetriable) {
		// Fetch/resolve failure after retry exhaustion — write to failed file
		// and flush from inbox so it doesn't get retried forever.
		writeRetryFailure(wikiRoot, entry, err)
		slog.Error("Failed to process URL after retries",
			"url", entry.URL, "error", err)
	}

	return &urlResult{
		lineIndex: entry.LineIndex,
		url:       entry.URL,
		success:   true, // removed from inbox (either processed or written to failed)
	}
}

func writeRetryFailure(wikiRoot string, entry wiki.InboxEntry, err error) {
	failureType := classifyFailureType(err)
	failItem := &wiki.ClassifyItem{
		URL:   entry.URL,
		Title: entry.URL,
	}
	opts := &wiki.WriteOptions{WikiRoot: wikiRoot}
	if _, wErr := wiki.WriteFailureEntry(failItem, failureType, err.Error(), opts); wErr != nil {
		slog.Error("Failed to write failure entry",
			"url", entry.URL, "error", wErr)
	}
}

func flushInboxResults(inboxPath string, entries []wiki.InboxEntry, results []*urlResult) error {
	processed := make(map[int]bool)
	var succeeded, failed int
	for _, r := range results {
		if r != nil {
			if r.success {
				processed[r.lineIndex] = true
				succeeded++
			} else {
				failed++
				slog.Warn("URL not processed",
					"url", r.url, "error", r.err)
			}
		}
	}

	slog.Info("Inbox processing complete",
		"total", len(entries), "succeeded", succeeded, "failed", failed)

	if len(processed) > 0 {
		if err := wiki.FlushInbox(inboxPath, processed); err != nil {
			return fmt.Errorf("flush inbox: %w", err)
		}
		slog.Info("Flushed processed lines from inbox", "count", len(processed))
	}

	return nil
}

// processSingleURL fetches, classifies, and writes a single inbox entry.
// Returns the classified item on success, nil on handled failure (classified to
// group-failed), or an error if retriable (fetch/resolve failure).
func processSingleURL(
	ctx context.Context, cfg *Config, wikiRoot string,
	classifier *wiki.Classifier, fetcher *wiki.Fetcher, entry *wiki.InboxEntry,
) (*wiki.ClassifyItem, error) {
	slog.Info("Processing inbox URL", "url", entry.URL)

	contentType := wiki.DetectContentType(entry.URL)
	result := fetcher.FetchContent(ctx, entry.URL, contentType)

	if result.Error != "" {
		return nil, fmt.Errorf("fetch content: %s", result.Error)
	}

	title := result.Title
	if title == "" {
		title = entry.URL
	}

	classResult := classifier.ClassifyURL(ctx, entry.URL, title, result.Body)
	if classResult == nil {
		return nil, errors.New("classification failed (returned nil)")
	}

	item := &wiki.ClassifyItem{
		URL:         entry.URL,
		Title:       title,
		ContentType: classResult.ContentType,
		TopicPath:   classResult.TopicPath,
		Type:        classResult.WikiType,
		Summary:     classResult.Summary,
	}

	opts := &wiki.WriteOptions{
		WikiRoot: wikiRoot,
	}

	// Topic path is the sole routing criterion.
	// If AI couldn't find a matching topic, write to group-failed and mark as
	// processed (no retry — the content won't change).
	if classResult.TopicPath == "none" || classResult.TopicPath == "inbox" {
		extraInfo := "AI could not classify the content into any topic.\nSummary: " + classResult.Summary
		if _, err := wiki.WriteFailureEntry(item, wiki.FailureClassify, extraInfo, opts); err != nil {
			return nil, fmt.Errorf("write classify failure: %w", err)
		}
		slog.Info("URL written to group-failed (unclassifiable)", "url", entry.URL)

		return nil, errNotRetriable
	}

	// Valid topic path — write summary
	if _, err := wiki.WriteSummary(item, opts); err != nil {
		return nil, fmt.Errorf("write summary: %w", err)
	}

	slog.Info("Successfully processed URL",
		"url", entry.URL, "topic", classResult.TopicPath, "type", classResult.WikiType)

	return item, nil
}

// classifyFailureType determines whether a fetch error is a "resolve" failure
// (HTTP-level, e.g. 403 anti-bot that even opencli couldn't bypass) or a
// "fetch" failure (network-level, e.g. DNS/timeout).
func classifyFailureType(err error) string {
	if strings.Contains(err.Error(), "resolve:") {
		return "resolve"
	}

	return "fetch"
}
