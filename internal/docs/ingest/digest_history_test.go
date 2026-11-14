package wikiingest

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	wikitypes "github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
	wikiwrite "github.com/xbpk3t/docs-alfred/internal/docs/wiki/write"
)

// writeSuccessHistory seeds logs/digest-success.jsonl in the fake store through
// the real success-log writer, so the entry shape cannot drift.
func writeSuccessHistory(t *testing.T, store *fakeMediaStore, urls []string) {
	t.Helper()
	opts := &wikiwrite.WriteOptions{R2Store: digestStoreAdapter{store: store}}
	for _, u := range urls {
		_, err := wikiwrite.LogSuccessEntry(&wikitypes.ClassifyItem{
			URL:       u,
			TopicPath: "topic/path",
		}, "topic/path/summary.md", opts)
		require.NoError(t, err)
	}
}

func TestLoadDigestHistorySkipsFailuresAndMalformed(t *testing.T) {
	store := newFakeMediaStore()
	store.objects["logs/"+digestFileNameSuccess] = "" +
		`{"timestamp":"2026-01-01T00:00:00Z","url":"https://example.com/ok","status":"success","stage":"write"}` + "\n" +
		`{"timestamp":"2026-01-01T00:00:00Z","url":"https://example.com/failed","status":"failure","stage":"fetch"}` + "\n" +
		`not-json` + "\n" +
		`{"timestamp":"2026-01-01T00:00:00Z","url":"https://example.com/with?utm_source=x","status":"success","stage":"write"}` + "\n"

	h := loadDigestHistory(digestStoreAdapter{store: store})
	// Persisted success → already seen.
	require.True(t, h.claimOrSeen("https://example.com/ok"))
	// Failure entries are not remembered → treated as new (retried).
	require.False(t, h.claimOrSeen("https://example.com/bad"))
	// Tracking-param variant of a success merges to the same key.
	require.True(t, h.claimOrSeen("https://example.com/with"))
}

func TestLoadDigestHistoryMissingFileYieldsEmpty(t *testing.T) {
	h := loadDigestHistory(digestStoreAdapter{store: newFakeMediaStore()})
	require.NotNil(t, h)
	require.False(t, h.claimOrSeen("https://example.com/a"))
}

func TestDigestSkipsAlreadyDigestedURL(t *testing.T) {
	deps := newFakeDeps()
	deps.inbox.entries = []wikiwrite.InboxEntry{{URL: "https://example.com/old", LineIndex: 1}}
	cfg := testConfig(t)
	writeSuccessHistory(t, deps.r2, []string{"https://example.com/old"})
	require.NoError(t, os.WriteFile(filepath.Join(cfg.Wiki.WikiRoot, "inbox.md"), []byte("- https://example.com/old\n"), 0o600))
	deps.history = loadDigestHistory(digestStoreAdapter{store: deps.r2})

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
	writeSuccessHistory(t, deps.r2, []string{"https://example.com/post"})
	require.NoError(t, os.WriteFile(filepath.Join(cfg.Wiki.WikiRoot, "inbox.md"), []byte("- https://example.com/post?utm_source=nl&utm_medium=email\n"), 0o600))
	deps.history = loadDigestHistory(digestStoreAdapter{store: deps.r2})

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
	writeSuccessHistory(t, deps.r2, []string{"https://example.com/watch?v=abc"})
	require.NoError(t, os.WriteFile(filepath.Join(cfg.Wiki.WikiRoot, "inbox.md"), []byte("- https://example.com/watch?v=def\n"), 0o600))
	deps.history = loadDigestHistory(digestStoreAdapter{store: deps.r2})

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
	deps.r2.objects["logs/digest-fetch-error.jsonl"] = `{"timestamp":"2026-01-01T00:00:00Z","url":"https://example.com/flaky","status":"failure","stage":"fetch"}` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(cfg.Wiki.WikiRoot, "inbox.md"), []byte("- https://example.com/flaky\n"), 0o600))
	deps.history = loadDigestHistory(digestStoreAdapter{store: deps.r2})

	result, err := RunDigest(context.Background(), DigestInput{Config: cfg, deps: deps.dependencies()})
	require.NoError(t, err)
	require.Equal(t, StatusSummaryWritten, result.URLResults[0].Status)
	require.Equal(t, 1, result.Flushed)
}

func TestDigestDryRunSkipsWithoutFlushing(t *testing.T) {
	deps := newFakeDeps()
	deps.inbox.entries = []wikiwrite.InboxEntry{{URL: "https://example.com/old", LineIndex: 1}}
	cfg := testConfig(t)
	writeSuccessHistory(t, deps.r2, []string{"https://example.com/old"})
	require.NoError(t, os.WriteFile(filepath.Join(cfg.Wiki.WikiRoot, "inbox.md"), []byte("- https://example.com/old\n"), 0o600))
	deps.history = loadDigestHistory(digestStoreAdapter{store: deps.r2})

	result, err := RunDigest(context.Background(), DigestInput{Config: cfg, deps: deps.dependencies(), DryRun: true})
	require.NoError(t, err)
	require.Equal(t, StatusSkipped, result.URLResults[0].Status)
	require.Equal(t, 0, result.Flushed)
	require.Empty(t, deps.inbox.flushed)
}

func TestAddSkipsAlreadyDigestedURL(t *testing.T) {
	// wiki add now consults history: an already digested URL is skipped by default.
	deps := newFakeDeps()
	cfg := testConfig(t)
	writeSuccessHistory(t, deps.r2, []string{"https://example.com/again"})
	// RunAddURLs resolves deps itself; history is loaded from the wiki root.

	result, err := RunAddURLs(context.Background(), AddInput{Config: cfg, URLs: []string{"https://example.com/again"}, deps: deps.dependencies()})
	require.NoError(t, err)
	require.Len(t, result.URLResults, 1)
	require.Equal(t, StatusSkipped, result.URLResults[0].Status)
	// Must not write a skipped URL.
	require.Empty(t, deps.writer.summaries)
}

func TestAddForceRedigestsAlreadyDigestedURL(t *testing.T) {
	// --force opts into an explicit re-digest even though the URL is in history.
	deps := newFakeDeps()
	deps.fetcher.results["https://example.com/again"] = &wikitypes.ContentFetchResult{Title: "A", Body: "body"}
	deps.classifier.results["https://example.com/again"] = &wikitypes.ClassifyResult{
		TopicPath:   "topic/path",
		WikiType:    wikitypes.TypeDeepDive,
		ContentType: wikitypes.ContentText,
		Summary:     &wikitypes.StructuredSummary{Overview: "summary"},
	}
	cfg := testConfig(t)
	writeSuccessHistory(t, deps.r2, []string{"https://example.com/again"})

	result, err := RunAddURLs(context.Background(), AddInput{Config: cfg, URLs: []string{"https://example.com/again"}, Force: true, deps: deps.dependencies()})
	require.NoError(t, err)
	require.Len(t, result.URLResults, 1)
	require.Equal(t, StatusSummaryWritten, result.URLResults[0].Status)
	require.Len(t, deps.writer.summaries, 1)
}

func TestAddForceStillCollapsesDuplicatesWithinBatch(t *testing.T) {
	// Duplicates within a single add call collapse even under --force: force only
	// skips consulting persisted history (re-digest), never the in-run claim set.
	url := "https://example.com/dup"
	deps := newFakeDeps()
	deps.fetcher.results[url] = &wikitypes.ContentFetchResult{Title: "A", Body: "body"}
	deps.classifier.results[url] = &wikitypes.ClassifyResult{
		TopicPath:   "topic/path",
		WikiType:    wikitypes.TypeDeepDive,
		ContentType: wikitypes.ContentText,
		Summary:     &wikitypes.StructuredSummary{Overview: "summary"},
	}
	cfg := testConfig(t)
	writeSuccessHistory(t, deps.r2, []string{url})

	result, err := RunAddURLs(context.Background(), AddInput{Config: cfg, URLs: []string{url, url}, Force: true, deps: deps.dependencies()})
	require.NoError(t, err)
	require.Len(t, result.URLResults, 2)
	require.Equal(t, StatusSummaryWritten, result.URLResults[0].Status)
	require.Equal(t, StatusSkipped, result.URLResults[1].Status)
	require.Len(t, deps.writer.summaries, 1)
}

func TestDigestCollapsesDuplicateURLsWithinBatch(t *testing.T) {
	// The same URL appears twice in one inbox.md — even on a fresh (empty)
	// history it must be processed once and the duplicate collapsed to a skip,
	// so no double fetch/write. Both inbox lines still flush.
	url := "https://example.com/dup"
	deps := newFakeDeps()
	deps.inbox.entries = []wikiwrite.InboxEntry{{URL: url, LineIndex: 1}, {URL: url, LineIndex: 2}}
	deps.fetcher.results[url] = &wikitypes.ContentFetchResult{Title: "A", Body: "body"}
	deps.classifier.results[url] = &wikitypes.ClassifyResult{
		TopicPath:   "topic/path",
		WikiType:    wikitypes.TypeDeepDive,
		ContentType: wikitypes.ContentText,
		Summary:     &wikitypes.StructuredSummary{Overview: "summary"},
	}
	cfg := testConfig(t)
	// No success history — the duplicate here is intra-batch only.
	require.NoError(t, os.WriteFile(filepath.Join(cfg.Wiki.WikiRoot, "inbox.md"), []byte("- "+url+"\n- "+url+"\n"), 0o600))
	deps.history = loadDigestHistory(digestStoreAdapter{store: deps.r2})

	result, err := RunDigest(context.Background(), DigestInput{Config: cfg, deps: deps.dependencies()})
	require.NoError(t, err)
	require.Len(t, result.URLResults, 2)
	require.Equal(t, StatusSummaryWritten, result.URLResults[0].Status)
	require.Equal(t, StatusSkipped, result.URLResults[1].Status)
	require.Len(t, deps.writer.summaries, 1)
	require.Equal(t, 2, result.Flushed)
	require.Equal(t, []string{url}, deps.inbox.flushed[1])
	require.Equal(t, []string{url}, deps.inbox.flushed[2])
}

func TestLoadDigestHistoryFromR2SeedsDedup(t *testing.T) {
	store := newFakeMediaStore()
	// Seed the R2 success log with two successes and one failure.
	store.objects["logs/digest-success.jsonl"] = "" +
		`{"timestamp":"2026-01-01T00:00:00Z","url":"https://example.com/old","status":"success","stage":"write"}` + "\n" +
		`{"timestamp":"2026-01-01T00:00:00Z","url":"https://example.com/again","status":"success","stage":"write"}` + "\n" +
		`{"timestamp":"2026-01-01T00:00:00Z","url":"https://example.com/bad","status":"failure","stage":"fetch"}` + "\n"

	h := loadDigestHistory(digestStoreAdapter{store: store})

	require.True(t, h.claimOrSeen("https://example.com/old"))
	require.True(t, h.claimOrSeen("https://example.com/again"))
	// Failure entries are not remembered → retried.
	require.False(t, h.claimOrSeen("https://example.com/bad"))
}

func TestLoadDigestHistoryFromR2MissingObjectYieldsEmpty(t *testing.T) {
	h := loadDigestHistory(digestStoreAdapter{store: newFakeMediaStore()})

	require.NotNil(t, h)
	require.False(t, h.claimOrSeen("https://example.com/a"))
}
