package write

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
)

func TestDigestFilenameSuccessEntry(t *testing.T) {
	assert.Equal(t, "digest-success.jsonl", digestFilename(&types.DigestEntry{Status: types.DigestSuccess}))
}

func TestDigestFilenameFetchFailure(t *testing.T) {
	assert.Equal(t, "digest-fetch-error.jsonl",
		digestFilename(&types.DigestEntry{Status: types.DigestFailure, FailureKind: string(types.FailureFetch)}))
}

func TestDigestFilenameResolveFailure(t *testing.T) {
	assert.Equal(t, "digest-fetch-error.jsonl",
		digestFilename(&types.DigestEntry{Status: types.DigestFailure, FailureKind: string(types.FailureResolve)}))
}

func TestDigestFilenameExtractFailure(t *testing.T) {
	assert.Equal(t, "digest-extract-error.jsonl",
		digestFilename(&types.DigestEntry{Status: types.DigestFailure, FailureKind: string(types.FailureExtract)}))
}

func TestDigestFilenameClassifyFailure(t *testing.T) {
	assert.Equal(t, "digest-classify-rejected.jsonl",
		digestFilename(&types.DigestEntry{Status: types.DigestFailure, FailureKind: string(types.FailureClassify)}))
}

func TestDigestFilenameAIFailure(t *testing.T) {
	assert.Equal(t, "digest-ai-error.jsonl",
		digestFilename(&types.DigestEntry{Status: types.DigestFailure, FailureKind: string(types.FailureAI)}))
}

func TestDigestFilenameUnknownFailureDefaultsToAI(t *testing.T) {
	assert.Equal(t, "digest-ai-error.jsonl",
		digestFilename(&types.DigestEntry{Status: types.DigestFailure, FailureKind: "something-else"}))
}

func TestLogDigestEntryWithNilOptsReturnsEmpty(t *testing.T) {
	path, err := LogDigestEntry(&types.DigestEntry{URL: "https://example.com"}, nil)
	assert.NoError(t, err)
	assert.Empty(t, path)
}

func TestLogDigestEntryDryRunReturnsKey(t *testing.T) {
	store := newFakeDigestStore()
	entry := &types.DigestEntry{
		URL:    "https://example.com",
		Status: types.DigestSuccess,
	}

	path, err := LogDigestEntry(entry, &WriteOptions{R2Store: store, DryRun: true})

	require.NoError(t, err)
	assert.Equal(t, "logs/digest-success.jsonl", path)
	assert.Empty(t, store.puts, "dry run must not upload")
}

func TestLogDigestEntryDoesNotWriteWithoutR2(t *testing.T) {
	_, err := LogDigestEntry(&types.DigestEntry{URL: "https://example.com"}, &WriteOptions{})
	require.ErrorContains(t, err, "r2 store required")
}

func TestLogDigestEntryEmptyTimestampIsPopulated(t *testing.T) {
	entry := &types.DigestEntry{
		URL:    "https://example.com",
		Status: types.DigestSuccess,
		// Timestamp intentionally left empty.
	}

	_, err := LogDigestEntry(entry, &WriteOptions{R2Store: newFakeDigestStore()})
	require.NoError(t, err)

	// The entry should have been given a timestamp.
	assert.NotEmpty(t, entry.Timestamp, "Timestamp should be populated when empty")
}

func TestLogDigestEntryPreservesExistingTimestamp(t *testing.T) {
	entry := &types.DigestEntry{
		URL:       "https://example.com",
		Status:    types.DigestSuccess,
		Timestamp: "2026-01-01T00:00:00Z",
	}

	_, err := LogDigestEntry(entry, &WriteOptions{R2Store: newFakeDigestStore()})
	require.NoError(t, err)

	assert.Equal(t, "2026-01-01T00:00:00Z", entry.Timestamp)
}

func TestLogDigestEntryWithBatchIDSet(t *testing.T) {
	store := newFakeDigestStore()
	entry := &types.DigestEntry{
		URL:     "https://example.com",
		Status:  types.DigestSuccess,
		BatchID: "custom-batch",
	}

	_, err := LogDigestEntry(entry, &WriteOptions{R2Store: store, BatchID: "opts-batch"})
	require.NoError(t, err)

	// Entry's own BatchID should be preserved (not overwritten by opts.BatchID)
	assert.Equal(t, "custom-batch", entry.BatchID)
	assert.Contains(t, string(store.objects["logs/digest-success.jsonl"]), "custom-batch")
}

func TestLogDigestEntryUsesOptsBatchIDWhenEmpty(t *testing.T) {
	store := newFakeDigestStore()
	entry := &types.DigestEntry{
		URL:    "https://example.com",
		Status: types.DigestSuccess,
	}

	_, err := LogDigestEntry(entry, &WriteOptions{R2Store: store, BatchID: "opts-batch"})
	require.NoError(t, err)

	assert.Equal(t, "opts-batch", entry.BatchID)
	assert.Contains(t, string(store.objects["logs/digest-success.jsonl"]), "opts-batch")
}
