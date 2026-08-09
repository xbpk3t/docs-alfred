package wikiingest

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	wikitypes "github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
	wikiwrite "github.com/xbpk3t/docs-alfred/internal/docs/wiki/write"
)

// writeSuccessHistory writes a digest-success.jsonl under wikiRoot from the
// given URLs, using the same entry shape as LogSuccessEntry.
func writeSuccessHistory(t *testing.T, wikiRoot string, urls []string) {
	t.Helper()
	path := filepath.Join(wikiRoot, digestFileNameSuccess)
	var lines []byte
	for _, u := range urls {
		entry := wikitypes.DigestEntry{
			URL:        u,
			Timestamp:  "2026-01-01T00:00:00Z",
			Stage:      wikitypes.StageWrite,
			Status:     wikitypes.DigestSuccess,
			TopicPath:  "topic/path",
			OutputPath: "topic/path/summary.md",
		}
		b, err := json.Marshal(entry)
		require.NoError(t, err)
		lines = append(lines, b...)
		lines = append(lines, '\n')
	}
	require.NoError(t, os.WriteFile(path, lines, 0o600))
}

func TestLoadDigestHistorySkipsFailuresAndMalformed(t *testing.T) {
	wikiRoot := t.TempDir()
	path := filepath.Join(wikiRoot, digestFileNameSuccess)
	content := "" +
		`{"timestamp":"2026-01-01T00:00:00Z","url":"https://example.com/ok","status":"success","stage":"write"}` + "\n" +
		`{"timestamp":"2026-01-01T00:00:00Z","url":"https://example.com/failed","status":"failure","stage":"fetch"}` + "\n" +
		`not-json` + "\n" +
		`{"timestamp":"2026-01-01T00:00:00Z","url":"https://example.com/with?utm_source=x","status":"success","stage":"write"}` + "\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	h := loadDigestHistory(wikiRoot)
	require.True(t, h.alreadyDigested("https://example.com/ok"))
	// Failure entries are not remembered → retried.
	require.False(t, h.alreadyDigested("https://example.com/bad"))
	// Tracking-param variant of a success merges to the same key.
	require.True(t, h.alreadyDigested("https://example.com/with"))
}

func TestLoadDigestHistoryMissingFileYieldsEmpty(t *testing.T) {
	h := loadDigestHistory(t.TempDir())
	require.NotNil(t, h)
	require.False(t, h.alreadyDigested("https://example.com/a"))
}

func TestDigestSkipsAlreadyDigestedURL(t *testing.T) {
	deps := newFakeDeps()
	deps.inbox.entries = []wikiwrite.InboxEntry{{URL: "https://example.com/old", LineIndex: 1}}
	cfg := testConfig(t)
	writeSuccessHistory(t, cfg.Wiki.WikiRoot, []string{"https://example.com/old"})
	require.NoError(t, os.WriteFile(filepath.Join(cfg.Wiki.WikiRoot, "inbox.md"), []byte("- https://example.com/old\n"), 0o600))
	deps.history = loadDigestHistory(cfg.Wiki.WikiRoot)

	result, err := RunDigest(context.Background(), DigestInput{Config: cfg, deps: deps.dependencies()})
	require.NoError(t, err)
	require.True(t, result.OK())
	require.Len(t, result.URLResults, 1)
	require.Equal(t, StatusSkipped, result.URLResults[0].Status)
	require.True(t, result.URLResults[0].Handled)
	// Must not fetch or classify a skipped URL.
	require.Empty(t, deps.fetcher.results, "skipped URL must not be fetched")
	require.Empty(t, deps.writer.summaries)
	// Inbox line still flushed (skip counts as handled).
	require.Equal(t, 1, result.Flushed)
	require.Equal(t, []string{"https://example.com/old"}, deps.inbox.flushed[1])
}

func TestDigestSkipsTrackingParamVariantOfDigestedURL(t *testing.T) {
	deps := newFakeDeps()
	// History has the bare URL; inbox has the same page with tracking params.
	deps.inbox.entries = []wikiwrite.InboxEntry{{URL: "https://example.com/post?utm_source=nl&utm_medium=email", LineIndex: 1}}
	cfg := testConfig(t)
	writeSuccessHistory(t, cfg.Wiki.WikiRoot, []string{"https://example.com/post"})
	require.NoError(t, os.WriteFile(filepath.Join(cfg.Wiki.WikiRoot, "inbox.md"), []byte("- https://example.com/post?utm_source=nl&utm_medium=email\n"), 0o600))
	deps.history = loadDigestHistory(cfg.Wiki.WikiRoot)

	result, err := RunDigest(context.Background(), DigestInput{Config: cfg, deps: deps.dependencies()})
	require.NoError(t, err)
	require.Equal(t, StatusSkipped, result.URLResults[0].Status)
	require.True(t, result.URLResults[0].Handled)
	require.Equal(t, 1, result.Flushed)
	require.Empty(t, deps.writer.summaries)
}

func TestDigestDoesNotSkipContentParamVariant(t *testing.T) {
	deps := newFakeDeps()
	// History has ?v=abc; inbox has ?v=def — different content identifier, the
	// URL is a text path (not video) so the pipeline runs to a normal summary.
	deps.inbox.entries = []wikiwrite.InboxEntry{{URL: "https://example.com/watch?v=def", LineIndex: 1}}
	deps.fetcher.results["https://example.com/watch?v=def"] = &wikitypes.ContentFetchResult{Title: "B", Body: "body"}
	deps.classifier.results["https://example.com/watch?v=def"] = &wikitypes.ClassifyResult{
		TopicPath:   "topic/path",
		WikiType:    wikitypes.TypeDeepDive,
		ContentType: wikitypes.ContentText,
		Summary:     &wikitypes.StructuredSummary{Overview: "summary"},
	}
	cfg := testConfig(t)
	writeSuccessHistory(t, cfg.Wiki.WikiRoot, []string{"https://example.com/watch?v=abc"})
	require.NoError(t, os.WriteFile(filepath.Join(cfg.Wiki.WikiRoot, "inbox.md"), []byte("- https://example.com/watch?v=def\n"), 0o600))
	deps.history = loadDigestHistory(cfg.Wiki.WikiRoot)

	result, err := RunDigest(context.Background(), DigestInput{Config: cfg, deps: deps.dependencies()})
	require.NoError(t, err)
	require.Len(t, result.URLResults, 1)
	require.Equal(t, StatusSummaryWritten, result.URLResults[0].Status)
}

func TestDigestRetriesFailedURL(t *testing.T) {
	deps := newFakeDeps()
	// History reflects a previous failed run (fetch failure) for the same URL;
	// the URL must be retried, not skipped.
	deps.inbox.entries = []wikiwrite.InboxEntry{{URL: "https://example.com/flaky", LineIndex: 1}}
	deps.fetcher.results["https://example.com/flaky"] = &wikitypes.ContentFetchResult{Title: "A", Body: "body"}
	deps.classifier.results["https://example.com/flaky"] = &wikitypes.ClassifyResult{
		TopicPath:   "topic/path",
		WikiType:    wikitypes.TypeDeepDive,
		ContentType: wikitypes.ContentText,
		Summary:     &wikitypes.StructuredSummary{Overview: "summary"},
	}
	cfg := testConfig(t)
	// Only failure entries for this URL — success history is empty.
	failPath := filepath.Join(cfg.Wiki.WikiRoot, "digest-fetch-error.jsonl")
	require.NoError(t, os.WriteFile(failPath, []byte(`{"timestamp":"2026-01-01T00:00:00Z","url":"https://example.com/flaky","status":"failure","stage":"fetch"}`+"\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(cfg.Wiki.WikiRoot, "inbox.md"), []byte("- https://example.com/flaky\n"), 0o600))
	deps.history = loadDigestHistory(cfg.Wiki.WikiRoot)

	result, err := RunDigest(context.Background(), DigestInput{Config: cfg, deps: deps.dependencies()})
	require.NoError(t, err)
	require.Equal(t, StatusSummaryWritten, result.URLResults[0].Status)
	require.Equal(t, 1, result.Flushed)
}

func TestDigestDryRunSkipsWithoutFlushing(t *testing.T) {
	deps := newFakeDeps()
	deps.inbox.entries = []wikiwrite.InboxEntry{{URL: "https://example.com/old", LineIndex: 1}}
	cfg := testConfig(t)
	writeSuccessHistory(t, cfg.Wiki.WikiRoot, []string{"https://example.com/old"})
	require.NoError(t, os.WriteFile(filepath.Join(cfg.Wiki.WikiRoot, "inbox.md"), []byte("- https://example.com/old\n"), 0o600))
	deps.history = loadDigestHistory(cfg.Wiki.WikiRoot)

	result, err := RunDigest(context.Background(), DigestInput{Config: cfg, deps: deps.dependencies(), DryRun: true})
	require.NoError(t, err)
	require.Equal(t, StatusSkipped, result.URLResults[0].Status)
	require.Equal(t, 0, result.Flushed)
	require.Empty(t, deps.inbox.flushed)
}

func TestAddDoesNotConsultHistory(t *testing.T) {
	// wiki add is the explicit re-digest entry — history must not block it.
	deps := newFakeDeps()
	deps.fetcher.results["https://example.com/again"] = &wikitypes.ContentFetchResult{Title: "A", Body: "body"}
	deps.classifier.results["https://example.com/again"] = &wikitypes.ClassifyResult{
		TopicPath:   "topic/path",
		WikiType:    wikitypes.TypeDeepDive,
		ContentType: wikitypes.ContentText,
		Summary:     &wikitypes.StructuredSummary{Overview: "summary"},
	}
	cfg := testConfig(t)
	writeSuccessHistory(t, cfg.Wiki.WikiRoot, []string{"https://example.com/again"})
	// RunAddURLs resolves deps itself; give it the deps without history.

	result, err := RunAddURLs(context.Background(), AddInput{Config: cfg, URLs: []string{"https://example.com/again"}, deps: deps.dependencies()})
	require.NoError(t, err)
	require.Len(t, result.URLResults, 1)
	require.Equal(t, StatusSummaryWritten, result.URLResults[0].Status)
	require.Len(t, deps.writer.summaries, 1)
}
