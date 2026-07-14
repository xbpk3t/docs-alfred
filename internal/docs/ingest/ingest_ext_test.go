package wikiingest

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	wikisvc "github.com/xbpk3t/docs-alfred/internal/docs/wiki"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
	"github.com/xbpk3t/docs-alfred/pkg/configutil"
)

// --- shouldWriteClassifyFailure ---

func TestShouldWriteClassifyFailureNil(t *testing.T) {
	assert.True(t, shouldWriteClassifyFailure(nil))
}

func TestShouldWriteClassifyFailureRejectReason(t *testing.T) {
	result := &wikisvc.ClassifyResult{RejectReason: "low confidence"}
	assert.True(t, shouldWriteClassifyFailure(result))
}

func TestShouldWriteClassifyFailureNeedsManualReview(t *testing.T) {
	result := &wikisvc.ClassifyResult{NeedsManualReview: true}
	assert.True(t, shouldWriteClassifyFailure(result))
}

func TestShouldWriteClassifyFailureInboxType(t *testing.T) {
	result := &wikisvc.ClassifyResult{WikiType: wikisvc.TypeInbox}
	assert.True(t, shouldWriteClassifyFailure(result))
}

func TestShouldWriteClassifyFailureNoneTopicPath(t *testing.T) {
	result := &wikisvc.ClassifyResult{TopicPath: unclassifiedTopicPath}
	assert.True(t, shouldWriteClassifyFailure(result))
}

func TestShouldWriteClassifyFailureInboxTopicPath(t *testing.T) {
	result := &wikisvc.ClassifyResult{TopicPath: inboxTopicPath}
	assert.True(t, shouldWriteClassifyFailure(result))
}

func TestShouldWriteClassifyFailureValidResult(t *testing.T) {
	result := &wikisvc.ClassifyResult{
		TopicPath: "topic/path",
		WikiType:  wikisvc.TypeDeepDive,
	}
	assert.False(t, shouldWriteClassifyFailure(result))
}

func TestShouldWriteClassifyFailureManualReviewWithGoodSummary(t *testing.T) {
	result := &wikisvc.ClassifyResult{
		NeedsManualReview: true,
		Summary:           &wikisvc.StructuredSummary{Overview: "good content"},
	}
	assert.False(t, shouldWriteClassifyFailure(result))
}

// --- classifyFailureInfo ---

func TestClassifyFailureInfoNil(t *testing.T) {
	info := classifyFailureInfo(nil)
	assert.Contains(t, info, "classification failed (returned nil)")
}

func TestClassifyFailureInfoWithRejectReason(t *testing.T) {
	result := &wikisvc.ClassifyResult{
		RejectReason: "low confidence",
		TopicPath:    "topic/path",
		WikiType:     wikisvc.TypeDeepDive,
		Confidence:   0.3,
	}
	info := classifyFailureInfo(result)
	assert.Contains(t, info, "low confidence")
	assert.Contains(t, info, "Topic: topic/path")
	assert.Contains(t, info, "WikiType: research")
	assert.Contains(t, info, "Confidence: 0.30")
}

func TestClassifyFailureInfoNoRejectReason(t *testing.T) {
	result := &wikisvc.ClassifyResult{
		NeedsManualReview: true,
		Summary:           &wikisvc.StructuredSummary{Overview: "overview", KeyPoints: []string{"p"}},
	}
	info := classifyFailureInfo(result)
	assert.Contains(t, info, "AI marked the item as inbox/manual review")
	assert.Contains(t, info, "NeedsManualReview: true")
}

// --- failureKindForFetchResult ---

func TestFailureKindForFetchResultNil(t *testing.T) {
	assert.Equal(t, wikisvc.FailureFetch, failureKindForFetchResult(nil))
}

func TestFailureKindForFetchResultWithKind(t *testing.T) {
	result := &wikisvc.ContentFetchResult{FailureKind: wikisvc.FailureExtract}
	assert.Equal(t, wikisvc.FailureExtract, failureKindForFetchResult(result))
}

func TestFailureKindForFetchResultLegacyExtract(t *testing.T) {
	result := &wikisvc.ContentFetchResult{Error: "extract: low quality"}
	assert.Equal(t, wikisvc.FailureExtract, failureKindForFetchResult(result))
}

func TestFailureKindForFetchResultLegacyResolve(t *testing.T) {
	result := &wikisvc.ContentFetchResult{Error: "resolve: HTTP 403"}
	assert.Equal(t, wikisvc.FailureResolve, failureKindForFetchResult(result))
}

func TestFailureKindForFetchResultLegacyFetch(t *testing.T) {
	result := &wikisvc.ContentFetchResult{Error: "connection refused"}
	assert.Equal(t, wikisvc.FailureFetch, failureKindForFetchResult(result))
}

// --- legacyFailureKindFromMessage ---

func TestLegacyFailureKindFromMessage(t *testing.T) {
	assert.Equal(t, wikisvc.FailureExtract, legacyFailureKindFromMessage("extract: something"))
	assert.Equal(t, wikisvc.FailureResolve, legacyFailureKindFromMessage("resolve: HTTP error"))
	assert.Equal(t, wikisvc.FailureFetch, legacyFailureKindFromMessage("connection timeout"))
}

// --- handledURLsByLine ---

func TestHandledURLsByLine(t *testing.T) {
	results := []URLResult{
		{URL: "https://a.com", LineIndex: 0, Handled: true},
		{URL: "https://b.com", LineIndex: 0, Handled: true},
		{URL: "https://c.com", LineIndex: 1, Handled: false},
		{URL: "https://d.com", LineIndex: 2, Handled: true},
	}
	processed := handledURLsByLine(results)
	assert.Len(t, processed[0], 2)
	assert.NotContains(t, processed, 1)
	assert.Len(t, processed[2], 1)
}

func TestHandledURLsByLineEmpty(t *testing.T) {
	processed := handledURLsByLine(nil)
	assert.Empty(t, processed)
}

// --- Result ---

func TestResultOK(t *testing.T) {
	r := &Result{URLResults: []URLResult{{Status: StatusSummaryWritten}}}
	assert.True(t, r.OK())
}

func TestResultNotOK(t *testing.T) {
	r := &Result{URLResults: []URLResult{{Status: StatusUnhandledError}}}
	assert.False(t, r.OK())
}

func TestResultSummary(t *testing.T) {
	r := &Result{
		Flushed: 2,
		URLResults: []URLResult{
			{Status: StatusSummaryWritten, OutputPath: "/path/1"},
			{Status: StatusFailureWritten, OutputPath: "/path/2"},
			{Status: StatusUnhandledError},
		},
	}
	s := r.Summary()
	assert.Equal(t, 3, s["processed"])
	assert.Equal(t, 1, s["succeeded"])
	assert.Equal(t, 1, s["handledFailures"])
	assert.Equal(t, 1, s["unhandledFailures"])
	assert.Equal(t, 2, s["written"])
	assert.Equal(t, 2, s["flushed"])
}

func TestResultSummaryDryRun(t *testing.T) {
	r := &Result{
		DryRun: true,
		URLResults: []URLResult{
			{Status: StatusDryRunSummary},
			{Status: StatusDryRunFailure},
		},
	}
	s := r.Summary()
	assert.Equal(t, 1, s["succeeded"])
	assert.Equal(t, 1, s["handledFailures"])
	assert.True(t, s["dryRun"].(bool))
}

func TestResultActionsDryRun(t *testing.T) {
	r := &Result{DryRun: true, WouldFlush: 3}
	actions := r.Actions()
	assert.Len(t, actions, 2)
	assert.Contains(t, actions[0], "dry-run")
	assert.Contains(t, actions[1], "3 line(s)")
}

func TestResultActionsFlushed(t *testing.T) {
	r := &Result{Flushed: 5}
	actions := r.Actions()
	assert.Len(t, actions, 1)
	assert.Contains(t, actions[0], "5 inbox line(s)")
}

func TestResultActionsEmpty(t *testing.T) {
	r := &Result{}
	assert.Empty(t, r.Actions())
}

// --- AuditResult ---

func TestAuditResultOK(t *testing.T) {
	r := &AuditResult{Issues: nil}
	assert.True(t, r.OK())
}

func TestAuditResultNotOK(t *testing.T) {
	r := &AuditResult{Issues: []checkutil.Issue{{Severity: checkutil.SeverityError}}}
	assert.False(t, r.OK())
}

func TestAuditResultSummary(t *testing.T) {
	r := &AuditResult{Issues: []checkutil.Issue{
		{Severity: checkutil.SeverityError},
		{Severity: checkutil.SeverityWarn},
		{Severity: checkutil.SeverityError},
	}}
	s := r.Summary()
	assert.Equal(t, 3, s["issues"])
	assert.Equal(t, 2, s["errors"])
	assert.Equal(t, 1, s["warnings"])
}

// --- formatConfigLoadError ---

func TestFormatConfigLoadErrorNonLoadError(t *testing.T) {
	err := assert.AnError
	assert.Equal(t, err, formatConfigLoadError(err))
}

// --- resolveInboxConfig ---

func TestResolveInboxConfigDefaults(t *testing.T) {
	cfg := &Config{}
	ic := resolveInboxConfig(cfg)
	assert.Equal(t, 5, ic.concurrency)
	assert.Equal(t, 3*time.Minute, ic.perURLTimeout)
}

func TestResolveInboxConfigExplicit(t *testing.T) {
	cfg := &Config{Wiki: WikiConfig{Concurrency: 10, PerURLTimeout: 120}}
	ic := resolveInboxConfig(cfg)
	assert.Equal(t, 10, ic.concurrency)
	assert.Equal(t, 120*time.Second, ic.perURLTimeout)
}

// --- requireDir / requireFile ---

func TestRequireDirNotFound(t *testing.T) {
	err := requireDir("/tmp/nonexistent-dir-12345", "test dir")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRequireDirNotADir(t *testing.T) {
	f := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(f, []byte(""), 0o600))
	err := requireDir(f, "test dir")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a directory")
}

func TestRequireFileNotFound(t *testing.T) {
	err := requireFile("/tmp/nonexistent-file-12345", "test file")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRequireFileIsDir(t *testing.T) {
	err := requireFile(t.TempDir(), "test file")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is a directory")
}

// --- resolveWikiRoot ---

func TestResolveWikiRootDefault(t *testing.T) {
	assert.Equal(t, defaultWikiRoot, resolveWikiRoot(&Config{}))
}

func TestResolveWikiRootExplicit(t *testing.T) {
	assert.Equal(t, "/custom", resolveWikiRoot(&Config{Wiki: WikiConfig{WikiRoot: "/custom"}}))
}

// --- newAIConfig ---

func TestNewAIConfig(t *testing.T) {
	cfg := &Config{
		AI: AIConfig{
			APIKey:      "key",
			BaseURL:     "https://api.example.com/v1",
			Model:       "model",
			Temperature: 0.5,
		},
	}
	ac := newAIConfig(cfg)
	assert.Equal(t, "key", ac.APIKey)
	assert.Equal(t, "https://api.example.com/v1", ac.BaseURL)
	assert.Equal(t, "model", ac.Model)
	assert.Equal(t, 0.5, ac.Temperature)
	assert.True(t, ac.Streaming, "inherits DefaultConfig streaming even without YAML field")
}

// --- Error types ---

func TestFetchFailureError(t *testing.T) {
	e := &fetchFailureError{failureType: wikisvc.FailureFetch, message: "fetch failed"}
	assert.Equal(t, "fetch failed", e.Error())
}

func TestClassifyRetryError(t *testing.T) {
	e := &classifyRetryError{message: "AI timeout"}
	assert.Equal(t, "AI timeout", e.Error())
}

// --- writePendingURL edge cases ---

func TestWritePendingURLNil(t *testing.T) {
	deps := &dependencies{
		writer: &fakeWriter{},
	}
	result := writePendingURL(deps, t.TempDir(), nil, false)
	assert.Equal(t, StatusUnhandledError, result.Status)
}

func TestWritePendingURLUnhandled(t *testing.T) {
	deps := &dependencies{writer: &fakeWriter{}}
	pending := &pendingURLWrite{URL: "https://example.com", Kind: pendingUnhandled, Error: "some error"}
	result := writePendingURL(deps, t.TempDir(), pending, false)
	assert.Equal(t, StatusUnhandledError, result.Status)
	assert.Equal(t, "some error", result.Error)
}

func TestWritePendingURLDefault(t *testing.T) {
	deps := &dependencies{writer: &fakeWriter{}}
	pending := &pendingURLWrite{URL: "https://example.com", Kind: "invalid"}
	result := writePendingURL(deps, t.TempDir(), pending, false)
	assert.Equal(t, StatusUnhandledError, result.Status)
}

func TestWritePendingURLAIError(t *testing.T) {
	deps := &dependencies{writer: &fakeWriter{}}
	pending := &pendingURLWrite{URL: "https://example.com", Kind: pendingAIError, Error: "AI failed"}
	result := writePendingURL(deps, t.TempDir(), pending, false)
	assert.Equal(t, StatusFailureWritten, result.Status)
	assert.Equal(t, wikisvc.FailureAI, result.FailureType)
}

func TestWritePendingURLExtractFailure(t *testing.T) {
	deps := &dependencies{writer: &fakeWriter{}}
	item := &wikisvc.ClassifyItem{URL: "https://example.com", Title: "Test"}
	pending := &pendingURLWrite{URL: "https://example.com", Kind: pendingExtractFailure, Item: item, ExtraInfo: "too short"}
	result := writePendingURL(deps, t.TempDir(), pending, false)
	assert.Equal(t, StatusFailureWritten, result.Status)
	assert.Equal(t, wikisvc.FailureExtract, result.FailureType)
}

func TestWritePendingURLFetchFailure(t *testing.T) {
	deps := &dependencies{writer: &fakeWriter{}}
	pending := &pendingURLWrite{URL: "https://example.com", Kind: pendingFetchFailure, FailureType: wikisvc.FailureFetch, ExtraInfo: "timeout"}
	result := writePendingURL(deps, t.TempDir(), pending, false)
	assert.Equal(t, StatusFailureWritten, result.Status)
	assert.Equal(t, wikisvc.FailureFetch, result.FailureType)
}

func TestWritePendingURLSummaryDryRun(t *testing.T) {
	deps := &dependencies{writer: &fakeWriter{}}
	item := &wikisvc.ClassifyItem{
		URL:       "https://example.com",
		Title:     "Test",
		TopicPath: "topic/path",
		Type:      wikisvc.TypeDeepDive,
		Summary:   &wikisvc.StructuredSummary{Overview: "summary"},
	}
	pending := &pendingURLWrite{URL: "https://example.com", Kind: pendingSummary, Item: item}
	result := writePendingURL(deps, t.TempDir(), pending, true)
	assert.Equal(t, StatusDryRunSummary, result.Status)
	assert.True(t, result.Handled)
}

func TestWritePendingURLFailureDryRun(t *testing.T) {
	deps := &dependencies{writer: &fakeWriter{}}
	item := &wikisvc.ClassifyItem{URL: "https://example.com", Title: "Test"}
	pending := &pendingURLWrite{URL: "https://example.com", Kind: pendingClassifyFailure, Item: item, ExtraInfo: "info"}
	result := writePendingURL(deps, t.TempDir(), pending, true)
	assert.Equal(t, StatusDryRunFailure, result.Status)
	assert.True(t, result.Handled)
}

// --- Pipeline helpers ---

func TestPendingExtractFailureWrite(t *testing.T) {
	item := &wikisvc.ClassifyItem{URL: "https://example.com", Title: "Test"}
	p := pendingExtractFailureWrite(item, "too short")
	assert.Equal(t, pendingExtractFailure, p.Kind)
	assert.Equal(t, wikisvc.FailureExtract, p.FailureType)
	assert.Equal(t, "too short", p.ExtraInfo)
}

func TestNewPendingAIError(t *testing.T) {
	p := newPendingAIError("https://example.com", "AI failed")
	assert.Equal(t, pendingAIError, p.Kind)
	assert.Equal(t, "AI failed", p.Error)
}

func TestNewPendingUnhandled(t *testing.T) {
	p := newPendingUnhandled("https://example.com", "unknown error")
	assert.Equal(t, pendingUnhandled, p.Kind)
	assert.Equal(t, "unknown error", p.Error)
}

// --- serviceWriter and serviceInboxStore ---

func TestServiceWriterWriteSummary(t *testing.T) {
	root := t.TempDir()
	w := serviceWriter{}
	path, err := w.WriteSummary(&wikisvc.ClassifyItem{
		URL:       "https://example.com",
		Title:     "Test",
		TopicPath: "topic/path",
		Type:      wikisvc.TypeDeepDive,
		Summary:   &wikisvc.StructuredSummary{Overview: "summary"},
	}, &wikisvc.WriteOptions{WikiRoot: root})
	require.NoError(t, err)
	assert.NotEmpty(t, path)
}

func TestServiceWriterWriteFailureEntry(t *testing.T) {
	root := t.TempDir()
	w := serviceWriter{}
	path, err := w.WriteFailureEntry(
		&wikisvc.ClassifyItem{URL: "https://example.com"},
		wikisvc.FailureFetch, "timeout",
		&wikisvc.WriteOptions{WikiRoot: root},
	)
	require.NoError(t, err)
	assert.NotEmpty(t, path)
}

func TestServiceInboxStoreParseInbox(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inbox.md")
	require.NoError(t, os.WriteFile(path, []byte("- https://example.com/a\n"), 0o600))
	s := serviceInboxStore{}
	entries, err := s.ParseInbox(path)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}

func TestServiceWriterWriteManualReviewEntry(t *testing.T) {
	root := t.TempDir()
	w := serviceWriter{}
	path, err := w.WriteManualReviewEntry(
		&wikisvc.ClassifyItem{
			URL:       "https://example.com",
			Title:     "Test",
			TopicPath: "topic/path",
			Type:      wikisvc.TypeInbox,
			Summary:   &wikisvc.StructuredSummary{Overview: "needs review"},
		},
		&wikisvc.WriteOptions{WikiRoot: root},
	)
	require.NoError(t, err)
	assert.NotEmpty(t, path)
}

func TestServiceInboxStoreFlushInbox(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inbox.md")
	require.NoError(t, os.WriteFile(path, []byte("- https://example.com/a\n- https://example.com/b\n"), 0o600))
	s := serviceInboxStore{}
	err := s.FlushInbox(path, map[int][]string{0: {"https://example.com/a"}})
	require.NoError(t, err)
	data, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.NotContains(t, string(data), "https://example.com/a")
	assert.Contains(t, string(data), "https://example.com/b")
}

// --- RunAddURLs edge cases ---

func TestRunAddURLsNilConfig(t *testing.T) {
	_, err := RunAddURLs(context.Background(), AddInput{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config is required")
}

func TestRunAddURLsNoURLs(t *testing.T) {
	_, err := RunAddURLs(context.Background(), AddInput{Config: testConfig(t)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one URL")
}

// --- RunDigest edge cases ---

func TestRunDigestNilConfig(t *testing.T) {
	_, err := RunDigest(context.Background(), DigestInput{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config is required")
}

func TestRunDigestNoInboxFile(t *testing.T) {
	_, err := RunDigest(context.Background(), DigestInput{Config: testConfig(t)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "inbox file")
}

func TestRunDigestEmptyInbox(t *testing.T) {
	deps := newFakeDeps()
	deps.inbox.entries = nil
	cfg := testConfig(t)
	require.NoError(t, os.WriteFile(filepath.Join(cfg.Wiki.WikiRoot, "inbox.md"), []byte(""), 0o600))
	result, err := RunDigest(context.Background(), DigestInput{Config: cfg, deps: deps.dependencies()})
	require.NoError(t, err)
	assert.True(t, result.OK())
	assert.Empty(t, result.URLResults)
}

// --- RunAudit edge cases ---

func TestRunAuditNilConfig(t *testing.T) {
	_, err := RunAudit(context.Background(), AuditInput{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config is required")
}

// --- LoadConfig edge cases ---

func TestLoadConfigInvalidFile(t *testing.T) {
	_, err := LoadConfig("/tmp/nonexistent-config-12345.yml", "")
	require.Error(t, err)
}

func TestLoadConfigInvalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.yml")
	require.NoError(t, os.WriteFile(path, []byte("wiki:\n  concurrency: not-a-number"), 0o600))
	_, err := LoadConfig(path, "")
	require.Error(t, err)
}

// --- formatConfigLoadError ---

func TestFormatConfigLoadErrorStageRead(t *testing.T) {
	err := &configutil.LoadError{Stage: configutil.StageRead, Err: errors.New("file missing")}
	result := formatConfigLoadError(err)
	require.Error(t, result)
	assert.Contains(t, result.Error(), "read config")
	assert.Contains(t, result.Error(), "file missing")
}

func TestFormatConfigLoadErrorStageParse(t *testing.T) {
	err := &configutil.LoadError{Stage: configutil.StageParse, Err: errors.New("bad yaml")}
	result := formatConfigLoadError(err)
	require.Error(t, result)
	assert.Contains(t, result.Error(), "parse config")
	assert.Contains(t, result.Error(), "bad yaml")
}

func TestFormatConfigLoadErrorStageUnmarshal(t *testing.T) {
	err := &configutil.LoadError{Stage: configutil.StageUnmarshal, Err: errors.New("type mismatch")}
	result := formatConfigLoadError(err)
	require.Error(t, result)
	assert.Contains(t, result.Error(), "unmarshal config")
	assert.Contains(t, result.Error(), "type mismatch")
}

func TestFormatConfigLoadErrorStageValidate(t *testing.T) {
	err := &configutil.LoadError{Stage: configutil.StageValidate, Err: errors.New("required field")}
	result := formatConfigLoadError(err)
	require.Error(t, result)
	assert.Contains(t, result.Error(), "validate config")
	assert.Contains(t, result.Error(), "required field")
}

func TestFormatConfigLoadErrorUnknownStage(t *testing.T) {
	err := &configutil.LoadError{Stage: "unknown", Err: errors.New("something")}
	result := formatConfigLoadError(err)
	require.Error(t, result)
	// Unknown stage returns the original error
	assert.Contains(t, result.Error(), "something")
}

// --- RunDigest parse inbox error ---

func TestRunDigestParseInboxError(t *testing.T) {
	deps := newFakeDeps()
	deps.inbox.parseErr = errors.New("parse failed")
	cfg := testConfig(t)
	require.NoError(t, os.WriteFile(filepath.Join(cfg.Wiki.WikiRoot, "inbox.md"), []byte("bad content"), 0o600))

	_, err := RunDigest(context.Background(), DigestInput{Config: cfg, deps: deps.dependencies()})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse inbox")
}

// --- RunDigest flush inbox error ---

func TestRunDigestFlushInboxError(t *testing.T) {
	deps := newFakeDeps()
	deps.inbox.entries = []wikisvc.InboxEntry{{URL: "https://example.com/a", LineIndex: 1}}
	deps.inbox.flushErr = errors.New("flush failed")
	deps.fetcher.results["https://example.com/a"] = &wikisvc.ContentFetchResult{Title: "A", Body: "body"}
	deps.classifier.results["https://example.com/a"] = &wikisvc.ClassifyResult{
		TopicPath:   "topic/path",
		WikiType:    wikisvc.TypeDeepDive,
		ContentType: wikisvc.ContentText,
		Summary:     &wikisvc.StructuredSummary{Overview: "summary"},
	}
	cfg := testConfig(t)
	require.NoError(t, os.WriteFile(filepath.Join(cfg.Wiki.WikiRoot, "inbox.md"), []byte("- https://example.com/a\n"), 0o600))

	result, err := RunDigest(context.Background(), DigestInput{Config: cfg, deps: deps.dependencies()})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "flush inbox")
	// Result is still returned even on flush error
	assert.NotNil(t, result)
}

// --- RunAudit changed-only but no RunCmd ---

func TestRunAuditChangedOnlyNoRunCmd(t *testing.T) {
	cfg := testConfig(t)
	// No RunCmd provided but ChangedOnly=true → will fail in changedWikiMarkdownPaths
	// because it uses a nil CommandRunner
	_, err := RunAudit(context.Background(), AuditInput{
		Config:      cfg,
		ChangedOnly: true,
	})
	require.Error(t, err)
}

// --- prepareURLAttempt ---

func TestPrepareURLAttemptNilFetchResult(t *testing.T) {
	deps := newFakeDeps()
	// Configure fetcher to return nil
	deps.fetcher.results["https://example.com/a"] = nil
	deps.fetcher.returnNil = true

	_, err := prepareURLAttempt(context.Background(), deps.dependencies(), "https://example.com/a")
	require.Error(t, err)
	var fetchErr *fetchFailureError
	assert.ErrorAs(t, err, &fetchErr)
	assert.Equal(t, wikisvc.FailureFetch, fetchErr.failureType)
}

func TestPrepareURLAttemptEmptyFetchResultError(t *testing.T) {
	deps := newFakeDeps()
	deps.fetcher.results["https://example.com/a"] = &wikisvc.ContentFetchResult{
		Error:       "connection refused",
		FailureKind: wikisvc.FailureFetch,
	}

	_, err := prepareURLAttempt(context.Background(), deps.dependencies(), "https://example.com/a")
	require.Error(t, err)
	var fetchErr *fetchFailureError
	assert.ErrorAs(t, err, &fetchErr)
}

func TestPrepareURLAttemptVideoContentTooShort(t *testing.T) {
	deps := newFakeDeps()
	deps.fetcher.results["https://www.bilibili.com/video/BV1abc/"] = &wikisvc.ContentFetchResult{
		Title: "Video",
		Body:  "short", // < 600 runes
	}

	result, err := prepareURLAttempt(context.Background(), deps.dependencies(), "https://www.bilibili.com/video/BV1abc/")
	require.NoError(t, err)
	assert.Equal(t, pendingExtractFailure, result.Kind)
	assert.Contains(t, result.ExtraInfo, "too short")
}

func TestPrepareURLAttemptNilClassifierEmptyContent(t *testing.T) {
	deps := newFakeDeps()
	deps.fetcher.results["https://example.com/a"] = &wikisvc.ContentFetchResult{
		Title: "Title",
		Body:  "  ", // whitespace-only → empty after trim
	}
	// No classifier result set → returns nil

	result, err := prepareURLAttempt(context.Background(), deps.dependencies(), "https://example.com/a")
	require.NoError(t, err)
	assert.Equal(t, pendingExtractFailure, result.Kind)
	assert.Contains(t, result.ExtraInfo, "empty content")
}

func TestPrepareURLAttemptNilClassifierNonEmptyContent(t *testing.T) {
	deps := newFakeDeps()
	deps.fetcher.results["https://example.com/a"] = &wikisvc.ContentFetchResult{
		Title: "Title",
		Body:  "some real content here",
	}
	// No classifier result → AI unavailable pending (handled failure, not unhandled err)

	pending, err := prepareURLAttempt(context.Background(), deps.dependencies(), "https://example.com/a")
	require.NoError(t, err)
	assert.Equal(t, pendingAIError, pending.Kind)
	assert.Contains(t, pending.Error, "AI classify unavailable")
}

func TestPrepareURLAttemptClassifyFailure(t *testing.T) {
	deps := newFakeDeps()
	deps.fetcher.results["https://example.com/a"] = &wikisvc.ContentFetchResult{
		Title: "Title",
		Body:  "some real content here",
	}
	deps.classifier.results["https://example.com/a"] = &wikisvc.ClassifyResult{
		TopicPath:    "none",
		WikiType:     wikisvc.TypeInbox,
		RejectReason: "low quality",
	}

	result, err := prepareURLAttempt(context.Background(), deps.dependencies(), "https://example.com/a")
	require.NoError(t, err)
	assert.Equal(t, pendingClassifyFailure, result.Kind)
}

// --- prepareInboxEntry ---

func TestPrepareInboxEntryFetchFailureError(t *testing.T) {
	deps := newFakeDeps()
	deps.fetcher.results["https://example.com/a"] = &wikisvc.ContentFetchResult{
		Error:       "resolve: HTTP 403",
		FailureKind: wikisvc.FailureResolve,
	}

	entry := wikisvc.InboxEntry{URL: "https://example.com/a", LineIndex: 0}
	inboxCfg := inboxConfig{concurrency: 1, perURLTimeout: 60 * time.Second}
	result := prepareInboxEntry(context.Background(), deps.dependencies(), entry, inboxCfg)
	assert.Equal(t, pendingFetchFailure, result.Kind)
	assert.Equal(t, wikisvc.FailureResolve, result.FailureType)
}

func TestPrepareInboxEntryClassifyRetryError(t *testing.T) {
	deps := newFakeDeps()
	deps.fetcher.results["https://example.com/a"] = &wikisvc.ContentFetchResult{
		Title: "Title",
		Body:  "some real content here",
	}
	// No classifier result → AI error JSONL path (not raw uncat dump)

	entry := wikisvc.InboxEntry{URL: "https://example.com/a", LineIndex: 0}
	inboxCfg := inboxConfig{concurrency: 1, perURLTimeout: 60 * time.Second}
	result := prepareInboxEntry(context.Background(), deps.dependencies(), entry, inboxCfg)
	assert.Equal(t, pendingAIError, result.Kind)
	assert.Contains(t, result.Error, "AI classify unavailable")
	assert.Nil(t, result.Item)
}

// --- processAddURL ---

func TestProcessAddURLFetchFailure(t *testing.T) {
	deps := newFakeDeps()
	deps.fetcher.results["https://example.com/a"] = &wikisvc.ContentFetchResult{
		Error:       "resolve: HTTP 403",
		FailureKind: wikisvc.FailureResolve,
	}

	result := processAddURL(context.Background(), deps.dependencies(), t.TempDir(), "https://example.com/a", false)
	assert.Equal(t, StatusFailureWritten, result.Status)
	assert.Equal(t, wikisvc.FailureResolve, result.FailureType)
}

func TestProcessAddURLUnhandledError(t *testing.T) {
	deps := newFakeDeps()
	deps.fetcher.results["https://example.com/a"] = nil
	deps.fetcher.returnNil = true

	result := processAddURL(context.Background(), deps.dependencies(), t.TempDir(), "https://example.com/a", false)
	// nil fetch result → fetchFailureError → writePendingURL → failure written
	assert.Equal(t, StatusFailureWritten, result.Status)
}

// --- writeClassifyFailure ---

func TestWriteClassifyFailureWriterError(t *testing.T) {
	deps := newFakeDeps()
	deps.writer.failureErr = errors.New("disk full")
	item := &wikisvc.ClassifyItem{URL: "https://example.com", Title: "Test"}

	result := writeClassifyFailure(deps.dependencies(), t.TempDir(), item, "info", false)
	assert.Equal(t, StatusUnhandledError, result.Status)
	assert.Contains(t, result.Error, "disk full")
	assert.Equal(t, wikisvc.FailureClassify, result.FailureType)
}

// --- writeExtractFailure ---

func TestWriteExtractFailureWriterError(t *testing.T) {
	deps := newFakeDeps()
	deps.writer.failureErr = errors.New("write failed")
	item := &wikisvc.ClassifyItem{URL: "https://example.com", Title: "Test"}

	result := writeExtractFailure(deps.dependencies(), t.TempDir(), item, "info", false)
	assert.Equal(t, StatusUnhandledError, result.Status)
	assert.Contains(t, result.Error, "write failed")
	assert.Equal(t, wikisvc.FailureExtract, result.FailureType)
}

func TestWriteExtractFailureDryRun(t *testing.T) {
	deps := newFakeDeps()
	item := &wikisvc.ClassifyItem{URL: "https://example.com", Title: "Test"}

	result := writeExtractFailure(deps.dependencies(), t.TempDir(), item, "info", true)
	assert.Equal(t, StatusDryRunFailure, result.Status)
	assert.True(t, result.Handled)
	assert.True(t, deps.writer.failures[0].dryRun)
}

// --- writeFetchFailure ---

func TestWriteFetchFailureWriterError(t *testing.T) {
	deps := newFakeDeps()
	deps.writer.failureErr = errors.New("write failed")

	result := writeFetchFailure(deps.dependencies(), t.TempDir(), "https://example.com", wikisvc.FailureFetch, "timeout", false)
	assert.Equal(t, StatusUnhandledError, result.Status)
	assert.Contains(t, result.Error, "write failed")
	assert.Equal(t, wikisvc.FailureFetch, result.FailureType)
}

func TestWriteFetchFailureDryRun(t *testing.T) {
	deps := newFakeDeps()

	result := writeFetchFailure(deps.dependencies(), t.TempDir(), "https://example.com", wikisvc.FailureFetch, "timeout", true)
	assert.Equal(t, StatusDryRunFailure, result.Status)
	assert.True(t, result.Handled)
	assert.True(t, deps.writer.failures[0].dryRun)
}

// --- writeAIError ---

func TestWriteAIErrorWriterError(t *testing.T) {
	deps := newFakeDeps()
	deps.writer.failureErr = errors.New("write failed")

	result := writeAIError(deps.dependencies(), t.TempDir(), "https://example.com", "AI timeout", false)
	assert.Equal(t, StatusUnhandledError, result.Status)
	assert.Contains(t, result.Error, "write failed")
	assert.Equal(t, wikisvc.FailureAI, result.FailureType)
}

func TestWriteAIErrorDryRun(t *testing.T) {
	deps := newFakeDeps()

	result := writeAIError(deps.dependencies(), t.TempDir(), "https://example.com", "AI timeout", true)
	assert.Equal(t, StatusDryRunFailure, result.Status)
	assert.True(t, result.Handled)
	assert.True(t, deps.writer.failures[0].dryRun)
}

// --- writeSummary ---

func TestWriteSummaryNeedsManualReview(t *testing.T) {
	deps := newFakeDeps()
	// NMR + legal topic path + overview → promote to topic write (recall)
	item := &wikisvc.ClassifyItem{
		URL:               "https://example.com",
		Title:             "Test",
		TopicPath:         "topic/path",
		Type:              wikisvc.TypeDeepDive,
		Summary:           &wikisvc.StructuredSummary{Overview: "summary"},
		NeedsManualReview: true,
	}

	result := writeSummary(deps.dependencies(), t.TempDir(), item, false)
	assert.Equal(t, StatusSummaryWritten, result.Status)
	assert.True(t, result.Handled)
	assert.Contains(t, result.OutputPath, "summary.md")
	assert.NotContains(t, result.OutputPath, "uncat.md")
	assert.False(t, item.NeedsManualReview)
}

func TestWriteSummaryNeedsManualReviewNoTopicGoesUncat(t *testing.T) {
	deps := newFakeDeps()
	item := &wikisvc.ClassifyItem{
		URL:               "https://example.com",
		Title:             "Test",
		TopicPath:         "",
		Type:              wikisvc.TypeInbox,
		Summary:           &wikisvc.StructuredSummary{Overview: "summary"},
		NeedsManualReview: true,
		RouteReason:       wikisvc.RouteReasonNoTopicMatch,
	}

	result := writeSummary(deps.dependencies(), t.TempDir(), item, false)
	assert.Equal(t, StatusSummaryWritten, result.Status)
	assert.True(t, result.Handled)
	assert.Contains(t, result.OutputPath, "uncat.md")
}

func TestWriteSummaryManualReviewWriterError(t *testing.T) {
	deps := newFakeDeps()
	deps.writer.failureErr = errors.New("disk full")
	// No topic path → stays on uncat path and hits WriteManualReviewEntry error
	item := &wikisvc.ClassifyItem{
		URL:               "https://example.com",
		Title:             "Test",
		TopicPath:         "",
		Type:              wikisvc.TypeInbox,
		Summary:           &wikisvc.StructuredSummary{Overview: "summary"},
		NeedsManualReview: true,
	}

	result := writeSummary(deps.dependencies(), t.TempDir(), item, false)
	assert.Equal(t, StatusUnhandledError, result.Status)
	assert.Contains(t, result.Error, "disk full")
}

func TestWriteSummaryWriterError(t *testing.T) {
	deps := newFakeDeps()
	deps.writer.summaryErr = errors.New("write failed")
	item := &wikisvc.ClassifyItem{
		URL:       "https://example.com",
		Title:     "Test",
		TopicPath: "topic/path",
		Type:      wikisvc.TypeDeepDive,
		Summary:   &wikisvc.StructuredSummary{Overview: "summary"},
	}

	result := writeSummary(deps.dependencies(), t.TempDir(), item, false)
	assert.Equal(t, StatusUnhandledError, result.Status)
	assert.Contains(t, result.Error, "write failed")
}

// --- requireDir non-IsNotExist stat error ---

func TestRequireDirStatError(t *testing.T) {
	// Use a path with null byte to trigger a stat error that's not IsNotExist
	err := requireDir("/tmp/\x00invalid-path", "test dir")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stat test dir")
}

// --- writeSummary manual review dry run ---

func TestWriteSummaryNeedsManualReviewDryRun(t *testing.T) {
	deps := newFakeDeps()
	item := &wikisvc.ClassifyItem{
		URL:               "https://example.com",
		Title:             "Test",
		TopicPath:         "topic/path",
		Type:              wikisvc.TypeDeepDive,
		Summary:           &wikisvc.StructuredSummary{Overview: "summary"},
		NeedsManualReview: true,
	}

	result := writeSummary(deps.dependencies(), t.TempDir(), item, true)
	assert.Equal(t, StatusDryRunSummary, result.Status)
	assert.True(t, result.Handled)
}

// --- RunAddURLs wikiRoot not found ---

func TestRunAddURLsWikiRootNotFound(t *testing.T) {
	cfg := &Config{
		AI:   AIConfig{Model: "model", BaseURL: "https://example.com/v1"},
		Wiki: WikiConfig{WikiRoot: "/tmp/nonexistent-wiki-root-12345"},
	}
	_, err := RunAddURLs(context.Background(), AddInput{
		Config: cfg,
		URLs:   []string{"https://example.com/a"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "wiki root")
}

// --- RunDigest wikiRoot not found ---

func TestRunDigestWikiRootNotFound(t *testing.T) {
	cfg := &Config{
		AI:   AIConfig{Model: "model", BaseURL: "https://example.com/v1"},
		Wiki: WikiConfig{WikiRoot: "/tmp/nonexistent-wiki-root-12345"},
	}
	_, err := RunDigest(context.Background(), DigestInput{Config: cfg})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "wiki root")
}

// --- RunAudit wikiRoot not found ---

func TestRunAuditWikiRootNotFound(t *testing.T) {
	cfg := &Config{
		AI:   AIConfig{Model: "model", BaseURL: "https://example.com/v1"},
		Wiki: WikiConfig{WikiRoot: "/tmp/nonexistent-wiki-root-12345"},
	}
	_, err := RunAudit(context.Background(), AuditInput{Config: cfg})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "wiki root")
}

// --- prepareURLAttempt title fallback ---

func TestPrepareURLAttemptTitleFallback(t *testing.T) {
	deps := newFakeDeps()
	deps.fetcher.results["https://example.com/a"] = &wikisvc.ContentFetchResult{
		Title: "", // empty title → falls back to URL
		Body:  "some real content here with enough data for classification",
	}
	deps.classifier.results["https://example.com/a"] = &wikisvc.ClassifyResult{
		TopicPath:   "topic/path",
		WikiType:    wikisvc.TypeDeepDive,
		ContentType: wikisvc.ContentText,
		Summary:     &wikisvc.StructuredSummary{Overview: "summary"},
	}

	result, err := prepareURLAttempt(context.Background(), deps.dependencies(), "https://example.com/a")
	require.NoError(t, err)
	assert.NotNil(t, result.Item)
	assert.Equal(t, "https://example.com/a", result.Item.Title)
}

// --- processAddURL unhandled error ---

func TestProcessAddURLUnhandledClassifyRetryError(t *testing.T) {
	deps := newFakeDeps()
	deps.fetcher.results["https://example.com/a"] = &wikisvc.ContentFetchResult{
		Title: "Title",
		Body:  "some real content",
	}
	// No classifier result → AI failure JSONL (handled), not unhandled

	result := processAddURL(context.Background(), deps.dependencies(), t.TempDir(), "https://example.com/a", false)
	assert.Equal(t, StatusFailureWritten, result.Status)
	assert.True(t, result.Handled)
	assert.Equal(t, wikisvc.FailureAI, result.FailureType)
}

// --- RunDigest no entries handled (unhandled error) ---

func TestRunDigestNoEntriesHandled(t *testing.T) {
	deps := newFakeDeps()
	deps.inbox.entries = []wikisvc.InboxEntry{{URL: "https://example.com/a", LineIndex: 1}}
	deps.fetcher.returnNil = true
	cfg := testConfig(t)
	require.NoError(t, os.WriteFile(filepath.Join(cfg.Wiki.WikiRoot, "inbox.md"), []byte("- https://example.com/a\n"), 0o600))

	result, err := RunDigest(context.Background(), DigestInput{Config: cfg, deps: deps.dependencies()})
	require.NoError(t, err)
	// fetchFailureError from nil result is retryable; after retries, it writes a failure entry (Handled=true)
	assert.True(t, result.OK())
	assert.Equal(t, 1, result.Flushed)
}

// --- LoadConfig validation error ---

// --- resolveDependencies coverage ---

func TestResolveDependenciesNilDeps(t *testing.T) {
	cfg := testConfig(t)
	deps := resolveDependencies(cfg, nil)
	assert.NotNil(t, deps)
	assert.NotNil(t, deps.fetcher)
	assert.NotNil(t, deps.classifier)
	assert.NotNil(t, deps.writer)
	assert.NotNil(t, deps.inbox)
}

func TestResolveDependenciesPartialDeps(t *testing.T) {
	cfg := testConfig(t)
	deps := resolveDependencies(cfg, &dependencies{
		fetcher: &fakeFetcher{results: map[string]*wikisvc.ContentFetchResult{}},
	})
	assert.NotNil(t, deps)
	assert.NotNil(t, deps.fetcher)
	assert.NotNil(t, deps.classifier)
	assert.NotNil(t, deps.writer)
	assert.NotNil(t, deps.inbox)
}

// --- changedMarkdownPathsFromGit git error ---

func TestChangedMarkdownPathsFromGitGitError(t *testing.T) {
	runCmd := func(_ context.Context, _, _ string, _ ...string) ([]byte, error) {
		return []byte("error"), errors.New("git failed")
	}
	_, err := changedMarkdownPathsFromGit(context.Background(), "/repo", "wiki", runCmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list changed wiki files")
}

// --- RunDigestLocal with non-dir entries (skip non-directory) ---

func TestRunDigestLocalSkipsNonDirEntries(t *testing.T) {
	cfg := testConfig(t)
	deps := newFakeDeps()

	fromDir := t.TempDir()
	// Create a regular file (not a directory)
	require.NoError(t, os.WriteFile(filepath.Join(fromDir, "not-a-dir.txt"), []byte("file"), 0o600))
	// Create a valid subdirectory
	subDir := filepath.Join(fromDir, "BV1abc123_Title")
	require.NoError(t, os.MkdirAll(subDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "bv.txt"), []byte("BV1abc123"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "title.txt"), []byte("Title"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "transcript.md"), []byte(strings.Repeat("transcript content here. ", 50)), 0o600))
	deps.classifier.results["https://www.bilibili.com/video/BV1abc123/"] = &wikisvc.ClassifyResult{
		TopicPath:   "tech/ai",
		WikiType:    wikisvc.TypeDeepDive,
		ContentType: wikisvc.ContentVideo,
		Summary:     &wikisvc.StructuredSummary{Overview: "summary"},
	}

	result, err := RunDigestLocal(context.Background(), DigestLocalInput{
		Config:  cfg,
		FromDir: fromDir,
		deps:    deps.dependencies(),
	})

	require.NoError(t, err)
	// Should have processed only the directory, not the file
	assert.Len(t, result.URLResults, 1)
	assert.Equal(t, StatusSummaryWritten, result.URLResults[0].Status)
}

// --- processLocalDir with classify failure (nil classifier, non-empty content, bilibili URL with >= 600 chars) ---

func TestProcessLocalDirClassifierNilNonEmptyContent(t *testing.T) {
	deps := newFakeDeps()
	// No classifier result → nil

	dir := t.TempDir()
	wikiRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bv.txt"), []byte("BV1abc123"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "title.txt"), []byte("Title"), 0o600))
	// >= 600 runes of non-whitespace content → passes video quality gate, then classifier nil → unhandled error
	require.NoError(t, os.WriteFile(filepath.Join(dir, "transcript.md"), []byte(strings.Repeat("real content for transcript. ", 30)), 0o600))

	result := processLocalDir(context.Background(), deps.dependencies(), wikiRoot, dir)
	assert.Equal(t, StatusUnhandledError, result.Status)
	assert.Contains(t, result.Error, "classification failed")
}

// --- copyTranscriptToWiki MkdirAll and WriteFile errors ---

func TestCopyTranscriptToWikiMkdirAllError(t *testing.T) {
	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "transcript.md"), []byte("data"), 0o600))
	// Use a file as wikiRoot so MkdirAll fails
	wikiRoot := filepath.Join(t.TempDir(), "file-as-root")
	require.NoError(t, os.WriteFile(wikiRoot, []byte(""), 0o600))

	err := copyTranscriptToWiki(srcDir, wikiRoot, "tech/ai", "BV1abc")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create transcript dir")
}

// --- changedWikiGitRoots filepath.Abs error ---

func TestChangedWikiGitRootsAbsError(t *testing.T) {
	// filepath.Abs can fail on some systems with invalid paths, but on Unix
	// it almost never fails. Test with empty string (returns cwd, no error).
	// Instead, test the gitOutput nil CommandRunner path inside changedWikiGitRoots.
	runCmd := func(_ context.Context, _, _ string, _ ...string) ([]byte, error) {
		return []byte(""), errors.New("git not found")
	}
	_, err := changedWikiGitRoots(context.Background(), t.TempDir(), runCmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "find git worktree")
}
