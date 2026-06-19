package wikiingest

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
	"golang.org/x/sync/errgroup"

	"github.com/xbpk3t/docs-alfred/pkg/ai"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
	"github.com/xbpk3t/docs-alfred/pkg/configutil"
	"github.com/xbpk3t/docs-alfred/service/ghindex"
	wikisvc "github.com/xbpk3t/docs-alfred/service/wiki"
)

const (
	defaultWikiRoot       = "wiki"
	unclassifiedTopicPath = "none"
	inboxTopicPath        = "inbox"

	StatusSummaryWritten = "summary_written"
	StatusFailureWritten = "failure_written"
	StatusUnhandledError = "unhandled_error"
	StatusDryRunSummary  = "dry_run_summary"
	StatusDryRunFailure  = "dry_run_failure"
)

// Config holds wiki workflow configuration.
type Config struct {
	AI   AIConfig   `yaml:"ai"`
	Wiki WikiConfig `yaml:"wiki"`
}

// WikiConfig contains wiki-specific workflow settings.
type WikiConfig struct {
	WikiRoot          string          `default:"wiki"                        validate:"required"     yaml:"wikiRoot"`
	GhTopicsURL       string          `default:"https://cdn.lucc.dev/gh.yml" validate:"required,url" yaml:"ghTopicsURL"`
	GhTopicsCachePath string          `yaml:"ghTopicsCachePath"`
	GhTopicsMaxAge    string          `default:"24h"                         validate:"required"     yaml:"ghTopicsMaxAge"`
	Driver            string          `default:"opencli"                     yaml:"driver"`
	Concurrency       int             `default:"3"                           validate:"gte=1"        yaml:"concurrency"`
	PerURLTimeout     int             `default:"600"                         validate:"gte=1"        yaml:"perURLTimeout"`
	MaxRetries        int             `default:"6"                           validate:"gte=0"        yaml:"maxRetries"`
	MaxContentSize    int             `default:"20000"                       yaml:"maxContentSize"`
	Media             wikiMediaConfig `yaml:"media"`
}

// wikiMediaConfig controls media content extraction.
type wikiMediaConfig struct {
	Enabled bool `default:"true" yaml:"enabled"`
}

// AIConfig contains AI model settings.
type AIConfig struct {
	APIKey      string  `yaml:"apiKey"`
	Model       string  `default:"deepseek-v4-flash"       validate:"required"     yaml:"model"`
	BaseURL     string  `default:"https://api.lucc.dev/v1" validate:"required,url" yaml:"baseUrl"`
	Temperature float64 `default:"0.3"                     yaml:"temperature"`
}

// LoadConfig loads wiki config from disk, preserving defaults for omitted fields.
func LoadConfig(configPath, wikiRootOverride string) (*Config, error) {
	cfg, err := configutil.LoadYAMLConfig(configutil.LoadYAMLConfigOptions[Config]{
		Path:    configPath,
		Initial: defaultConfig(),
		AfterUnmarshal: func(cfg *Config) error {
			if wikiRootOverride != "" {
				cfg.Wiki.WikiRoot = wikiRootOverride
			}

			return nil
		},
		Validate: func(cfg *Config) error {
			if err := validator.New().Struct(cfg); err != nil {
				return err
			}
			if _, err := parseGHTopicsMaxAge(cfg); err != nil {
				return err
			}

			return nil
		},
	})
	if err != nil {
		return nil, formatConfigLoadError(err)
	}

	return &cfg, nil
}

func formatConfigLoadError(err error) error {
	var loadErr *configutil.LoadError
	if !errors.As(err, &loadErr) {
		return err
	}

	switch loadErr.Stage {
	case configutil.StageRead:
		return fmt.Errorf("read config: %w", loadErr.Err)
	case configutil.StageParse:
		return fmt.Errorf("parse config: %w", loadErr.Err)
	case configutil.StageUnmarshal:
		return fmt.Errorf("unmarshal config: %w", loadErr.Err)
	case configutil.StageValidate:
		return fmt.Errorf("validate config: %w", loadErr.Err)
	default:
		return err
	}
}

func defaultConfig() Config {
	var cfg Config
	defaults.MustSet(&cfg)

	return cfg
}

// AddInput contains inputs for wiki URL ingestion.
type AddInput struct {
	Config *Config
	deps   *dependencies
	URLs   []string
	DryRun bool
}

// DigestInput contains inputs for wiki digest processing.
type DigestInput struct {
	Config *Config
	deps   *dependencies
	DryRun bool
}

// AuditInput contains inputs for read-only wiki auditing.
type AuditInput struct {
	Config      *Config
	RunCmd      CommandRunner // optional; required if ChangedOnly is set
	Paths       []string
	ChangedOnly bool
}

// AuditResult is the structured outcome for wiki audit.
type AuditResult struct {
	Name     string            `json:"-"`
	WikiRoot string            `json:"wikiRoot"`
	Issues   []checkutil.Issue `json:"issues"`
}

// Summary returns count-oriented audit details.
func (r *AuditResult) Summary() map[string]any {
	var errorCount, warnings int
	for _, issue := range r.Issues {
		switch issue.Severity {
		case checkutil.SeverityError:
			errorCount++
		case checkutil.SeverityWarn:
			warnings++
		}
	}

	return map[string]any{
		"issues":   len(r.Issues),
		"errors":   errorCount,
		"warnings": warnings,
	}
}

// OK reports whether audit found no error-severity issues.
func (r *AuditResult) OK() bool {
	return !checkutil.HasErrors(r.Issues)
}

// Result is the structured outcome for wiki commands.
type Result struct {
	Name       string      `json:"-"`
	WikiRoot   string      `json:"wikiRoot"`
	URLResults []URLResult `json:"urls"`
	Flushed    int         `json:"flushed"`
	WouldFlush int         `json:"wouldFlush"`
	DryRun     bool        `json:"dryRun"`
}

// URLResult records the outcome for one URL.
type URLResult struct {
	URL         string              `json:"url"`
	Status      string              `json:"status"`
	OutputPath  string              `json:"outputPath,omitempty"`
	TopicPath   string              `json:"topicPath,omitempty"`
	WikiType    string              `json:"wikiType,omitempty"`
	ContentType string              `json:"contentType,omitempty"`
	FailureType wikisvc.FailureKind `json:"failureType,omitempty"`
	Error       string              `json:"error,omitempty"`
	LineIndex   int                 `json:"lineIndex,omitempty"`
	Handled     bool                `json:"handled"`
}

// Summary returns count-oriented command details for structured output.
func (r *Result) Summary() map[string]any {
	var succeeded, handledFailures, unhandledFailures, written int
	for i := range r.URLResults {
		item := &r.URLResults[i]
		switch item.Status {
		case StatusSummaryWritten, StatusDryRunSummary:
			succeeded++
		case StatusFailureWritten, StatusDryRunFailure:
			handledFailures++
		case StatusUnhandledError:
			unhandledFailures++
		}
		if item.OutputPath != "" && (item.Status == StatusSummaryWritten || item.Status == StatusFailureWritten) {
			written++
		}
	}

	return map[string]any{
		"processed":         len(r.URLResults),
		"succeeded":         succeeded,
		"handledFailures":   handledFailures,
		"unhandledFailures": unhandledFailures,
		"written":           written,
		"flushed":           r.Flushed,
		"wouldFlush":        r.WouldFlush,
		"dryRun":            r.DryRun,
	}
}

// OK reports whether the workflow had no unhandled URL-level failures.
func (r *Result) OK() bool {
	for i := range r.URLResults {
		item := &r.URLResults[i]
		if item.Status == StatusUnhandledError {
			return false
		}
	}

	return true
}

// Actions returns command actions for human-readable output.
func (r *Result) Actions() []string {
	var actions []string
	if r.DryRun {
		actions = append(actions, "dry-run: skipped wiki writes")
		if r.WouldFlush > 0 {
			actions = append(actions, fmt.Sprintf("dry-run: skipped inbox flush for %d line(s)", r.WouldFlush))
		}

		return actions
	}
	if r.Flushed > 0 {
		actions = append(actions, fmt.Sprintf("flushed %d inbox line(s)", r.Flushed))
	}

	return actions
}

// CommandRunner abstracts external command execution for testability.
// The dir parameter is the working directory for the command.
type CommandRunner func(ctx context.Context, dir string, name string, args ...string) ([]byte, error)

type dependencies struct {
	fetcher    fetcher
	classifier classifier
	writer     writer
	inbox      inboxStore
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

func changedWikiMarkdownPaths(ctx context.Context, wikiRoot string, runCmd CommandRunner) ([]string, error) {
	roots, err := changedWikiGitRoots(ctx, wikiRoot, runCmd)
	if err != nil {
		return nil, err
	}

	return changedMarkdownPathsFromGit(ctx, roots.repoRoot, roots.relWikiRoot, runCmd)
}

type changedWikiRoots struct {
	repoRoot    string
	relWikiRoot string
}

func changedWikiGitRoots(ctx context.Context, wikiRoot string, runCmd CommandRunner) (changedWikiRoots, error) {
	absWikiRoot, err := filepath.Abs(wikiRoot)
	if err != nil {
		return changedWikiRoots{}, fmt.Errorf("resolve wiki root: %w", err)
	}
	absWikiRoot = evalSymlinksOrOriginal(absWikiRoot)
	repoRoot, err := gitOutput(ctx, absWikiRoot, runCmd, "rev-parse", "--show-toplevel")
	if err != nil {
		return changedWikiRoots{}, fmt.Errorf("find git worktree for changed-only audit: %w", err)
	}
	repoRoot = strings.TrimSpace(repoRoot)
	if repoRoot == "" {
		return changedWikiRoots{}, errors.New("find git worktree for changed-only audit: empty git root")
	}
	repoRoot = evalSymlinksOrOriginal(repoRoot)
	relWikiRoot, err := filepath.Rel(repoRoot, absWikiRoot)
	if err != nil {
		return changedWikiRoots{}, fmt.Errorf("resolve wiki root relative to git root: %w", err)
	}

	return changedWikiRoots{repoRoot: repoRoot, relWikiRoot: filepath.ToSlash(relWikiRoot)}, nil
}

func evalSymlinksOrOriginal(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}

	return resolved
}

func changedMarkdownPathsFromGit(ctx context.Context, repoRoot, relWikiRoot string, runCmd CommandRunner) ([]string, error) {
	seen := make(map[string]bool)
	var paths []string
	for _, args := range [][]string{
		{"diff", "--name-only", "--cached", "--", relWikiRoot},
		{"diff", "--name-only", "--", relWikiRoot},
		{"ls-files", "--others", "--exclude-standard", "--", relWikiRoot},
	} {
		out, err := gitOutput(ctx, repoRoot, runCmd, args...)
		if err != nil {
			return nil, fmt.Errorf("list changed wiki files: %w", err)
		}
		paths = appendChangedMarkdownPaths(paths, seen, repoRoot, out)
	}

	return paths, nil
}

func appendChangedMarkdownPaths(paths []string, seen map[string]bool, repoRoot, output string) []string {
	for rel := range strings.FieldsSeq(output) {
		if filepath.Ext(rel) != ".md" {
			continue
		}
		path := filepath.Join(repoRoot, filepath.FromSlash(rel))
		if seen[path] || !fileExists(path) {
			continue
		}
		seen[path] = true
		paths = append(paths, path)
	}

	return paths
}

func gitOutput(ctx context.Context, dir string, runCmd CommandRunner, args ...string) (string, error) {
	if runCmd == nil {
		return "", fmt.Errorf("git %s: CommandRunner not provided", strings.Join(args, " "))
	}
	out, err := runCmd(ctx, dir, "git", args...)
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}

	return string(out), nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)

	return err == nil && !info.IsDir()
}

func processAddURL(ctx context.Context, deps *dependencies, wikiRoot, urlStr string, dryRun bool) URLResult {
	pending, err := prepareURLAttempt(ctx, deps, urlStr)
	if err == nil {
		return writePendingURL(deps, wikiRoot, &pending, dryRun)
	}

	var fetchErr *fetchFailureError
	if errors.As(err, &fetchErr) {
		pending := newPendingFetchFailure(urlStr, fetchErr.failureType, fetchErr.Error())

		return writePendingURL(deps, wikiRoot, &pending, dryRun)
	}

	return URLResult{URL: urlStr, Status: StatusUnhandledError, Error: err.Error()}
}

func runInboxEntries(
	ctx context.Context,
	deps *dependencies,
	wikiRoot string,
	entries []wikisvc.InboxEntry,
	inboxCfg inboxConfig,
	dryRun bool,
) []URLResult {
	pending := make([]pendingURLWrite, len(entries))
	var mu sync.Mutex

	g, groupCtx := errgroup.WithContext(ctx)
	g.SetLimit(inboxCfg.concurrency)

	for i, entry := range entries {
		g.Go(func() error {
			prepared := prepareInboxEntry(groupCtx, deps, entry, inboxCfg)

			mu.Lock()
			pending[i] = prepared
			mu.Unlock()

			return nil
		})
	}
	_ = g.Wait()

	results := make([]URLResult, len(entries))
	for i, prepared := range pending {
		result := writePendingURL(deps, wikiRoot, &prepared, dryRun)
		result.LineIndex = entries[i].LineIndex
		results[i] = result
	}

	return results
}

func prepareInboxEntry(
	ctx context.Context,
	deps *dependencies,
	entry wikisvc.InboxEntry,
	inboxCfg inboxConfig,
) pendingURLWrite {
	urlCtx, cancel := context.WithTimeout(ctx, inboxCfg.perURLTimeout)
	defer cancel()

	var result pendingURLWrite
	err := retry.Do(
		func() error {
			attemptResult, attemptErr := prepareURLAttempt(urlCtx, deps, entry.URL)
			if attemptErr == nil {
				result = attemptResult
			}

			return attemptErr
		},
		retry.Context(urlCtx),
		retry.Attempts(inboxCfg.maxRetries),
		retry.Delay(5*time.Second),
		retry.DelayType(retry.BackOffDelay),
		retry.LastErrorOnly(true),
		retry.RetryIf(func(err error) bool {
			var fetchErr *fetchFailureError
			if errors.As(err, &fetchErr) {
				return true
			}
			var classifyErr *classifyRetryError

			return errors.As(err, &classifyErr)
		}),
		retry.OnRetry(func(n uint, retryErr error) {
			slog.Warn("Retrying processing wiki URL", "url", entry.URL, "attempt", n+1, "error", retryErr)
		}),
	)
	if err == nil {
		return result
	}

	var fetchErr *fetchFailureError
	if errors.As(err, &fetchErr) {
		return newPendingFetchFailure(entry.URL, fetchErr.failureType, fetchErr.Error())
	}

	var classifyErr *classifyRetryError
	if errors.As(err, &classifyErr) {
		return newPendingAIError(entry.URL, classifyErr.Error())
	}

	return newPendingUnhandled(entry.URL, err.Error())
}

type pendingWriteKind string

const (
	pendingSummary         pendingWriteKind = "summary"
	pendingClassifyFailure pendingWriteKind = "classify_failure"
	pendingExtractFailure  pendingWriteKind = "extract_failure"
	pendingFetchFailure    pendingWriteKind = "fetch_failure"
	pendingAIError         pendingWriteKind = "ai_error"
	pendingUnhandled       pendingWriteKind = "unhandled"
)

type pendingURLWrite struct {
	URL         string
	Kind        pendingWriteKind
	Item        *wikisvc.ClassifyItem
	FailureType wikisvc.FailureKind
	ExtraInfo   string
	Error       string
}

func prepareURLAttempt(ctx context.Context, deps *dependencies, urlStr string) (pendingURLWrite, error) {
	slog.Info("Processing wiki URL", "url", urlStr)

	contentType := wikisvc.DetectContentType(urlStr)
	fetchResult := deps.fetcher.FetchContent(ctx, urlStr, contentType)
	if fetchResult == nil {
		return pendingURLWrite{}, &fetchFailureError{failureType: wikisvc.FailureFetch, message: "fetch content: empty result"}
	}
	if fetchResult.Error != "" {
		return pendingURLWrite{}, &fetchFailureError{
			failureType: failureKindForFetchResult(fetchResult),
			message:     "fetch content: " + fetchResult.Error,
		}
	}

	title := fetchResult.Title
	if title == "" {
		title = urlStr
	}

	content := fetchResult.Body

	// Pre-classification content quality: for video content, require enough
	// content for meaningful classification (i.e., actually got a transcript).
	if contentType == wikisvc.ContentVideo && len([]rune(content)) < 600 {
		item := &wikisvc.ClassifyItem{URL: urlStr, Title: title, ContentType: contentType, Summary: &wikisvc.StructuredSummary{Overview: content}}

		return pendingExtractFailureWrite(item, "video content too short (likely no transcript)"), nil
	}

	classResult := deps.classifier.ClassifyURL(ctx, urlStr, title, content)
	if classResult == nil {
		// Distinguish: empty content is a permanent classify failure (content-side issue);
		// non-empty content with nil classifier means AI call failed (transient).
		item := &wikisvc.ClassifyItem{URL: urlStr, Title: title, ContentType: contentType, Summary: &wikisvc.StructuredSummary{Overview: content}}
		if strings.TrimSpace(content) == "" {
			return pendingExtractFailureWrite(item, "extraction failed: empty content"), nil
		}

		return pendingURLWrite{}, &classifyRetryError{message: "classification failed: AI error"}
	}

	item := &wikisvc.ClassifyItem{
		URL:               urlStr,
		Title:             title,
		ContentType:       classResult.ContentType,
		TopicPath:         classResult.TopicPath,
		Type:              classResult.WikiType,
		Summary:           classResult.Summary,
		MetadataBlock:     classResult.MetadataBlock,
		NeedsManualReview: classResult.NeedsManualReview,
	}

	if shouldWriteClassifyFailure(classResult) {
		extraInfo := classifyFailureInfo(classResult)

		return pendingClassifyFailureWrite(item, extraInfo), nil
	}

	return pendingURLWrite{URL: urlStr, Kind: pendingSummary, Item: item}, nil
}

func pendingClassifyFailureWrite(item *wikisvc.ClassifyItem, extraInfo string) pendingURLWrite {
	return pendingURLWrite{
		URL:         item.URL,
		Kind:        pendingClassifyFailure,
		Item:        item,
		FailureType: wikisvc.FailureClassify,
		ExtraInfo:   extraInfo,
	}
}

func pendingExtractFailureWrite(item *wikisvc.ClassifyItem, extraInfo string) pendingURLWrite {
	return pendingURLWrite{
		URL:         item.URL,
		Kind:        pendingExtractFailure,
		Item:        item,
		FailureType: wikisvc.FailureExtract,
		ExtraInfo:   extraInfo,
	}
}

func newPendingAIError(urlStr, message string) pendingURLWrite {
	return pendingURLWrite{URL: urlStr, Kind: pendingAIError, Error: message}
}

func newPendingFetchFailure(urlStr string, failureType wikisvc.FailureKind, extraInfo string) pendingURLWrite {
	return pendingURLWrite{URL: urlStr, Kind: pendingFetchFailure, FailureType: failureType, ExtraInfo: extraInfo}
}

func newPendingUnhandled(urlStr, message string) pendingURLWrite {
	return pendingURLWrite{URL: urlStr, Kind: pendingUnhandled, Error: message}
}

func writePendingURL(deps *dependencies, wikiRoot string, pending *pendingURLWrite, dryRun bool) URLResult {
	if pending == nil {
		return URLResult{Status: StatusUnhandledError, Error: "missing pending wiki write"}
	}
	switch pending.Kind {
	case pendingSummary:
		return writeSummary(deps, wikiRoot, pending.Item, dryRun)
	case pendingClassifyFailure:
		return writeClassifyFailure(deps, wikiRoot, pending.Item, pending.ExtraInfo, dryRun)
	case pendingExtractFailure:
		return writeExtractFailure(deps, wikiRoot, pending.Item, pending.ExtraInfo, dryRun)
	case pendingFetchFailure:
		return writeFetchFailure(deps, wikiRoot, pending.URL, pending.FailureType, pending.ExtraInfo, dryRun)
	case pendingAIError:
		return writeAIError(deps, wikiRoot, pending.URL, pending.Error, dryRun)
	case pendingUnhandled:
		return URLResult{URL: pending.URL, Status: StatusUnhandledError, Error: pending.Error}
	default:
		return URLResult{URL: pending.URL, Status: StatusUnhandledError, Error: "missing pending wiki write"}
	}
}

func writeSummary(deps *dependencies, wikiRoot string, item *wikisvc.ClassifyItem, dryRun bool) URLResult {
	// Items with NeedsManualReview and good content get written to wiki/uncat.md
	// for manual triage, not under a topic dir.
	if item.NeedsManualReview {
		path, err := deps.writer.WriteManualReviewEntry(
			item,
			&wikisvc.WriteOptions{WikiRoot: wikiRoot, DryRun: dryRun},
		)
		if err != nil {
			return URLResult{URL: item.URL, Status: StatusUnhandledError, Error: fmt.Sprintf("write manual review: %v", err)}
		}

		status := StatusSummaryWritten
		if dryRun {
			status = StatusDryRunSummary
		}

		return URLResult{
			URL:         item.URL,
			Status:      status,
			Handled:     true,
			OutputPath:  path,
			TopicPath:   item.TopicPath,
			WikiType:    string(item.Type),
			ContentType: item.ContentType,
		}
	}

	path, err := deps.writer.WriteSummary(item, &wikisvc.WriteOptions{WikiRoot: wikiRoot, DryRun: dryRun})
	if err != nil {
		return URLResult{URL: item.URL, Status: StatusUnhandledError, Error: fmt.Sprintf("write summary: %v", err)}
	}

	status := StatusSummaryWritten
	if dryRun {
		status = StatusDryRunSummary
	}

	return URLResult{
		URL:         item.URL,
		Status:      status,
		Handled:     true,
		OutputPath:  path,
		TopicPath:   item.TopicPath,
		WikiType:    string(item.Type),
		ContentType: item.ContentType,
	}
}

func shouldWriteClassifyFailure(result *wikisvc.ClassifyResult) bool {
	if result == nil {
		return true
	}
	// If NeedsManualReview but content was good (AI produced a valid summary),
	// treat as success — write layer will route to uncat.md for manual review.
	if result.NeedsManualReview && result.Summary != nil && strings.TrimSpace(result.Summary.Overview) != "" {
		return false
	}

	return result.RejectReason != "" || result.NeedsManualReview || result.WikiType == wikisvc.TypeInbox ||
		result.TopicPath == unclassifiedTopicPath || result.TopicPath == inboxTopicPath
}

func classifyFailureInfo(result *wikisvc.ClassifyResult) string {
	if result == nil {
		return "classification failed (returned nil)"
	}
	var lines []string
	reason := strings.TrimSpace(result.RejectReason)
	if reason == "" {
		reason = "AI marked the item as inbox/manual review"
	}
	lines = append(lines, reason)
	if result.TopicPath != "" {
		lines = append(lines, "Topic: "+result.TopicPath)
	}
	if result.WikiType != "" {
		lines = append(lines, "WikiType: "+string(result.WikiType))
	}
	if result.Confidence > 0 {
		lines = append(lines, fmt.Sprintf("Confidence: %.2f", result.Confidence))
	}
	if result.NeedsManualReview {
		lines = append(lines, "NeedsManualReview: true")
	}
	if result.Summary != nil {
		if s := strings.TrimSpace(wikisvc.RenderStructuredSummary(result.Summary)); s != "" {
			lines = append(lines, "Summary: "+s)
		}
	}

	return strings.Join(lines, "\n")
}

func writeClassifyFailure(
	deps *dependencies,
	wikiRoot string,
	item *wikisvc.ClassifyItem,
	extraInfo string,
	dryRun bool,
) URLResult {
	path, err := deps.writer.WriteFailureEntry(
		item,
		wikisvc.FailureClassify,
		extraInfo,
		&wikisvc.WriteOptions{WikiRoot: wikiRoot, DryRun: dryRun},
	)
	if err != nil {
		return URLResult{
			URL:         item.URL,
			Status:      StatusUnhandledError,
			FailureType: wikisvc.FailureClassify,
			Error:       fmt.Sprintf("write classify failure: %v", err),
		}
	}

	status := StatusFailureWritten
	if dryRun {
		status = StatusDryRunFailure
	}

	return URLResult{
		URL:         item.URL,
		Status:      status,
		Handled:     true,
		OutputPath:  path,
		TopicPath:   item.TopicPath,
		WikiType:    string(item.Type),
		ContentType: item.ContentType,
		FailureType: wikisvc.FailureClassify,
	}
}

func writeExtractFailure(
	deps *dependencies,
	wikiRoot string,
	item *wikisvc.ClassifyItem,
	extraInfo string,
	dryRun bool,
) URLResult {
	path, err := deps.writer.WriteFailureEntry(
		item,
		wikisvc.FailureExtract,
		extraInfo,
		&wikisvc.WriteOptions{WikiRoot: wikiRoot, DryRun: dryRun},
	)
	if err != nil {
		return URLResult{
			URL:         item.URL,
			Status:      StatusUnhandledError,
			FailureType: wikisvc.FailureExtract,
			Error:       fmt.Sprintf("write extract failure: %v", err),
		}
	}

	status := StatusFailureWritten
	if dryRun {
		status = StatusDryRunFailure
	}

	return URLResult{
		URL:         item.URL,
		Status:      status,
		Handled:     true,
		OutputPath:  path,
		FailureType: wikisvc.FailureExtract,
	}
}

func writeFetchFailure(
	deps *dependencies,
	wikiRoot,
	urlStr string,
	failureType wikisvc.FailureKind,
	extraInfo string,
	dryRun bool,
) URLResult {
	item := &wikisvc.ClassifyItem{URL: urlStr, Title: urlStr}
	path, err := deps.writer.WriteFailureEntry(
		item,
		failureType,
		extraInfo,
		&wikisvc.WriteOptions{WikiRoot: wikiRoot, DryRun: dryRun},
	)
	if err != nil {
		return URLResult{
			URL:         urlStr,
			Status:      StatusUnhandledError,
			FailureType: failureType,
			Error:       fmt.Sprintf("write %s failure: %v", failureType, err),
		}
	}

	status := StatusFailureWritten
	if dryRun {
		status = StatusDryRunFailure
	}

	return URLResult{
		URL:         urlStr,
		Status:      status,
		Handled:     true,
		OutputPath:  path,
		FailureType: failureType,
	}
}

func writeAIError(
	deps *dependencies,
	wikiRoot,
	urlStr,
	message string,
	dryRun bool,
) URLResult {
	item := &wikisvc.ClassifyItem{URL: urlStr, Title: urlStr}
	path, err := deps.writer.WriteFailureEntry(
		item,
		wikisvc.FailureAI,
		message,
		&wikisvc.WriteOptions{WikiRoot: wikiRoot, DryRun: dryRun},
	)
	if err != nil {
		return URLResult{
			URL:         urlStr,
			Status:      StatusUnhandledError,
			FailureType: wikisvc.FailureAI,
			Error:       fmt.Sprintf("write AI error: %v", err),
		}
	}

	status := StatusFailureWritten
	if dryRun {
		status = StatusDryRunFailure
	}

	return URLResult{
		URL:         urlStr,
		Status:      status,
		Handled:     true,
		OutputPath:  path,
		FailureType: wikisvc.FailureAI,
	}
}

type fetchFailureError struct {
	failureType wikisvc.FailureKind
	message     string
}

func (e *fetchFailureError) Error() string {
	return e.message
}

// classifyRetryError is returned when a transient classifcation failure occurs
// (AI timeout, rate limit, invalid response) that may succeed on retry.
type classifyRetryError struct {
	message string
}

func (e *classifyRetryError) Error() string {
	return e.message
}

func failureKindForFetchResult(result *wikisvc.ContentFetchResult) wikisvc.FailureKind {
	if result != nil && result.FailureKind != "" {
		return result.FailureKind
	}
	if result == nil {
		return wikisvc.FailureFetch
	}

	return legacyFailureKindFromMessage(result.Error)
}

func legacyFailureKindFromMessage(message string) wikisvc.FailureKind {
	// Compatibility for test doubles or older fetcher implementations that only
	// return the pre-typed error string. Real fetchers should set FailureKind.
	if strings.Contains(message, "extract:") {
		return wikisvc.FailureExtract
	}
	if strings.Contains(message, "resolve:") {
		return wikisvc.FailureResolve
	}

	return wikisvc.FailureFetch
}

func parseGHTopicsMaxAge(cfg *Config) (time.Duration, error) {
	if cfg == nil || strings.TrimSpace(cfg.Wiki.GhTopicsMaxAge) == "" {
		return ghindex.DefaultMaxAge, nil
	}
	duration, err := time.ParseDuration(strings.TrimSpace(cfg.Wiki.GhTopicsMaxAge))
	if err != nil {
		return 0, fmt.Errorf("wiki.ghTopicsMaxAge must be a Go duration: %w", err)
	}
	if duration <= 0 {
		return 0, errors.New("wiki.ghTopicsMaxAge must be positive")
	}

	return duration, nil
}

type inboxConfig struct {
	concurrency   int
	perURLTimeout time.Duration
	maxRetries    uint
}

func resolveInboxConfig(cfg *Config) inboxConfig {
	resolved := inboxConfig{
		concurrency:   cfg.Wiki.Concurrency,
		perURLTimeout: time.Duration(cfg.Wiki.PerURLTimeout) * time.Second,
		maxRetries:    uint(cfg.Wiki.MaxRetries),
	}
	if resolved.concurrency <= 0 {
		resolved.concurrency = 5
	}
	if resolved.perURLTimeout <= 0 {
		resolved.perURLTimeout = 3 * time.Minute
	}
	if resolved.maxRetries <= 0 {
		resolved.maxRetries = 3
	}

	return resolved
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
		maxAge, err := parseGHTopicsMaxAge(cfg)
		if err != nil {
			maxAge = ghindex.DefaultMaxAge
		}
		deps.classifier = wikisvc.NewClassifier(
			newAIConfig(cfg),
			resolveWikiRoot(cfg),
			cfg.Wiki.GhTopicsURL,
			wikisvc.WithGHTopicsCachePath(cfg.Wiki.GhTopicsCachePath),
			wikisvc.WithGHTopicsMaxAge(maxAge),
			wikisvc.WithMaxContentSize(cfg.Wiki.MaxContentSize),
		)
	}
	if deps.writer == nil {
		deps.writer = serviceWriter{}
	}
	if deps.inbox == nil {
		deps.inbox = serviceInboxStore{}
	}

	return deps
}

func newAIConfig(cfg *Config) *ai.ClientConfig {
	return &ai.ClientConfig{
		APIKey:      cfg.AI.APIKey,
		BaseURL:     cfg.AI.BaseURL,
		Model:       cfg.AI.Model,
		Temperature: cfg.AI.Temperature,
	}
}

type serviceWriter struct{}

func (serviceWriter) WriteSummary(item *wikisvc.ClassifyItem, opts *wikisvc.WriteOptions) (string, error) {
	return wikisvc.WriteSummary(item, opts)
}

func (serviceWriter) WriteFailureEntry(
	item *wikisvc.ClassifyItem,
	failureType wikisvc.FailureKind,
	extraInfo string,
	opts *wikisvc.WriteOptions,
) (string, error) {
	return wikisvc.WriteFailureEntry(item, failureType, extraInfo, opts)
}

func (serviceWriter) WriteManualReviewEntry(
	item *wikisvc.ClassifyItem,
	opts *wikisvc.WriteOptions,
) (string, error) {
	return wikisvc.WriteManualReviewEntry(item, opts)
}

type serviceInboxStore struct{}

func (serviceInboxStore) ParseInbox(filePath string) ([]wikisvc.InboxEntry, error) {
	return wikisvc.ParseInbox(filePath)
}

func (serviceInboxStore) FlushInbox(filePath string, handledURLsByLine map[int][]string) error {
	return wikisvc.FlushInbox(filePath, handledURLsByLine)
}
