package wiki

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	wikisvc "github.com/xbpk3t/docs-alfred/service/wiki"
)

func TestLoadConfigPreservesDefaultsWithPartialFile(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "wiki.yml")
	require.NoError(t, os.WriteFile(configPath, []byte("wiki:\n  concurrency: 2\n"), 0o600))

	cfg, err := LoadConfig(configPath, "")
	require.NoError(t, err)
	require.Equal(t, 2, cfg.Wiki.Concurrency)
	require.Equal(t, "wiki", cfg.Wiki.WikiRoot)
	require.Equal(t, "https://docs.lucc.dev/gh.yml", cfg.Wiki.GhTopicsURL)
	require.Equal(t, "deepseek-v4-flash", cfg.AI.Model)
}

func TestLoadConfigAppliesWikiRootOverride(t *testing.T) {
	cfg, err := LoadConfig("", "custom-wiki")
	require.NoError(t, err)
	require.Equal(t, "custom-wiki", cfg.Wiki.WikiRoot)
}

func TestRunAddURLsWritesSummary(t *testing.T) {
	deps := newFakeDeps()
	deps.fetcher.results["https://example.com/a"] = &wikisvc.ContentFetchResult{Title: "A", Body: "body"}
	deps.classifier.results["https://example.com/a"] = &wikisvc.ClassifyResult{
		TopicPath:   "topic/path",
		WikiType:    wikisvc.TypeDeepDive,
		ContentType: wikisvc.ContentText,
		Summary:     "summary",
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
	deps.fetcher.results["https://example.com/a"] = &wikisvc.ContentFetchResult{Title: "A", Body: "body"}
	deps.classifier.results["https://example.com/a"] = &wikisvc.ClassifyResult{
		TopicPath:   "none",
		WikiType:    wikisvc.TypeInbox,
		ContentType: wikisvc.ContentText,
		Summary:     "summary",
	}

	result, err := RunAddURLs(context.Background(), AddInput{
		Config: testConfig(t),
		URLs:   []string{"https://example.com/a"},
		deps:   deps.dependencies(),
	})

	require.NoError(t, err)
	require.True(t, result.OK())
	require.Equal(t, StatusFailureWritten, result.URLResults[0].Status)
	require.Equal(t, wikisvc.FailureClassify, result.URLResults[0].FailureType)
	require.Len(t, deps.writer.failures, 1)
}

func TestRunAddURLsWritesFetchFailure(t *testing.T) {
	deps := newFakeDeps()
	deps.fetcher.results["https://example.com/a"] = &wikisvc.ContentFetchResult{Error: "resolve: HTTP 403"}

	result, err := RunAddURLs(context.Background(), AddInput{
		Config: testConfig(t),
		URLs:   []string{"https://example.com/a"},
		deps:   deps.dependencies(),
	})

	require.NoError(t, err)
	require.True(t, result.OK())
	require.Equal(t, StatusFailureWritten, result.URLResults[0].Status)
	require.Equal(t, wikisvc.FailureResolve, result.URLResults[0].FailureType)
	require.Len(t, deps.writer.failures, 1)
}

func TestRunAddURLsWriterFailureIsUnhandled(t *testing.T) {
	deps := newFakeDeps()
	deps.fetcher.results["https://example.com/a"] = &wikisvc.ContentFetchResult{Title: "A", Body: "body"}
	deps.classifier.results["https://example.com/a"] = &wikisvc.ClassifyResult{
		TopicPath:   "topic/path",
		WikiType:    wikisvc.TypeDeepDive,
		ContentType: wikisvc.ContentText,
		Summary:     "summary",
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

func TestRunProcessInboxFlushesHandledLines(t *testing.T) {
	deps := newFakeDeps()
	deps.inbox.entries = []wikisvc.InboxEntry{{URL: "https://example.com/a", LineIndex: 1}}
	deps.fetcher.results["https://example.com/a"] = &wikisvc.ContentFetchResult{Title: "A", Body: "body"}
	deps.classifier.results["https://example.com/a"] = &wikisvc.ClassifyResult{
		TopicPath:   "topic/path",
		WikiType:    wikisvc.TypeDeepDive,
		ContentType: wikisvc.ContentText,
		Summary:     "summary",
	}
	cfg := testConfig(t)
	require.NoError(t, os.WriteFile(filepath.Join(cfg.Wiki.WikiRoot, "inbox.md"), []byte("- https://example.com/a\n"), 0o600))

	result, err := RunProcessInbox(context.Background(), InboxInput{
		Config: cfg,
		deps:   deps.dependencies(),
	})

	require.NoError(t, err)
	require.True(t, result.OK())
	require.Equal(t, 1, result.Flushed)
	require.True(t, deps.inbox.flushed[1])
}

func TestRunProcessInboxDryRunDoesNotWriteOrFlush(t *testing.T) {
	deps := newFakeDeps()
	deps.inbox.entries = []wikisvc.InboxEntry{{URL: "https://example.com/a", LineIndex: 1}}
	deps.fetcher.results["https://example.com/a"] = &wikisvc.ContentFetchResult{Title: "A", Body: "body"}
	deps.classifier.results["https://example.com/a"] = &wikisvc.ClassifyResult{
		TopicPath:   "topic/path",
		WikiType:    wikisvc.TypeDeepDive,
		ContentType: wikisvc.ContentText,
		Summary:     "summary",
	}
	cfg := testConfig(t)
	require.NoError(t, os.WriteFile(filepath.Join(cfg.Wiki.WikiRoot, "inbox.md"), []byte("- https://example.com/a\n"), 0o600))

	result, err := RunProcessInbox(context.Background(), InboxInput{
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

func testConfig(t *testing.T) *Config {
	t.Helper()
	wikiRoot := t.TempDir()

	return &Config{
		AI: AIConfig{Model: "model", BaseURL: "https://example.com/v1"},
		Wiki: WikiConfig{
			WikiRoot:      wikiRoot,
			GhTopicsURL:   "https://example.com/gh.yml",
			Concurrency:   1,
			PerURLTimeout: 1,
			MaxRetries:    1,
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
		fetcher:    &fakeFetcher{results: map[string]*wikisvc.ContentFetchResult{}},
		classifier: &fakeClassifier{results: map[string]*wikisvc.ClassifyResult{}},
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
	results map[string]*wikisvc.ContentFetchResult
}

func (f *fakeFetcher) FetchContent(_ context.Context, urlStr, _ string) *wikisvc.ContentFetchResult {
	if result, ok := f.results[urlStr]; ok {
		return result
	}

	return &wikisvc.ContentFetchResult{Title: urlStr, Body: "body"}
}

type fakeClassifier struct {
	results map[string]*wikisvc.ClassifyResult
}

func (f *fakeClassifier) ClassifyURL(_ context.Context, urlStr, _, _ string) *wikisvc.ClassifyResult {
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
	failureType string
	dryRun      bool
}

func (f *fakeWriter) WriteSummary(item *wikisvc.ClassifyItem, opts *wikisvc.WriteOptions) (string, error) {
	if f.summaryErr != nil {
		return "", f.summaryErr
	}
	f.summaries = append(f.summaries, writeCall{url: item.URL, dryRun: opts.DryRun})

	return filepath.Join(opts.WikiRoot, item.TopicPath, "summary.md"), nil
}

func (f *fakeWriter) WriteFailureEntry(item *wikisvc.ClassifyItem, failureType, _ string, opts *wikisvc.WriteOptions) (string, error) {
	if f.failureErr != nil {
		return "", f.failureErr
	}
	f.failures = append(f.failures, failureCall{url: item.URL, failureType: failureType, dryRun: opts.DryRun})

	return filepath.Join(opts.WikiRoot, "failed", failureType+"-failed.md"), nil
}

type fakeInbox struct {
	parseErr error
	flushErr error
	entries  []wikisvc.InboxEntry
	flushed  map[int]bool
}

func (f *fakeInbox) ParseInbox(string) ([]wikisvc.InboxEntry, error) {
	if f.parseErr != nil {
		return nil, f.parseErr
	}

	return f.entries, nil
}

func (f *fakeInbox) FlushInbox(_ string, processedLineIndices map[int]bool) error {
	if f.flushErr != nil {
		return f.flushErr
	}
	f.flushed = processedLineIndices

	return nil
}
