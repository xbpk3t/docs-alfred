package wikiingest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	wikiaudit "github.com/xbpk3t/docs-alfred/internal/docs/wiki/audit"
	wikiclassify "github.com/xbpk3t/docs-alfred/internal/docs/wiki/classify"
	wikifetch "github.com/xbpk3t/docs-alfred/internal/docs/wiki/fetch"
	wikitypes "github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
	wikiwrite "github.com/xbpk3t/docs-alfred/internal/docs/wiki/write"
	"github.com/xbpk3t/docs-alfred/pkg/ai"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
	"github.com/xbpk3t/docs-alfred/pkg/litter"
)

// CommandRunner abstracts external command execution for testability.
// The dir parameter is the working directory for the command.
type CommandRunner func(ctx context.Context, dir string, name string, args ...string) ([]byte, error)

type dependencies struct {
	fetcher    fetcher
	classifier classifier
	writer     writer
	inbox      inboxStore
	// r2 is the media store for transcript uploads and the rss2nl trns index;
	// nil when no R2 bucket is configured.
	r2 mediaStore
	// validTopicPaths is loaded from ghindex for write-layer validation.
	validTopicPaths map[string]bool
	// history holds previously digested URLs (from digest-success.jsonl).
	// digest skips URLs found here; add skips them too unless --force re-digests.
	history *digestHistory
	// failures counts prior auth_required outcomes so an expired login cannot
	// keep the same inbox line forever. Nil for wiki add.
	failures *failureHistory
}

type fetcher interface {
	FetchContent(ctx context.Context, urlStr, contentType string) *wikitypes.ContentFetchResult
}

type classifier interface {
	// ClassifyURLInTopic classifies one page. A zero pin lets the AI choose the
	// topic; a set pin (child posts of a multi-item page) inherits the
	// container's archive location instead.
	ClassifyURLInTopic(
		ctx context.Context,
		urlStr, title, content string,
		pin wikiclassify.TopicPin,
	) (*wikitypes.ClassifyResult, error)
}

type writer interface {
	WriteSummary(item *wikitypes.ClassifyItem, opts *wikiwrite.WriteOptions) (string, error)
	WriteFailureEntry(
		item *wikitypes.ClassifyItem,
		failureType wikitypes.FailureKind,
		failureReason wikitypes.FailureReason,
		extraInfo string,
		opts *wikiwrite.WriteOptions,
	) (string, error)
	WriteManualReviewEntry(
		item *wikitypes.ClassifyItem,
		opts *wikiwrite.WriteOptions,
	) (string, error)
}

type inboxStore interface {
	ParseInbox(filePath string) ([]wikiwrite.InboxEntry, error)
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
	wikiRoot := ResolveWikiRoot(input.Config)
	if err := requireDir(wikiRoot, "wiki root"); err != nil {
		return nil, err
	}

	deps := resolveDependencies(input.Config, input.deps)
	// Digest artifacts are remote-only; a missing R2 store would make every
	// write fail one URL at a time, so refuse the whole batch up front.
	if deps.r2 == nil {
		return nil, errors.New("wiki add: r2.bucket is required (digest artifacts are remote-only)")
	}
	// wiki add also consults digest history so an already digested URL is skipped
	// rather than written again. --force (input.Force) skips seeding the persisted
	// history — re-digesting already seen URLs — but the per-run claim set below
	// still collapses duplicates within this single call in every case.
	if deps.history == nil {
		deps.history = newDigestHistory(digestStoreOrNil(deps))
	}
	if !input.Force {
		deps.history.loadSuccesses()
	}
	result := &Result{Name: "wiki add", WikiRoot: wikiRoot, DryRun: input.DryRun}

	for _, urlStr := range input.URLs {
		if deps.history.claimOrSeen(urlStr) {
			slog.Info("wiki add: skip already digested or duplicate URL", "url", urlStr)
			result.URLResults = append(result.URLResults, URLResult{URL: urlStr, Status: StatusSkipped, Handled: true})
			continue
		}
		result.URLResults = append(result.URLResults, processAddURL(ctx, deps, urlStr, input.DryRun)...)
	}

	return result, nil
}

// PruneInput selects uncat.md entries to remove by URL.
type PruneInput struct {
	Config *Config
	URLs   []string
	DryRun bool
}

// RunPrune removes entries from the remote uncat.md queue by URL. It is a
// serialized R2 read-merge-write and must not run during a digest.
func RunPrune(ctx context.Context, input PruneInput) (*wikiwrite.PruneResult, error) {
	if input.Config == nil {
		return nil, errors.New("wiki config is required")
	}
	store := newMediaStore(input.Config)
	if store == nil {
		return nil, errors.New("wiki prune requires r2.bucket")
	}

	return wikiwrite.PruneManualReview(ctx, digestStoreAdapter{store: store}, input.URLs, input.DryRun)
}

// RunDigest processes wiki/inbox.md and flushes handled lines.
func RunDigest(ctx context.Context, input DigestInput) (result *Result, retErr error) {
	slog.Info("wiki digest started")
	if input.Config == nil {
		return nil, errors.New("wiki config is required")
	}
	wikiRoot := ResolveWikiRoot(input.Config)
	if err := requireDir(wikiRoot, "wiki root"); err != nil {
		return nil, err
	}

	inboxPath := filepath.Join(wikiRoot, "inbox.md")
	if err := requireFile(inboxPath, "inbox file"); err != nil {
		return nil, err
	}

	deps := resolveDependencies(input.Config, input.deps)
	// Digest artifacts are remote-only: summaries, history dedup, logs and feeds
	// all live in the bucket. There is no local fallback.
	if deps.r2 == nil {
		if input.Config.R2.Bucket == "" {
			return nil, errors.New("wiki digest: r2.bucket is required (digest artifacts are remote-only)")
		}

		return nil, errors.New("wiki digest: r2.bucket is configured but the R2 client is unavailable")
	}
	// Only digest consults history — explicit `wiki add` stays re-digest-capable.
	if deps.history == nil {
		deps.history = loadDigestHistory(digestStoreOrNil(deps))
	}
	entries, err := deps.inbox.ParseInbox(inboxPath)
	if err != nil {
		return nil, fmt.Errorf("parse inbox: %w", err)
	}
	// The auth-retry budget only applies to inbox lines, so an empty inbox
	// skips the failure logs entirely. A nil history is safe: authFailures
	// reports zero.
	if deps.failures == nil && len(entries) > 0 {
		deps.failures = loadFailureHistory(digestStoreOrNil(deps))
	}

	result = &Result{Name: "wiki digest", WikiRoot: wikiRoot, DryRun: input.DryRun}
	started := time.Now()
	defer func() {
		logDigestRun(deps, result, started, retErr)
	}()

	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()
	breaker := newAIFailureBreaker(cancelRun)
	inboxCfg := resolveInboxConfig(input.Config)

	result.URLResults = collectDigestResults(runCtx, deps, entries, inboxCfg, input, breaker)
	if err := flushDigestInbox(deps, inboxPath, result, input.DryRun); err != nil {
		return result, fmt.Errorf("flush inbox: %w", err)
	}
	rebuildDigestFeeds(ctx, deps, input.Config, result, input.DryRun)

	if breaker.Tripped() {
		result.AbortReason = "consecutive_ai_failures"

		return result, errAIBreaker
	}

	return result, nil
}

// collectDigestResults runs the inbox and rss2nl trns-index loops.
func collectDigestResults(
	runCtx context.Context,
	deps *dependencies,
	entries []wikiwrite.InboxEntry,
	inboxCfg inboxConfig,
	input DigestInput,
	breaker *aiFailureBreaker,
) []URLResult {
	var results []URLResult
	if len(entries) == 0 {
		slog.Info("wiki digest: no inbox entries found")
	} else {
		slog.Info("wiki digest: processing entries", "count", len(entries))
		results = runInboxEntries(runCtx, deps, entries, inboxCfg, input.DryRun, breaker)
	}

	// Podcast loop: consume the rss2nl trns index so episodes with uploaded
	// transcripts are digested without manual inbox edits. Skip it once the
	// breaker has stopped the run.
	if input.Config.TrnsIndex.Enabled && !breaker.Tripped() {
		results = append(results, runTrnsIndexEntries(runCtx, deps, inboxCfg, input.DryRun)...)
	}

	return results
}

// flushDigestInbox removes handled URLs from inbox.md. On failure the run's
// abort reason is recorded for the run log.
func flushDigestInbox(deps *dependencies, inboxPath string, result *Result, dryRun bool) error {
	processed := handledURLsByLine(result.URLResults)
	if dryRun {
		result.WouldFlush = len(processed)
		slog.Info("wiki digest dry-run: would flush", "count", len(processed))

		return nil
	}
	if len(processed) == 0 {
		slog.Info("wiki digest: nothing to flush")

		return nil
	}
	if err := deps.inbox.FlushInbox(inboxPath, processed); err != nil {
		result.AbortReason = "flush_failed"

		return err
	}
	result.Flushed = len(processed)
	slog.Info("wiki digest completed", "flushed", len(processed))

	return nil
}

// rebuildDigestFeeds rewrites every topic feed and the OPML index. Best-effort:
// a broken feed rebuild must not fail the run. The parent ctx is used so an
// aborted run still publishes what it wrote.
func rebuildDigestFeeds(ctx context.Context, deps *dependencies, cfg *Config, result *Result, dryRun bool) {
	if dryRun || len(result.URLResults) == 0 {
		return
	}
	if err := BuildFeeds(ctx, deps.r2, cfg.R2.PublicBaseURL); err != nil {
		slog.Warn("wiki digest: feed rebuild failed", "error", err)
	}
	if _, err := BuildOPML(ctx, deps.r2, cfg.R2.PublicBaseURL); err != nil {
		slog.Warn("wiki digest: opml rebuild failed", "error", err)
	}
}

// logDigestRun appends a run-level summary to logs/digest-runs.jsonl. It is
// best-effort: a logging failure must not mask the digest outcome.
func logDigestRun(deps *dependencies, result *Result, started time.Time, runErr error) {
	if result == nil {
		return
	}
	opts := newWriteOpts(deps, result.DryRun)
	if _, err := wikiwrite.LogDigestRun(result.RunLog(started, runErr), opts); err != nil {
		slog.Warn("wiki digest: run log write failed", "error", err)
	}
}

// RunAudit scans wiki markdown files for known extraction and URL pollution issues.
func RunAudit(ctx context.Context, input AuditInput) (*AuditResult, error) {
	if input.Config == nil {
		return nil, errors.New("wiki config is required")
	}
	wikiRoot := ResolveWikiRoot(input.Config)
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
		issues, err = wikiaudit.AuditWikiPaths(wikiRoot, auditPaths)
	} else {
		issues, err = wikiaudit.AuditWiki(wikiRoot)
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
		// Negative LineIndex marks non-inbox work (trns index entries); it is
		// never part of the flush set.
		if result.Handled && result.LineIndex >= 0 {
			processed[result.LineIndex] = append(processed[result.LineIndex], result.URL)
		}
	}

	return processed
}

// ResolveWikiRoot returns the configured wiki root, or the default "wiki".
func ResolveWikiRoot(cfg *Config) string {
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

// newWikiFetcher builds the fetcher with an explicit text driver and media
// driver. Text keeps the static-HTTP fallback; media can switch between
// opencli and yt-dlp without affecting text adapters.
func newWikiFetcher(cfg *Config) fetcher {
	// A browser slot is held per in-flight fetch, so more slots than workers
	// is dead config; clamp it so a mistyped value cannot stampede opencli.
	browserConcurrency := cfg.Wiki.BrowserConcurrency
	if limit := cfg.Wiki.Concurrency; limit > 0 && browserConcurrency > limit {
		slog.Warn("browserConcurrency above wiki concurrency has no effect; clamping",
			"browserConcurrency", browserConcurrency, "wikiConcurrency", limit)
		browserConcurrency = limit
	}
	driverOpts := wikifetch.DriverOptions{
		MaxBodySize:         cfg.Wiki.MaxContentSize,
		MediaEnabled:        cfg.Wiki.Media.Enabled,
		BrowserConcurrency:  browserConcurrency,
		YTDLPCookiesBrowser: cfg.Wiki.YTDLPCookiesBrowser,
		YTDLPSubtitleLangs:  cfg.Wiki.YTDLPSubtitleLangs,
	}
	textDriver := newDriverOrOpenCLI(cfg.Wiki.Text.Driver, driverOpts, "text")
	mediaDriver := newDriverOrOpenCLI(cfg.Wiki.Media.Driver, driverOpts, "media")
	// Each opencli driver owns a browser-concurrency semaphore; share one
	// instance so the budget stays global instead of doubling per role.
	if mediaDriver.Name() == "opencli" && textDriver.Name() == "opencli" {
		mediaDriver = textDriver
	}

	fetcherOpts := []wikifetch.FetcherOption{
		wikifetch.WithDriver(textDriver),
		wikifetch.WithMediaDriver(mediaDriver),
		wikifetch.WithMediaEnabled(cfg.Wiki.Media.Enabled),
	}
	if wikifetch.UsesHTTPFallback(textDriver) || wikifetch.UsesHTTPFallback(mediaDriver) {
		fetcherOpts = append(fetcherOpts, httpFallbackOptions(driverOpts)...)
	}
	// Bilibili's official AI summary is an opencli feature. When yt-dlp owns
	// media, keep an opencli media-only fallback for the no-subtitle case.
	if mediaDriver.Name() == "ytdlp" {
		fetcherOpts = append(fetcherOpts, mediaFallbackOptions(textDriver, driverOpts)...)
	}

	return wikifetch.NewFetcher(fetcherOpts...)
}

func newDriverOrOpenCLI(name string, opts wikifetch.DriverOptions, label string) wikifetch.ContentDriver {
	if name == "" {
		name = "opencli"
	}
	driver, err := wikifetch.NewDriver(name, opts)
	if err != nil {
		slog.Warn("Unknown driver, falling back to opencli", "role", label, "driver", name, "error", err)

		driver, _ = wikifetch.NewDriver("opencli", opts)
	}

	return driver
}

// mediaFallbackOptions returns the opencli media fallback for the Bilibili
// no-subtitle case, reusing the text driver when it is already opencli so the
// browser-concurrency budget stays shared.
func mediaFallbackOptions(textDriver wikifetch.ContentDriver, opts wikifetch.DriverOptions) []wikifetch.FetcherOption {
	fallback := textDriver
	if fallback.Name() != "opencli" {
		var err error
		fallback, err = wikifetch.NewDriver("opencli", opts)
		if err != nil {
			slog.Warn("media fallback driver unavailable", "error", err)

			return nil
		}
	}

	return []wikifetch.FetcherOption{wikifetch.WithMediaFallbackDriver(fallback)}
}

// httpFallbackOptions returns the HTTP fallback fetcher option, or nil when the
// fallback driver cannot be created.
func httpFallbackOptions(opts wikifetch.DriverOptions) []wikifetch.FetcherOption {
	fallback, err := wikifetch.NewDriver("http-readability", opts)
	if err != nil {
		slog.Warn("HTTP fallback driver unavailable", "error", err)

		return nil
	}

	return []wikifetch.FetcherOption{wikifetch.WithFallbackDriver(fallback)}
}

func resolveDependencies(cfg *Config, deps *dependencies) *dependencies {
	if deps == nil {
		deps = &dependencies{}
	}
	if deps.fetcher == nil {
		deps.fetcher = newWikiFetcher(cfg)
	}
	if deps.classifier == nil {
		deps.classifier = wikiclassify.NewClassifier(
			newAIConfig(cfg),
			ResolveWikiRoot(cfg),
			cfg.Wiki.GHTopicsURL,
			wikiclassify.WithMaxContentSize(cfg.Wiki.MaxContentSize),
		)
	}
	if deps.writer == nil {
		deps.writer = serviceWriter{}
	}
	if deps.inbox == nil {
		deps.inbox = serviceInboxStore{}
	}
	if deps.validTopicPaths == nil {
		deps.validTopicPaths = wikiwrite.LoadValidTopicPaths(ResolveWikiRoot(cfg), cfg.Wiki.GHTopicsURL)
	}
	if deps.r2 == nil {
		deps.r2 = newMediaStore(cfg)
	}

	return deps
}

// newMediaStore builds the R2 media store from config, or nil when R2 is not
// configured. An invalid config or a failed client init is logged and degrades
// the R2 features off rather than failing the run.
func newMediaStore(cfg *Config) mediaStore {
	r2cfg, err := cfg.R2.LitterConfig()
	if err != nil {
		slog.Warn("wiki: r2 config invalid", "error", err)

		return nil
	}
	if r2cfg == nil {
		return nil
	}
	r2, err := litter.NewR2(r2cfg)
	if err != nil {
		slog.Warn("wiki: r2 client init failed", "error", err)

		return nil
	}

	return r2
}

func newAIConfig(cfg *Config) *ai.ClientConfig {
	// Client-side rate shaping: keep the process under the gateway's account
	// RPM so 429 storms never start. 0 = unlimited.
	ai.SetGlobalRPM(cfg.AI.MaxRPM)
	// Start from DefaultConfig so Streaming defaults to true. Do not assign
	// Streaming from a zero-value bool — that would re-enable CF 524 for
	// tests/hand-built configs that never went through creasty defaults.
	ac := ai.DefaultConfig()
	ac.APIKey = cfg.AI.APIKey
	ac.BaseURL = cfg.AI.BaseURL
	ac.Model = cfg.AI.Model
	if cfg.AI.Effort != "" {
		ac.Effort = cfg.AI.Effort
	}
	if cfg.AI.Timeout > 0 {
		ac.Timeout = time.Duration(cfg.AI.Timeout) * time.Second
	}
	ac.Temperature = cfg.AI.Temperature
	return ac
}
