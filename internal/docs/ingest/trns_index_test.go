package wikiingest

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	wikitypes "github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
	wikiwrite "github.com/xbpk3t/docs-alfred/internal/docs/wiki/write"
	"github.com/xbpk3t/docs-alfred/pkg/urlutil"
)

const (
	episodeOne = "https://pod.example/ep1"
	pageOne    = "https://media.example/trns/t1.html"
)

func indexBatch(t *testing.T, entries ...trnsIndexEntry) string {
	t.Helper()
	data, err := json.Marshal(entries)
	require.NoError(t, err)

	return string(data)
}

func TestLoadTrnsIndexMergesBatchesAndSkipsGarbage(t *testing.T) {
	store := newFakeMediaStore()
	store.objects["index-2026-10-18.json"] = indexBatch(t,
		trnsIndexEntry{URL: episodeOne, TranscriptURL: pageOne, Title: "Ep 1"})
	store.objects["index-2026-10-19.json"] = indexBatch(t,
		trnsIndexEntry{URL: "https://pod.example/ep2", TranscriptURL: "https://media.example/trns/t2.html"})
	store.objects["index-broken.json"] = "{not json"
	store.objects["other.json"] = indexBatch(t, trnsIndexEntry{URL: "https://ignored.example/x"})

	entries, err := loadTrnsIndex(context.Background(), store)
	require.NoError(t, err)
	require.Len(t, entries, 2, "batches merge, non-index and unparsable objects are skipped")
	assert.ElementsMatch(t,
		[]string{episodeOne, "https://pod.example/ep2"},
		[]string{entries[0].URL, entries[1].URL},
	)
}

func TestLoadTrnsIndexListErrorPropagates(t *testing.T) {
	store := newFakeMediaStore()
	store.listErr = errors.New("r2 list: 503")

	_, err := loadTrnsIndex(context.Background(), store)
	require.Error(t, err)
}

func TestRunTrnsIndexEntriesWritesTranscriptLinkedEntries(t *testing.T) {
	store := newFakeMediaStore()
	store.objects["index-2026-10-19.json"] = indexBatch(t,
		trnsIndexEntry{URL: episodeOne, TranscriptURL: pageOne, Title: "Ep 1", FeedTitle: "Feed"})

	deps := newFakeDeps()
	deps.fetcher.results[pageOne] = &wikitypes.ContentFetchResult{Title: "Ep 1", Body: "transcript body"}
	deps.classifier.results[episodeOne] = classified("AI/LLM/x", wikitypes.TypeDeepDive)
	d := deps.dependencies()
	d.r2 = store

	results := runTrnsIndexEntries(context.Background(), d, t.TempDir(),
		inboxConfig{concurrency: 1, perURLTimeout: time.Minute}, false)

	require.Len(t, results, 1)
	assert.Equal(t, episodeOne, results[0].URL)
	assert.Equal(t, -1, results[0].LineIndex, "trns entries are not inbox lines")
	assert.Equal(t, StatusSummaryWritten, results[0].Status)

	require.Len(t, deps.writer.summaries, 1)
	assert.Equal(t, episodeOne, deps.writer.summaries[0].url, "identity stays the episode URL")
	assert.Equal(t, pageOne, deps.writer.summaries[0].transcriptURL, "the entry links its transcript page")
}

func TestRunTrnsIndexEntriesSkipsAlreadyDigested(t *testing.T) {
	store := newFakeMediaStore()
	store.objects["index-2026-10-19.json"] = indexBatch(t,
		trnsIndexEntry{URL: episodeOne, TranscriptURL: pageOne})

	deps := newFakeDeps()
	deps.history = newDigestHistory()
	deps.history.keys[urlutil.NormalizeForDedup(episodeOne)] = true
	d := deps.dependencies()
	d.r2 = store

	results := runTrnsIndexEntries(context.Background(), d, t.TempDir(),
		inboxConfig{concurrency: 1, perURLTimeout: time.Minute}, false)

	require.Len(t, results, 1)
	assert.Equal(t, StatusSkipped, results[0].Status)
	assert.Empty(t, deps.writer.summaries)
}

func TestRunTrnsIndexEntriesFetchFailureIsIsolated(t *testing.T) {
	store := newFakeMediaStore()
	store.objects["index-2026-10-19.json"] = indexBatch(t,
		trnsIndexEntry{URL: episodeOne, TranscriptURL: pageOne})

	deps := newFakeDeps()
	deps.fetcher.results[pageOne] = &wikitypes.ContentFetchResult{
		Error:       "opencli: HTTP 404",
		FailureKind: wikitypes.FailureFetch,
	}
	d := deps.dependencies()
	d.r2 = store

	results := runTrnsIndexEntries(context.Background(), d, t.TempDir(),
		inboxConfig{concurrency: 1, perURLTimeout: time.Minute}, false)

	require.Len(t, results, 1)
	assert.Equal(t, StatusFailureWritten, results[0].Status)
	assert.Empty(t, deps.writer.summaries)
	require.Len(t, deps.writer.failures, 1)
	assert.Equal(t, episodeOne, deps.writer.failures[0].url)
}

func TestRunTrnsIndexEntriesSkipsIncompleteEntries(t *testing.T) {
	store := newFakeMediaStore()
	store.objects["index-2026-10-19.json"] = indexBatch(t,
		trnsIndexEntry{URL: episodeOne},        // no transcript page
		trnsIndexEntry{TranscriptURL: pageOne}) // no episode URL

	deps := newFakeDeps()
	d := deps.dependencies()
	d.r2 = store

	results := runTrnsIndexEntries(context.Background(), d, t.TempDir(),
		inboxConfig{concurrency: 1, perURLTimeout: time.Minute}, false)

	require.Len(t, results, 2)
	for _, r := range results {
		assert.Equal(t, StatusSkipped, r.Status)
	}
	assert.Empty(t, deps.writer.summaries)
}

func TestRunTrnsIndexEntriesWithoutStoreIsNoop(t *testing.T) {
	results := runTrnsIndexEntries(context.Background(), &dependencies{}, t.TempDir(),
		inboxConfig{concurrency: 1, perURLTimeout: time.Minute}, false)

	assert.Empty(t, results)
}

// --- end to end through RunDigest ---

func TestRunDigestConsumesTrnsIndexWithoutInboxEntries(t *testing.T) {
	cfg := testConfig(t)
	cfg.TrnsIndex.Enabled = true
	require.NoError(t, os.WriteFile(filepath.Join(cfg.Wiki.WikiRoot, "inbox.md"), []byte("# inbox\n"), 0o600))

	store := newFakeMediaStore()
	store.objects["index-2026-10-19.json"] = indexBatch(t,
		trnsIndexEntry{URL: episodeOne, TranscriptURL: pageOne, Title: "Ep 1"})

	deps := newFakeDeps()
	deps.fetcher.results[pageOne] = &wikitypes.ContentFetchResult{Title: "Ep 1", Body: "transcript body"}
	deps.classifier.results[episodeOne] = classified("AI/LLM/x", wikitypes.TypeDeepDive)
	d := deps.dependencies()
	d.r2 = store

	result, err := RunDigest(context.Background(), DigestInput{Config: cfg, deps: d})
	require.NoError(t, err)

	require.Len(t, result.URLResults, 1)
	assert.Equal(t, episodeOne, result.URLResults[0].URL)
	assert.Equal(t, -1, result.URLResults[0].LineIndex)
	assert.Equal(t, 0, result.Flushed, "a trns entry is not an inbox line")
	assert.Len(t, deps.writer.summaries, 1)
	assert.Equal(t, pageOne, deps.writer.summaries[0].transcriptURL)
}

// A broken r2 setup must not block the inbox run: the trns loop is skipped,
// the inbox lines still flush.
func TestRunDigestTrnsIndexWithoutStoreStillFlushesInbox(t *testing.T) {
	cfg := testConfig(t)
	cfg.TrnsIndex.Enabled = true
	require.NoError(t, os.WriteFile(filepath.Join(cfg.Wiki.WikiRoot, "inbox.md"), []byte("inbox\n"), 0o600))

	deps := newFakeDeps()
	deps.inbox.entries = []wikiwrite.InboxEntry{{URL: "https://example.com/a", LineIndex: 0}}
	deps.fetcher.results["https://example.com/a"] = &wikitypes.ContentFetchResult{Title: "A", Body: "body"}
	deps.classifier.results["https://example.com/a"] = classified("AI/LLM/x", wikitypes.TypeDeepDive)

	result, err := RunDigest(context.Background(), DigestInput{Config: cfg, deps: deps.dependencies()})
	require.NoError(t, err)

	require.Len(t, result.URLResults, 1)
	assert.Equal(t, 1, result.Flushed)
	assert.Equal(t, map[int][]string{0: {"https://example.com/a"}}, deps.inbox.flushed)
}

// --- config ---

func TestLoadConfigR2AndTrnsIndexKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wiki.yml")
	require.NoError(t, os.WriteFile(path, []byte(`r2:
  bucket: media
  publicBaseUrl: https://media.example
  keyPrefix: wiki
trnsIndex:
  enabled: true
`), 0o600))

	cfg, err := LoadConfig(path, "")
	require.NoError(t, err)
	assert.Equal(t, "media", cfg.R2.Bucket)
	assert.Equal(t, "https://media.example", cfg.R2.PublicBaseURL)
	assert.Equal(t, "wiki", cfg.R2.KeyPrefix)
	assert.True(t, cfg.TrnsIndex.Enabled)
}

func TestR2ConfigLitterConfigOffWithoutBucket(t *testing.T) {
	got, err := R2Config{}.LitterConfig()
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestR2ConfigLitterConfigRequiresPublicBase(t *testing.T) {
	_, err := R2Config{Bucket: "media"}.LitterConfig()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "publicBaseUrl")
}

func TestR2ConfigLitterConfigReadsEnvCredentials(t *testing.T) {
	// Pin every name in the fallback chains so an ambient CF_R2_* / R2_SK in
	// the developer shell cannot leak into the assertions.
	t.Setenv("R2_AK", "ak")
	t.Setenv("R2_SK", "")
	t.Setenv("R2_KEY", "sk")
	t.Setenv("R2_ENDPOINT", "https://acct.r2.cloudflarestorage.com")

	got, err := R2Config{Bucket: "media", PublicBaseURL: "https://media.example"}.LitterConfig()

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "ak", got.AccessKeyID)
	assert.Equal(t, "sk", got.SecretAccessKey)
	assert.Equal(t, "https://acct.r2.cloudflarestorage.com", got.Endpoint)
}

func TestHandledURLsByLineIgnoresTrnsEntries(t *testing.T) {
	results := []URLResult{
		{URL: episodeOne, LineIndex: -1, Handled: true},
		{URL: "https://example.com/a", LineIndex: 0, Handled: true},
	}

	processed := handledURLsByLine(results)

	assert.Equal(t, map[int][]string{0: {"https://example.com/a"}}, processed)
}
