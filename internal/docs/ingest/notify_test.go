package wikiingest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	wikitypes "github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
)

func TestBuildDigestNotifyRendersAbortAndCounts(t *testing.T) {
	cfg := &Config{Compact: CompactConfig{Title: "wiki digest"}}
	cfg.R2.PublicBaseURL = "https://dw.test"
	entry := &wikitypes.DigestRunEntry{
		BatchID:         "wiki-2026-11-07",
		StartedAt:       "2026-11-07T22:00:00+08:00",
		DurationMs:      1234,
		Processed:       8,
		Succeeded:       3,
		Retryable:       5,
		AbortReason:     "consecutive_ai_failures",
		Error:           "wiki digest aborted: consecutive AI failures",
		FailureByReason: map[string]int{"ai_unavailable": 5},
	}

	subject, body := buildDigestNotify(cfg, entry, true)

	assert.Contains(t, subject, "ABORTED (consecutive_ai_failures)")
	assert.Contains(t, subject, "wiki-2026-11-07")
	assert.Contains(t, body, "retryable</td><td>5")
	assert.Contains(t, body, "ai_unavailable")
	assert.Contains(t, body, "https://dw.test/logs/digest-runs.jsonl")
}

func TestBuildDigestNotifyWithoutRunLog(t *testing.T) {
	cfg := &Config{Compact: CompactConfig{Title: "wiki digest"}}

	subject, body := buildDigestNotify(cfg, nil, true)

	assert.Contains(t, subject, "FAILED")
	assert.Contains(t, subject, "no run log")
	assert.Contains(t, body, "No digest run summary")
}

func TestLastRunLogEntrySkipsMalformedLines(t *testing.T) {
	data := []byte(`{"batchId":"first","processed":1}` + "\n" +
		"not-json\n" +
		`{"batchId":"last","processed":2}` + "\n")

	entry, err := lastRunLogEntry(data)
	require.NoError(t, err)
	require.NotNil(t, entry)
	assert.Equal(t, "last", entry.BatchID)
	assert.Equal(t, 2, entry.Processed)
}

func TestLastRunLogEntryEmpty(t *testing.T) {
	entry, err := lastRunLogEntry(nil)
	require.NoError(t, err)
	assert.Nil(t, entry)
}
