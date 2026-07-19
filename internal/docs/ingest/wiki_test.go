package wikiingest

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	wikitypes "github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
	wikiwrite "github.com/xbpk3t/docs-alfred/internal/docs/wiki/write"
	"github.com/xbpk3t/docs-alfred/pkg/cmdutil"
)

func TestLoadConfigPreservesDefaultsWithPartialFile(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "wiki.yml")
	require.NoError(t, os.WriteFile(configPath, []byte("wiki:\n  concurrency: 2\n"), 0o600))

	cfg, err := LoadConfig(configPath, "")
	require.NoError(t, err)
	require.Equal(t, 2, cfg.Wiki.Concurrency)
	require.Equal(t, "wiki", cfg.Wiki.WikiRoot)
	require.True(t, cfg.Wiki.Media.Enabled)
	require.Equal(t, "deepseek-v4-flash", cfg.AI.Model)
}

func TestLoadConfigAppliesWikiRootOverride(t *testing.T) {
	cfg, err := LoadConfig("", "custom-wiki")
	require.NoError(t, err)
	require.Equal(t, "custom-wiki", cfg.Wiki.WikiRoot)
}

func TestRunAddURLsWritesSummary(t *testing.T) {
	deps := newFakeDeps()
	deps.fetcher.results["https://example.com/a"] = &wikitypes.ContentFetchResult{Title: "A", Body: "body"}
	deps.classifier.results["https://example.com/a"] = &wikitypes.ClassifyResult{
		TopicPath:   "topic/path",
		WikiType:    wikitypes.TypeDeepDive,
		ContentType: wikitypes.ContentText,
		Summary:     &wikitypes.StructuredSummary{Overview: "summary"},
	}

	result, err := RunAddURLs(context.Background(), AddInput{
		Config: testConfig(t),
		URLs:   []string{"https://example.com/a"},
		deps:   deps.dependencies(),
	})

	require.NoError(t, err)
	require.True(t, result.OK())
	require.Len(t, result.URLResults, 1)
	require.Equal(t, StatusSummaryWritten, result.URLResults[0].Status)
	require.Equal(t, "topic/path", result.URLResults[0].TopicPath)
	require.Len(t, deps.writer.summaries, 1)
}

func TestRunAddURLsWritesClassifyFailure(t *testing.T) {
	deps := newFakeDeps()
	deps.fetcher.results["https://example.com/a"] = &wikitypes.ContentFetchResult{Title: "A", Body: "body"}
	deps.classifier.results["https://example.com/a"] = &wikitypes.ClassifyResult{
		TopicPath:   "none",
		WikiType:    wikitypes.TypeInbox,
		ContentType: wikitypes.ContentText,
		Summary:     &wikitypes.StructuredSummary{Overview: "summary"},
	}

	result, err := RunAddURLs(context.Background(), AddInput{
		Config: testConfig(t),
		URLs:   []string{"https://example.com/a"},
		deps:   deps.dependencies(),
	})

	require.NoError(t, err)
	require.True(t, result.OK())
	// Good summary with none path → uncat success (not classify failure)
	require.Equal(t, StatusSummaryWritten, result.URLResults[0].Status)
	require.Contains(t, result.URLResults[0].OutputPath, "uncat.md")
	require.Empty(t, deps.writer.failures)
}

func TestRunAddURLsTreatsInboxWikiTypeAsClassifyFailure(t *testing.T) {
	deps := newFakeDeps()
	deps.fetcher.results["https://example.com/a"] = &wikitypes.ContentFetchResult{Title: "A", Body: "body"}
	deps.classifier.results["https://example.com/a"] = &wikitypes.ClassifyResult{
		TopicPath:         "topic/path",
		WikiType:          wikitypes.TypeInbox,
		ContentType:       wikitypes.ContentText,
		Summary:           &wikitypes.StructuredSummary{Overview: "needs review summary"},
		Confidence:        0.88,
		NeedsManualReview: true,
		RejectReason:      "AI marked result for manual review",
	}

	result, err := RunAddURLs(context.Background(), AddInput{
		Config: testConfig(t),
		URLs:   []string{"https://example.com/a"},
		deps:   deps.dependencies(),
	})

	require.NoError(t, err)
	require.True(t, result.OK())
	// NeedsManualReview with valid summary → goes to manual review (uncat.md)
	// instead of classify failure.
	require.Equal(t, StatusSummaryWritten, result.URLResults[0].Status)
	require.Contains(t, result.URLResults[0].OutputPath, "uncat.md")
	require.Empty(t, deps.writer.failures)
	require.Empty(t, deps.writer.summaries)
}

func TestRunAddURLsWritesFetchFailure(t *testing.T) {
	deps := newFakeDeps()
	deps.fetcher.results["https://example.com/a"] = &wikitypes.ContentFetchResult{
		Error:       "resolve: HTTP 403",
		FailureKind: wikitypes.FailureResolve,
	}

	result, err := RunAddURLs(context.Background(), AddInput{
		Config: testConfig(t),
		URLs:   []string{"https://example.com/a"},
		deps:   deps.dependencies(),
	})

	require.NoError(t, err)
	require.True(t, result.OK())
	require.Equal(t, StatusFailureWritten, result.URLResults[0].Status)
	require.Equal(t, wikitypes.FailureResolve, result.URLResults[0].FailureType)
	require.Len(t, deps.writer.failures, 1)
}

func TestRunAddURLsWritesExtractFailure(t *testing.T) {
	deps := newFakeDeps()
	deps.fetcher.results["https://example.com/a"] = &wikitypes.ContentFetchResult{
		Error:       "extract: low quality HTTP content",
		FailureKind: wikitypes.FailureExtract,
	}

	result, err := RunAddURLs(context.Background(), AddInput{
		Config: testConfig(t),
		URLs:   []string{"https://example.com/a"},
		deps:   deps.dependencies(),
	})

	require.NoError(t, err)
	require.True(t, result.OK())
	require.Equal(t, StatusFailureWritten, result.URLResults[0].Status)
	require.Equal(t, wikitypes.FailureExtract, result.URLResults[0].FailureType)
	require.Len(t, deps.writer.failures, 1)
}

func TestRunAddURLsUsesTypedFetchFailureKind(t *testing.T) {
	deps := newFakeDeps()
	deps.fetcher.results["https://example.com/a"] = &wikitypes.ContentFetchResult{
		Error:       "low quality HTTP content",
		FailureKind: wikitypes.FailureExtract,
	}

	result, err := RunAddURLs(context.Background(), AddInput{
		Config: testConfig(t),
		URLs:   []string{"https://example.com/a"},
		deps:   deps.dependencies(),
	})

	require.NoError(t, err)
	require.True(t, result.OK())
	require.Equal(t, StatusFailureWritten, result.URLResults[0].Status)
	require.Equal(t, wikitypes.FailureExtract, result.URLResults[0].FailureType)
	require.Len(t, deps.writer.failures, 1)
	require.Equal(t, wikitypes.FailureExtract, deps.writer.failures[0].failureType)
}

func TestRunAddURLsWriterFailureIsUnhandled(t *testing.T) {
	deps := newFakeDeps()
	deps.fetcher.results["https://example.com/a"] = &wikitypes.ContentFetchResult{Title: "A", Body: "body"}
	deps.classifier.results["https://example.com/a"] = &wikitypes.ClassifyResult{
		TopicPath:   "topic/path",
		WikiType:    wikitypes.TypeDeepDive,
		ContentType: wikitypes.ContentText,
		Summary:     &wikitypes.StructuredSummary{Overview: "summary"},
	}
	deps.writer.summaryErr = errors.New("disk full")

	result, err := RunAddURLs(context.Background(), AddInput{
		Config: testConfig(t),
		URLs:   []string{"https://example.com/a"},
		deps:   deps.dependencies(),
	})

	require.NoError(t, err)
	require.False(t, result.OK())
	require.Equal(t, StatusUnhandledError, result.URLResults[0].Status)
	require.Contains(t, result.URLResults[0].Error, "disk full")
}

func TestRunDigestFlushesHandledLines(t *testing.T) {
	deps := newFakeDeps()
	deps.inbox.entries = []wikiwrite.InboxEntry{{URL: "https://example.com/a", LineIndex: 1}}
	deps.fetcher.results["https://example.com/a"] = &wikitypes.ContentFetchResult{Title: "A", Body: "body"}
	deps.classifier.results["https://example.com/a"] = &wikitypes.ClassifyResult{
		TopicPath:   "topic/path",
		WikiType:    wikitypes.TypeDeepDive,
		ContentType: wikitypes.ContentText,
		Summary:     &wikitypes.StructuredSummary{Overview: "summary"},
	}
	cfg := testConfig(t)
	require.NoError(t, os.WriteFile(filepath.Join(cfg.Wiki.WikiRoot, "inbox.md"), []byte("- https://example.com/a\n"), 0o600))

	result, err := RunDigest(context.Background(), DigestInput{
		Config: cfg,
		deps:   deps.dependencies(),
	})

	require.NoError(t, err)
	require.True(t, result.OK())
	require.Equal(t, 1, result.Flushed)
	require.Equal(t, []string{"https://example.com/a"}, deps.inbox.flushed[1])
}

func TestRunDigestWritesSameTopicInInboxOrderAfterReverseCompletion(t *testing.T) {
	urls := []string{"https://example.com/a", "https://example.com/b", "https://example.com/c"}
	blocks := map[string]chan struct{}{
		urls[0]: make(chan struct{}),
		urls[1]: make(chan struct{}),
	}
	started := map[string]chan struct{}{
		urls[0]: make(chan struct{}),
		urls[1]: make(chan struct{}),
	}
	fetcher := &fakeFetcher{results: map[string]*wikitypes.ContentFetchResult{}, blocks: blocks, started: started}
	classifier := &fakeClassifier{results: map[string]*wikitypes.ClassifyResult{}}
	entries := make([]wikiwrite.InboxEntry, 0, len(urls))
	for i, url := range urls {
		fetcher.results[url] = &wikitypes.ContentFetchResult{Title: "Title " + string(rune('A'+i)), Body: "body"}
		classifier.results[url] = &wikitypes.ClassifyResult{
			TopicPath:   "topic/path",
			WikiType:    wikitypes.TypeDeepDive,
			ContentType: wikitypes.ContentText,
			Summary:     &wikitypes.StructuredSummary{Overview: "summary " + url},
		}
		entries = append(entries, wikiwrite.InboxEntry{URL: url, LineIndex: i})
	}

	cfg := testConfig(t)
	cfg.Wiki.Concurrency = 3
	require.NoError(t, os.WriteFile(filepath.Join(cfg.Wiki.WikiRoot, "inbox.md"), []byte("inbox\n"), 0o600))
	deps := &dependencies{
		fetcher:         fetcher,
		classifier:      classifier,
		writer:          serviceWriter{},
		inbox:           &fakeInbox{entries: entries},
		validTopicPaths: map[string]bool{"topic/path": true},
	}

	type runResult struct {
		result *Result
		err    error
	}
	done := make(chan runResult, 1)
	go func() {
		result, err := RunDigest(context.Background(), DigestInput{Config: cfg, deps: deps})
		done <- runResult{result: result, err: err}
	}()

	<-started[urls[0]]
	<-started[urls[1]]
	close(blocks[urls[1]])
	close(blocks[urls[0]])

	outcome := <-done
	require.NoError(t, outcome.err)
	require.True(t, outcome.result.OK())

	data, err := os.ReadFile(filepath.Join(cfg.Wiki.WikiRoot, "topic", "path", "summary.md"))
	require.NoError(t, err)
	content := string(data)
	require.Contains(t, content, "total_urls: 3")
	require.Contains(t, content, "succeeded: 3")
	require.Less(t, strings.Index(content, urls[0]), strings.Index(content, urls[1]))
	require.Less(t, strings.Index(content, urls[1]), strings.Index(content, urls[2]))
}

func TestRunDigestDryRunDoesNotWriteOrFlush(t *testing.T) {
	deps := newFakeDeps()
	deps.inbox.entries = []wikiwrite.InboxEntry{{URL: "https://example.com/a", LineIndex: 1}}
	deps.fetcher.results["https://example.com/a"] = &wikitypes.ContentFetchResult{Title: "A", Body: "body"}
	deps.classifier.results["https://example.com/a"] = &wikitypes.ClassifyResult{
		TopicPath:   "topic/path",
		WikiType:    wikitypes.TypeDeepDive,
		ContentType: wikitypes.ContentText,
		Summary:     &wikitypes.StructuredSummary{Overview: "summary"},
	}
	cfg := testConfig(t)
	require.NoError(t, os.WriteFile(filepath.Join(cfg.Wiki.WikiRoot, "inbox.md"), []byte("- https://example.com/a\n"), 0o600))

	result, err := RunDigest(context.Background(), DigestInput{
		Config: cfg,
		DryRun: true,
		deps:   deps.dependencies(),
	})

	require.NoError(t, err)
	require.True(t, result.OK())
	require.Equal(t, 0, result.Flushed)
	require.Equal(t, 1, result.WouldFlush)
	require.Empty(t, deps.inbox.flushed)
	require.Equal(t, StatusDryRunSummary, result.URLResults[0].Status)
	require.True(t, deps.writer.summaries[0].dryRun)
}

func TestRunAuditReportsIssues(t *testing.T) {
	cfg := testConfig(t)
	topicDir := filepath.Join(cfg.Wiki.WikiRoot, "topic", "path")
	require.NoError(t, os.MkdirAll(topicDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(topicDir, "summary.md"), []byte(`### Bad

- URL: https://t.co/a](https://x.com/a)
- Type: deep_dive

This page requires JavaScript.
`), 0o600))

	result, err := RunAudit(context.Background(), AuditInput{Config: cfg})

	require.NoError(t, err)
	require.False(t, result.OK())
	require.NotEmpty(t, result.Issues)
	require.Equal(t, "wiki audit", result.Name)
}

func TestRunAuditPathScopeIgnoresUnrelatedPollution(t *testing.T) {
	cfg := testConfig(t)
	pollutedDir := filepath.Join(cfg.Wiki.WikiRoot, "old", "polluted")
	cleanDir := filepath.Join(cfg.Wiki.WikiRoot, "new", "clean")
	require.NoError(t, os.MkdirAll(pollutedDir, 0o700))
	require.NoError(t, os.MkdirAll(cleanDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(pollutedDir, "summary.md"), []byte(`### Bad

- URL: https://t.co/a](https://x.com/a)
- Type: deep_dive

This page requires JavaScript.
`), 0o600))
	cleanPath := filepath.Join(cleanDir, "summary.md")
	require.NoError(t, os.WriteFile(cleanPath, []byte(`### Good

- URL: https://example.com/a
- Type: deep_dive

This scoped audit entry is long enough and clean enough to pass without looking at old files.
`), 0o600))

	result, err := RunAudit(context.Background(), AuditInput{Config: cfg, Paths: []string{cleanPath}})

	require.NoError(t, err)
	require.True(t, result.OK())
	require.Empty(t, result.Issues)
}

func TestRunAuditChangedOnlyIgnoresTrackedHistoricalPollution(t *testing.T) {
	repo := t.TempDir()
	wikiRoot := filepath.Join(repo, "wiki")
	pollutedDir := filepath.Join(wikiRoot, "old", "polluted")
	cleanDir := filepath.Join(wikiRoot, "new", "clean")
	require.NoError(t, os.MkdirAll(pollutedDir, 0o700))
	require.NoError(t, os.MkdirAll(cleanDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(pollutedDir, "summary.md"), []byte(`### Bad

- URL: https://t.co/a](https://x.com/a)
- Type: deep_dive

This page requires JavaScript.
`), 0o600))
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "Test")
	runGit(t, repo, "add", "wiki/old/polluted/summary.md")
	runGit(t, repo, "commit", "-m", "seed")
	cleanPath := filepath.Join(cleanDir, "summary.md")
	require.NoError(t, os.WriteFile(cleanPath, []byte(`### Good

- URL: https://example.com/a
- Type: deep_dive

This changed-only audit entry is clean and long enough to avoid historical pollution blocking the run.
`), 0o600))
	cfg := &Config{
		AI:   AIConfig{Model: "model", BaseURL: "https://example.com/v1"},
		Wiki: WikiConfig{WikiRoot: wikiRoot, Concurrency: 1, PerURLTimeout: 1},
	}

	result, err := RunAudit(context.Background(), AuditInput{
		Config:      cfg,
		RunCmd:      testCommandRunner(),
		ChangedOnly: true,
	})

	require.NoError(t, err)
	require.True(t, result.OK())
	require.Empty(t, result.Issues)
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	out, err := cmdutil.RunWithOutput(context.Background(), dir, "git", args...)
	require.NoError(t, err, string(out))
}

func testCommandRunner() CommandRunner {
	return func(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
		return cmdutil.RunWithOutput(ctx, dir, name, args...)
	}
}

func testConfig(t *testing.T) *Config {
	t.Helper()
	wikiRoot := t.TempDir()

	return &Config{
		AI: AIConfig{Model: "model", BaseURL: "https://example.com/v1"},
		Wiki: WikiConfig{
			WikiRoot:      wikiRoot,
			Concurrency:   1,
			PerURLTimeout: 1,
		},
	}
}

type fakeDeps struct {
	fetcher    *fakeFetcher
	classifier *fakeClassifier
	writer     *fakeWriter
	inbox      *fakeInbox
}

func newFakeDeps() *fakeDeps {
	return &fakeDeps{
		fetcher:    &fakeFetcher{results: map[string]*wikitypes.ContentFetchResult{}},
		classifier: &fakeClassifier{results: map[string]*wikitypes.ClassifyResult{}},
		writer:     &fakeWriter{},
		inbox:      &fakeInbox{},
	}
}

func (f *fakeDeps) dependencies() *dependencies {
	return &dependencies{
		fetcher:    f.fetcher,
		classifier: f.classifier,
		writer:     f.writer,
		inbox:      f.inbox,
	}
}

type fakeFetcher struct {
	results   map[string]*wikitypes.ContentFetchResult
	blocks    map[string]chan struct{}
	started   map[string]chan struct{}
	returnNil bool
}

func (f *fakeFetcher) FetchContent(_ context.Context, urlStr, _ string) *wikitypes.ContentFetchResult {
	if ch, ok := f.started[urlStr]; ok {
		close(ch)
	}
	if ch, ok := f.blocks[urlStr]; ok {
		<-ch
	}
	if f.returnNil {
		return nil
	}
	if result, ok := f.results[urlStr]; ok {
		return result
	}

	return &wikitypes.ContentFetchResult{Title: urlStr, Body: "body"}
}

type fakeClassifier struct {
	results map[string]*wikitypes.ClassifyResult
}

func (f *fakeClassifier) ClassifyURL(_ context.Context, urlStr, _, _ string) *wikitypes.ClassifyResult {
	return f.results[urlStr]
}

type fakeWriter struct {
	summaryErr error
	failureErr error
	summaries  []writeCall
	failures   []failureCall
}

type writeCall struct {
	url    string
	dryRun bool
}

type failureCall struct {
	url         string
	failureType wikitypes.FailureKind
	extraInfo   string
	dryRun      bool
}

func (f *fakeWriter) WriteSummary(item *wikitypes.ClassifyItem, opts *wikiwrite.WriteOptions) (string, error) {
	if f.summaryErr != nil {
		return "", f.summaryErr
	}
	f.summaries = append(f.summaries, writeCall{url: item.URL, dryRun: opts.DryRun})

	return filepath.Join(opts.WikiRoot, item.TopicPath, "summary.md"), nil
}

func (f *fakeWriter) WriteFailureEntry(
	item *wikitypes.ClassifyItem,
	failureType wikitypes.FailureKind,
	extraInfo string,
	opts *wikiwrite.WriteOptions,
) (string, error) {
	if f.failureErr != nil {
		return "", f.failureErr
	}
	f.failures = append(f.failures, failureCall{url: item.URL, failureType: failureType, dryRun: opts.DryRun, extraInfo: extraInfo})

	return filepath.Join(opts.WikiRoot, failureType.String()+"-failed.md"), nil
}

func (f *fakeWriter) WriteManualReviewEntry(item *wikitypes.ClassifyItem, opts *wikiwrite.WriteOptions) (string, error) {
	if f.failureErr != nil {
		return "", f.failureErr
	}

	return filepath.Join(opts.WikiRoot, "uncat.md"), nil
}

type fakeInbox struct {
	parseErr error
	flushErr error
	flushed  map[int][]string
	entries  []wikiwrite.InboxEntry
}

func (f *fakeInbox) ParseInbox(string) ([]wikiwrite.InboxEntry, error) {
	if f.parseErr != nil {
		return nil, f.parseErr
	}

	return f.entries, nil
}

func (f *fakeInbox) FlushInbox(_ string, processedLineIndices map[int][]string) error {
	if f.flushErr != nil {
		return f.flushErr
	}
	f.flushed = processedLineIndices

	return nil
}
