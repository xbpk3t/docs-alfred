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
	"github.com/creasty/defaults"
	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/xbpk3t/docs-alfred/pkg/ai"
	"github.com/xbpk3t/docs-alfred/service/wiki"
)

// errNotRetriable is returned when a URL was processed but did not produce a
// valid classification (e.g. AI couldn't assign a topic). The entry is flushed
// from the inbox so it is not retried.
var errNotRetriable = errors.New("not retriable")

// wikiConfig holds wiki CLI configuration.
type wikiConfig struct {
	Ai   wikiAiConfig   `yaml:"ai"`
	Wiki wikiWikiConfig `yaml:"wiki"`
}

// wikiWikiConfig wiki-specific config.
type wikiWikiConfig struct {
	WikiRoot      string `default:"wiki"                         validate:"required"     yaml:"wikiRoot"`
	GhTopicsURL   string `default:"https://docs.lucc.dev/gh.yml" validate:"required,url" yaml:"ghTopicsURL"`
	Concurrency   int    `default:"5"                            validate:"gte=1"        yaml:"concurrency"`
	PerURLTimeout int    `default:"180"                          validate:"gte=1"        yaml:"perURLTimeout"` // seconds
	MaxRetries    int    `default:"3"                            validate:"gte=0"        yaml:"maxRetries"`
}

// wikiAiConfig AI model configuration.
type wikiAiConfig struct {
	Model   string `default:"deepseek-v4-flash"       validate:"required"     yaml:"model"`
	BaseURL string `default:"https://api.lucc.dev/v1" validate:"required,url" yaml:"baseUrl"`
}

func defaultWikiConfig() wikiConfig {
	var cfg wikiConfig
	defaults.MustSet(&cfg)

	return cfg
}

var (
	wikiConfigFile  string
	wikiRootOpt     string
)

func newWikiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wiki [flags] [urls...]",
		Short: "Classify and summarize URLs into wiki knowledge base",
		Long: `Classify and summarize URLs into wiki knowledge base.

Uses AI to classify URLs by content type (video/audio/text), topic path,
and entry type (repo_eval/deep_dive/inbox). Writes structured entries.

Use --inbox to process wiki/inbox.md. Pass URLs as positional args.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadWikiConfig()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			inbox, _ := cmd.Flags().GetBool("inbox")
			if inbox {
				return runWikiInbox(cfg)
			}
			if len(args) == 0 {
				slog.Info("No URLs provided and --inbox not set, doing nothing")

				return nil
			}

			return runWikiURLs(cfg, args)
		},
	}

	cmd.Flags().Bool("inbox", false, "Read URLs from wiki/inbox.md, process, and flush")
	cmd.Flags().StringVarP(&wikiConfigFile, "config", "c", "", "Config file path")
	cmd.Flags().StringVar(&wikiRootOpt, "wiki-root", "", "Wiki root directory (overrides config)")

	return cmd
}

func loadWikiConfig() (*wikiConfig, error) {
	cfg := defaultWikiConfig()

	if wikiConfigFile != "" {
		data, err := os.ReadFile(wikiConfigFile)
		if err != nil {
			return nil, fmt.Errorf("read config: %w", err)
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parse config: %w", err)
		}
	}

	if wikiRootOpt != "" {
		cfg.Wiki.WikiRoot = wikiRootOpt
	}
	if err := validator.New().Struct(&cfg); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return &cfg, nil
}

func newWikiAIConfig(cfg *wikiConfig) *ai.ClientConfig {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("LLM_AxonHub")
	}

	return &ai.ClientConfig{
		APIKey:  apiKey,
		BaseURL: cfg.Ai.BaseURL,
		Model:   cfg.Ai.Model,
	}
}

func resolveWikiRoot(cfg *wikiConfig) string {
	if cfg.Wiki.WikiRoot != "" {
		return cfg.Wiki.WikiRoot
	}

	return "wiki"
}

// --- classify (URLs mode) ---

func runWikiURLs(cfg *wikiConfig, urls []string) error {
	wikiRoot := resolveWikiRoot(cfg)

	if _, err := os.Stat(wikiRoot); os.IsNotExist(err) {
		return fmt.Errorf("wiki root not found: %s", wikiRoot)
	}

	aiCfg := newWikiAIConfig(cfg)
	classifier := wiki.NewClassifier(aiCfg, wikiRoot, cfg.Wiki.GhTopicsURL)
	fetcher := wiki.NewFetcher()
	ctx := context.Background()

	slog.Info("Processing URLs for wiki", "count", len(urls), "wikiRoot", wikiRoot)

	for _, urlStr := range urls {
		processClassifyURL(ctx, cfg, wikiRoot, classifier, fetcher, urlStr)
	}

	return nil
}

// processClassifyURL processes a single URL for classification (not inbox mode).
func processClassifyURL(
	ctx context.Context, cfg *wikiConfig, wikiRoot string,
	classifier *wiki.Classifier, fetcher *wiki.Fetcher, urlStr string,
) {
	slog.Info("Processing URL", "url", urlStr)

	contentType := wiki.DetectContentType(urlStr)
	result := fetcher.FetchContent(ctx, urlStr, contentType)

	if result.Error != "" {
		slog.Error("Failed to fetch URL", "url", urlStr, "error", result.Error)

		return
	}

	title := result.Title
	if title == "" {
		title = urlStr
	}

	classResult := classifier.ClassifyURL(ctx, urlStr, title, result.Body)
	if classResult == nil {
		slog.Warn("Classification unavailable", "url", urlStr)

		return
	}

	item := &wiki.ClassifyItem{
		URL:         urlStr,
		Title:       title,
		ContentType: classResult.ContentType,
		TopicPath:   classResult.TopicPath,
		Type:        classResult.WikiType,
		Summary:     classResult.Summary,
	}

	opts := &wiki.WriteOptions{
		WikiRoot: wikiRoot,
	}

	if classResult.TopicPath == "none" || classResult.TopicPath == "inbox" {
		extraInfo := "AI could not classify the content into any topic.\nSummary: " + classResult.Summary
		if _, err := wiki.WriteFailureEntry(item, wiki.FailureClassify, extraInfo, opts); err != nil {
			slog.Error("Failed to write failure entry", "url", urlStr, "error", err)

			return
		}
		slog.Info("URL written to group-failed (unclassifiable)", "url", urlStr)

		return
	}

	if _, err := wiki.WriteSummary(item, opts); err != nil {
		slog.Error("Failed to write summary", "url", urlStr, "error", err)
	}
}

// --- inbox mode ---

type wikiURLResult struct {
	err       error
	url       string
	lineIndex int
	success   bool
}

type wikiInboxConfig struct {
	concurrency   int
	perURLTimeout time.Duration
	maxRetries    uint
}

func runWikiInbox(cfg *wikiConfig) error {
	wikiRoot := resolveWikiRoot(cfg)

	if _, err := os.Stat(wikiRoot); os.IsNotExist(err) {
		return fmt.Errorf("wiki root not found: %s", wikiRoot)
	}

	inboxPath := filepath.Join(wikiRoot, "inbox.md")
	if _, err := os.Stat(inboxPath); os.IsNotExist(err) {
		return fmt.Errorf("inbox file not found: %s", inboxPath)
	}

	aiCfg := newWikiAIConfig(cfg)
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

	ic := resolveWikiInboxConfig(cfg)
	results := runWikiInboxEntries(cfg, wikiRoot, classifier, fetcher, entries, ic)

	return flushWikiInboxResults(inboxPath, entries, results)
}

func resolveWikiInboxConfig(cfg *wikiConfig) wikiInboxConfig {
	ic := wikiInboxConfig{
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

func runWikiInboxEntries(
	cfg *wikiConfig, wikiRoot string,
	classifier *wiki.Classifier, fetcher *wiki.Fetcher,
	entries []wiki.InboxEntry, ic wikiInboxConfig,
) []*wikiURLResult {
	results := make([]*wikiURLResult, len(entries))
	var mu sync.Mutex

	g, ctx := errgroup.WithContext(context.Background())
	g.SetLimit(ic.concurrency)

	for i, entry := range entries {
		g.Go(func() error {
			result := processWikiEntryWithRetry(ctx, cfg, wikiRoot, classifier, fetcher, entry, ic)

			mu.Lock()
			results[i] = result
			mu.Unlock()

			return nil // don't crash the whole group for one URL
		})
	}

	_ = g.Wait()

	return results
}

func processWikiEntryWithRetry(
	ctx context.Context, cfg *wikiConfig, wikiRoot string,
	classifier *wiki.Classifier, fetcher *wiki.Fetcher,
	entry wiki.InboxEntry, ic wikiInboxConfig,
) *wikiURLResult {
	urlCtx, cancel := context.WithTimeout(ctx, ic.perURLTimeout)
	defer cancel()

	err := retry.Do(
		func() error {
			_, retryErr := processWikiSingleURL(urlCtx, cfg, wikiRoot, classifier, fetcher, &entry)

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
		writeWikiRetryFailure(wikiRoot, entry, err)
		slog.Error("Failed to process URL after retries",
			"url", entry.URL, "error", err)
	}

	return &wikiURLResult{
		lineIndex: entry.LineIndex,
		url:       entry.URL,
		success:   true, // removed from inbox (either processed or written to failed)
	}
}

func writeWikiRetryFailure(wikiRoot string, entry wiki.InboxEntry, err error) {
	failureType := classifyWikiFailureType(err)
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

func flushWikiInboxResults(inboxPath string, entries []wiki.InboxEntry, results []*wikiURLResult) error {
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

// processWikiSingleURL fetches, classifies, and writes a single inbox entry.
// Returns the classified item on success, nil on handled failure (classified to
// group-failed), or an error if retriable (fetch/resolve failure).
func processWikiSingleURL(
	ctx context.Context, cfg *wikiConfig, wikiRoot string,
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

// classifyWikiFailureType determines whether a fetch error is a "resolve" failure
// (HTTP-level, e.g. 403 anti-bot that even opencli couldn't bypass) or a
// "fetch" failure (network-level, e.g. DNS/timeout).
func classifyWikiFailureType(err error) string {
	if strings.Contains(err.Error(), "resolve:") {
		return "resolve"
	}

	return "fetch"
}
