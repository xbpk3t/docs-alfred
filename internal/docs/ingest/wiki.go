package wikiingest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	wikisvc "github.com/xbpk3t/docs-alfred/internal/docs/wiki"
	"github.com/xbpk3t/docs-alfred/pkg/ai"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

// CommandRunner abstracts external command execution for testability.
// The dir parameter is the working directory for the command.
type CommandRunner func(ctx context.Context, dir string, name string, args ...string) ([]byte, error)

type dependencies struct {
	fetcher         fetcher
	classifier      classifier
	writer          writer
	inbox           inboxStore
	validTopicPaths map[string]bool // loaded from ghindex for write-layer validation
}

type fetcher interface {
	FetchContent(ctx context.Context, urlStr, contentType string) *wikisvc.ContentFetchResult
}

type classifier interface {
	ClassifyURL(ctx context.Context, urlStr, title, content string) *wikisvc.ClassifyResult
}

type writer interface {
	WriteSummary(item *wikisvc.ClassifyItem, opts *wikisvc.WriteOptions) (string, error)
	WriteFailureEntry(
		item *wikisvc.ClassifyItem,
		failureType wikisvc.FailureKind,
		extraInfo string,
		opts *wikisvc.WriteOptions,
	) (string, error)
	WriteManualReviewEntry(
		item *wikisvc.ClassifyItem,
		opts *wikisvc.WriteOptions,
	) (string, error)
}

type inboxStore interface {
	ParseInbox(filePath string) ([]wikisvc.InboxEntry, error)
	FlushInbox(filePath string, handledURLsByLine map[int][]string) error
}

// RunAddURLs classifies and writes explicit URLs.
func RunAddURLs(ctx context.Context, input AddInput) (*Result, error) {
	if input.Config == nil {
		return nil, errors.New("wiki config is required")
	}
	if len(input.URLs) == 0 {
		return nil, errors.New("at least one URL is required")
	}
	wikiRoot := resolveWikiRoot(input.Config)
	if err := requireDir(wikiRoot, "wiki root"); err != nil {
		return nil, err
	}

	deps := resolveDependencies(input.Config, input.deps)
	result := &Result{Name: "wiki add", WikiRoot: wikiRoot, DryRun: input.DryRun}

	for _, urlStr := range input.URLs {
		itemResult := processAddURL(ctx, deps, wikiRoot, urlStr, input.DryRun)
		result.URLResults = append(result.URLResults, itemResult)
	}

	return result, nil
}

// RunDigest processes wiki/inbox.md and flushes handled lines.
func RunDigest(ctx context.Context, input DigestInput) (*Result, error) {
	slog.Info("wiki digest started")
	if input.Config == nil {
		return nil, errors.New("wiki config is required")
	}
	wikiRoot := resolveWikiRoot(input.Config)
	if err := requireDir(wikiRoot, "wiki root"); err != nil {
		return nil, err
	}

	inboxPath := filepath.Join(wikiRoot, "inbox.md")
	if err := requireFile(inboxPath, "inbox file"); err != nil {
		return nil, err
	}

	deps := resolveDependencies(input.Config, input.deps)
	entries, err := deps.inbox.ParseInbox(inboxPath)
	if err != nil {
		return nil, fmt.Errorf("parse inbox: %w", err)
	}

	result := &Result{Name: "wiki digest", WikiRoot: wikiRoot, DryRun: input.DryRun}
	if len(entries) == 0 {
		slog.Info("wiki digest: no entries found")

		return result, nil
	}

	slog.Info("wiki digest: processing entries", "count", len(entries))
	inboxCfg := resolveInboxConfig(input.Config)
	result.URLResults = runInboxEntries(ctx, deps, wikiRoot, entries, inboxCfg, input.DryRun)

	processed := handledURLsByLine(result.URLResults)
	if len(processed) == 0 {
		slog.Info("wiki digest: no entries handled")

		return result, nil
	}
	if input.DryRun {
		result.WouldFlush = len(processed)
		slog.Info("wiki digest dry-run: would flush", "count", len(processed))

		return result, nil
	}

	if err := deps.inbox.FlushInbox(inboxPath, processed); err != nil {
		return result, fmt.Errorf("flush inbox: %w", err)
	}
	result.Flushed = len(processed)
	slog.Info("wiki digest completed", "flushed", len(processed))

	return result, nil
}

// RunAudit scans wiki markdown files for known extraction and URL pollution issues.
func RunAudit(ctx context.Context, input AuditInput) (*AuditResult, error) {
	if input.Config == nil {
		return nil, errors.New("wiki config is required")
	}
	wikiRoot := resolveWikiRoot(input.Config)
	if err := requireDir(wikiRoot, "wiki root"); err != nil {
		return nil, err
	}

	var auditPaths []string
	if len(input.Paths) > 0 {
		auditPaths = input.Paths
	} else if input.ChangedOnly {
		changed, err := changedWikiMarkdownPaths(ctx, wikiRoot, input.RunCmd)
		if err != nil {
			return nil, err
		}
		auditPaths = changed
	}

	var issues []checkutil.Issue
	var err error
	if len(auditPaths) > 0 || input.ChangedOnly {
		issues, err = wikisvc.AuditWikiPaths(wikiRoot, auditPaths)
	} else {
		issues, err = wikisvc.AuditWiki(wikiRoot)
	}
	if err != nil {
		return nil, fmt.Errorf("audit wiki: %w", err)
	}

	return &AuditResult{Name: "wiki audit", WikiRoot: wikiRoot, Issues: issues}, nil
}

func handledURLsByLine(results []URLResult) map[int][]string {
	processed := make(map[int][]string)
	for i := range results {
		result := &results[i]
		if result.Handled {
			processed[result.LineIndex] = append(processed[result.LineIndex], result.URL)
		}
	}

	return processed
}

func resolveWikiRoot(cfg *Config) string {
	if cfg.Wiki.WikiRoot != "" {
		return cfg.Wiki.WikiRoot
	}

	return defaultWikiRoot
}

func requireDir(path, label string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s not found: %s", label, path)
		}

		return fmt.Errorf("stat %s %s: %w", label, path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory: %s", label, path)
	}

	return nil
}

func requireFile(path, label string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s not found: %s", label, path)
		}

		return fmt.Errorf("stat %s %s: %w", label, path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory: %s", label, path)
	}

	return nil
}

func resolveDependencies(cfg *Config, deps *dependencies) *dependencies {
	if deps == nil {
		deps = &dependencies{}
	}
	if deps.fetcher == nil {
		driverName := cfg.Wiki.Driver
		if driverName == "" {
			driverName = "opencli"
		}
		driver, err := wikisvc.NewDriver(driverName, wikisvc.DriverOptions{
			MaxBodySize:  cfg.Wiki.MaxContentSize,
			MediaEnabled: cfg.Wiki.Media.Enabled,
		})
		if err != nil {
			slog.Warn("Unknown driver, falling back to opencli", "driver", driverName, "error", err)
			driver, _ = wikisvc.NewDriver("opencli", wikisvc.DriverOptions{
				MaxBodySize:  cfg.Wiki.MaxContentSize,
				MediaEnabled: cfg.Wiki.Media.Enabled,
			})
		}
		deps.fetcher = wikisvc.NewFetcher(
			wikisvc.WithDriver(driver),
			wikisvc.WithMediaEnabled(cfg.Wiki.Media.Enabled),
		)
	}
	if deps.classifier == nil {
		deps.classifier = wikisvc.NewClassifier(
			newAIConfig(cfg),
			resolveWikiRoot(cfg),
			"",
			wikisvc.WithMaxContentSize(cfg.Wiki.MaxContentSize),
		)
	}
	if deps.writer == nil {
		deps.writer = serviceWriter{}
	}
	if deps.inbox == nil {
		deps.inbox = serviceInboxStore{}
	}
	if deps.validTopicPaths == nil {
		deps.validTopicPaths = wikisvc.LoadValidTopicPaths()
	}

	return deps
}

func newAIConfig(cfg *Config) *ai.ClientConfig {
	// Start from DefaultConfig so Streaming defaults to true. Do not assign
	// Streaming from a zero-value bool — that would re-enable CF 524 for
	// tests/hand-built configs that never went through creasty defaults.
	ac := ai.DefaultConfig()
	ac.APIKey = cfg.AI.APIKey
	ac.BaseURL = cfg.AI.BaseURL
	ac.Model = cfg.AI.Model
	ac.Temperature = cfg.AI.Temperature
	return ac
}
