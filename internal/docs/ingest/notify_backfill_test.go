package wikiingest

import (
	"encoding/json"
	"strings"
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

func TestMergeSuccessHistoryAddsLocalOnlyEntries(t *testing.T) {
	local := strings.Join([]string{
		`{"url":"https://example.com/a","status":"success","stage":"write"}`,
		`{"url":"https://example.com/b","status":"success","stage":"write"}`,
		`{"url":"https://example.com/a?utm_source=x","status":"success","stage":"write"}`,
		`{"url":"https://example.com/failed","status":"failure","stage":"fetch"}`,
		`not-json`,
	}, "\n")
	remote := strings.Join([]string{
		`{"url":"https://example.com/b","status":"success","stage":"write"}`,
		`{"url":"https://example.com/d","status":"success","stage":"write"}`,
	}, "\n")

	merged, stats, err := mergeSuccessHistory([]byte(local), []byte(remote))
	require.NoError(t, err)
	assert.Equal(t, 2, stats.localUnique, "a and b after tracking-param dedup")
	assert.Equal(t, 2, stats.remoteUnique, "b and d")
	assert.Equal(t, 1, stats.added, "only a is missing from the remote log")

	urls := map[string]int{}
	for _, line := range strings.Split(strings.TrimSpace(string(merged)), "\n") {
		var entry wikitypes.DigestEntry
		require.NoError(t, json.Unmarshal([]byte(line), &entry))
		urls[entry.URL]++
	}
	assert.Equal(t, 1, urls["https://example.com/a"])
	assert.Equal(t, 1, urls["https://example.com/b"])
	assert.Equal(t, 1, urls["https://example.com/d"])
	assert.NotContains(t, urls, "https://example.com/failed")

	// Idempotent: the merged log is now the remote input.
	_, again, err := mergeSuccessHistory([]byte(local), merged)
	require.NoError(t, err)
	assert.Zero(t, again.added)
}
