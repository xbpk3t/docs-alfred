package wiki

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
	"github.com/xbpk3t/docs-alfred/pkg/configutil"
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
	WikiRoot      string `default:"wiki"                         validate:"required"     yaml:"wikiRoot"`
	GhTopicsURL   string `default:"https://docs.lucc.dev/gh.yml" validate:"required,url" yaml:"ghTopicsURL"`
	Concurrency   int    `default:"5"                            validate:"gte=1"        yaml:"concurrency"`
	PerURLTimeout int    `default:"180"                          validate:"gte=1"        yaml:"perURLTimeout"`
	MaxRetries    int    `default:"3"                            validate:"gte=0"        yaml:"maxRetries"`
}

// AIConfig contains AI model settings.
type AIConfig struct {
	Model   string `default:"deepseek-v4-flash"       validate:"required"     yaml:"model"`
	BaseURL string `default:"https://api.lucc.dev/v1" validate:"required,url" yaml:"baseUrl"`
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
			return validator.New().Struct(cfg)
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

// InboxInput contains inputs for wiki inbox processing.
type InboxInput struct {
	Config *Config
	deps   *dependencies
	DryRun bool
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
	URL         string `json:"url"`
	Status      string `json:"status"`
	OutputPath  string `json:"outputPath,omitempty"`
	TopicPath   string `json:"topicPath,omitempty"`
	WikiType    string `json:"wikiType,omitempty"`
	ContentType string `json:"contentType,omitempty"`
	FailureType string `json:"failureType,omitempty"`
	Error       string `json:"error,omitempty"`
	LineIndex   int    `json:"lineIndex,omitempty"`
	Handled     bool   `json:"handled"`
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
	WriteFailureEntry(item *wikisvc.ClassifyItem, failureType, extraInfo string, opts *wikisvc.WriteOptions) (string, error)
}

type inboxStore interface {
	ParseInbox(filePath string) ([]wikisvc.InboxEntry, error)
	FlushInbox(filePath string, processedLineIndices map[int]bool) error
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

// RunProcessInbox processes wiki/inbox.md and flushes handled lines.
func RunProcessInbox(ctx context.Context, input InboxInput) (*Result, error) {
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

	result := &Result{Name: "wiki inbox process", WikiRoot: wikiRoot, DryRun: input.DryRun}
	if len(entries) == 0 {
		return result, nil
	}

	inboxCfg := resolveInboxConfig(input.Config)
	result.URLResults = runInboxEntries(ctx, deps, wikiRoot, entries, inboxCfg, input.DryRun)

	processed := handledLineIndices(result.URLResults)
	if len(processed) == 0 {
		return result, nil
	}
	if input.DryRun {
		result.WouldFlush = len(processed)

		return result, nil
	}

	if err := deps.inbox.FlushInbox(inboxPath, processed); err != nil {
		return result, fmt.Errorf("flush inbox: %w", err)
	}
	result.Flushed = len(processed)

	return result, nil
}

func processAddURL(ctx context.Context, deps *dependencies, wikiRoot, urlStr string, dryRun bool) URLResult {
	result, err := processURLAttempt(ctx, deps, wikiRoot, urlStr, dryRun)
	if err == nil {
		return result
	}

	var fetchErr *fetchFailureError
	if errors.As(err, &fetchErr) {
		return writeFetchFailure(deps, wikiRoot, urlStr, fetchErr.failureType, fetchErr.Error(), dryRun)
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
	results := make([]URLResult, len(entries))
	var mu sync.Mutex

	g, groupCtx := errgroup.WithContext(ctx)
	g.SetLimit(inboxCfg.concurrency)

	for i, entry := range entries {
		g.Go(func() error {
			result := processInboxEntry(groupCtx, deps, wikiRoot, entry, inboxCfg, dryRun)
			result.LineIndex = entry.LineIndex

			mu.Lock()
			results[i] = result
			mu.Unlock()

			return nil
		})
	}
	_ = g.Wait()

	return results
}

func processInboxEntry(
	ctx context.Context,
	deps *dependencies,
	wikiRoot string,
	entry wikisvc.InboxEntry,
	inboxCfg inboxConfig,
	dryRun bool,
) URLResult {
	urlCtx, cancel := context.WithTimeout(ctx, inboxCfg.perURLTimeout)
	defer cancel()

	var result URLResult
	err := retry.Do(
		func() error {
			attemptResult, attemptErr := processURLAttempt(urlCtx, deps, wikiRoot, entry.URL, dryRun)
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

			return errors.As(err, &fetchErr)
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
		return writeFetchFailure(deps, wikiRoot, entry.URL, fetchErr.failureType, fetchErr.Error(), dryRun)
	}

	return URLResult{URL: entry.URL, Status: StatusUnhandledError, Error: err.Error()}
}

func processURLAttempt(ctx context.Context, deps *dependencies, wikiRoot, urlStr string, dryRun bool) (URLResult, error) {
	slog.Info("Processing wiki URL", "url", urlStr)

	contentType := wikisvc.DetectContentType(urlStr)
	fetchResult := deps.fetcher.FetchContent(ctx, urlStr, contentType)
	if fetchResult == nil {
		return URLResult{}, &fetchFailureError{failureType: wikisvc.FailureFetch, message: "fetch content: empty result"}
	}
	if fetchResult.Error != "" {
		return URLResult{}, &fetchFailureError{
			failureType: classifyFailureType(fetchResult.Error),
			message:     "fetch content: " + fetchResult.Error,
		}
	}

	title := fetchResult.Title
	if title == "" {
		title = urlStr
	}

	classResult := deps.classifier.ClassifyURL(ctx, urlStr, title, fetchResult.Body)
	if classResult == nil {
		item := &wikisvc.ClassifyItem{URL: urlStr, Title: title, ContentType: contentType}

		return writeClassifyFailure(deps, wikiRoot, item, "classification failed (returned nil)", dryRun), nil
	}

	item := &wikisvc.ClassifyItem{
		URL:         urlStr,
		Title:       title,
		ContentType: classResult.ContentType,
		TopicPath:   classResult.TopicPath,
		Type:        classResult.WikiType,
		Summary:     classResult.Summary,
	}

	if classResult.TopicPath == unclassifiedTopicPath || classResult.TopicPath == inboxTopicPath {
		extraInfo := "AI could not classify the content into any topic.\nSummary: " + classResult.Summary

		return writeClassifyFailure(deps, wikiRoot, item, extraInfo, dryRun), nil
	}

	path, err := deps.writer.WriteSummary(item, &wikisvc.WriteOptions{WikiRoot: wikiRoot, DryRun: dryRun})
	if err != nil {
		return URLResult{URL: urlStr, Status: StatusUnhandledError, Error: fmt.Sprintf("write summary: %v", err)}, nil
	}

	status := StatusSummaryWritten
	if dryRun {
		status = StatusDryRunSummary
	}

	return URLResult{
		URL:         urlStr,
		Status:      status,
		Handled:     true,
		OutputPath:  path,
		TopicPath:   classResult.TopicPath,
		WikiType:    string(classResult.WikiType),
		ContentType: classResult.ContentType,
	}, nil
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

func writeFetchFailure(deps *dependencies, wikiRoot, urlStr, failureType, extraInfo string, dryRun bool) URLResult {
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

type fetchFailureError struct {
	failureType string
	message     string
}

func (e *fetchFailureError) Error() string {
	return e.message
}

func classifyFailureType(message string) string {
	if strings.Contains(message, "resolve:") {
		return wikisvc.FailureResolve
	}

	return wikisvc.FailureFetch
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

func handledLineIndices(results []URLResult) map[int]bool {
	processed := make(map[int]bool)
	for i := range results {
		result := &results[i]
		if result.Handled {
			processed[result.LineIndex] = true
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
		deps.fetcher = wikisvc.NewFetcher()
	}
	if deps.classifier == nil {
		deps.classifier = wikisvc.NewClassifier(newAIConfig(cfg), resolveWikiRoot(cfg), cfg.Wiki.GhTopicsURL)
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
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("LLM_AxonHub")
	}

	return &ai.ClientConfig{
		APIKey:  apiKey,
		BaseURL: cfg.AI.BaseURL,
		Model:   cfg.AI.Model,
	}
}

type serviceWriter struct{}

func (serviceWriter) WriteSummary(item *wikisvc.ClassifyItem, opts *wikisvc.WriteOptions) (string, error) {
	return wikisvc.WriteSummary(item, opts)
}

func (serviceWriter) WriteFailureEntry(
	item *wikisvc.ClassifyItem,
	failureType string,
	extraInfo string,
	opts *wikisvc.WriteOptions,
) (string, error) {
	return wikisvc.WriteFailureEntry(item, failureType, extraInfo, opts)
}

type serviceInboxStore struct{}

func (serviceInboxStore) ParseInbox(filePath string) ([]wikisvc.InboxEntry, error) {
	return wikisvc.ParseInbox(filePath)
}

func (serviceInboxStore) FlushInbox(filePath string, processedLineIndices map[int]bool) error {
	return wikisvc.FlushInbox(filePath, processedLineIndices)
}
