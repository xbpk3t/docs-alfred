package rss

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassifyFeedFailureTimeoutIsTransient(t *testing.T) {
	result := classifyFeedFailure(&FeedError{
		URL: "https://rsshub.stefanzhang.com/xiaoyuzhou/podcast/5e5c52c9418a84a04625e6cc",
		Err: errors.New(`Get "https://rsshub.stefanzhang.com/xiaoyuzhou/podcast/5e5c52c9418a84a04625e6cc": context deadline exceeded (Client.Timeout exceeded while awaiting headers)`),
	})

	assert.Equal(t, FeedFailureKindTimeout, result.Kind)
	assert.True(t, result.Transient)
}

func TestBuildFeedFailureReportGroupsTransientFailures(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "feed-failures.json")
	report, err := BuildFeedFailureReport(
		[]*FeedError{
			timeoutFailure("https://rsshub.stefanzhang.com/xiaoyuzhou/podcast/a"),
			timeoutFailure("https://rsshub.stefanzhang.com/xiaoyuzhou/podcast/b"),
			timeoutFailure("https://rsshub.stefanzhang.com/xiaoyuzhou/podcast/c"),
		},
		FeedFailureReportConfig{
			ExampleLimit:               2,
			TransientEscalateAfterRuns: 3,
			StatePath:                  statePath,
		},
		time.Date(2026, 6, 6, 8, 0, 0, 0, time.UTC),
	)
	require.NoError(t, err)
	require.NotNil(t, report)
	require.Len(t, report.Groups, 1)

	group := report.Groups[0]
	assert.Equal(t, "rsshub.stefanzhang.com", group.Host)
	assert.Equal(t, FeedFailureKindTimeout, group.Kind)
	assert.Equal(t, 3, group.Count)
	assert.Len(t, group.ExampleURLs, 2)
	assert.Equal(t, 1, group.RemainingCount)
	assert.Equal(t, 1, group.MaxConsecutiveRuns)
	assert.True(t, group.Transient)
	assert.True(t, group.Suppressed)
	assert.False(t, group.Escalated)
	assert.Equal(t, 3, report.TotalFailures)
	assert.Equal(t, 3, report.SuppressedFailures)
}

func TestBuildFeedFailureReportEscalatesAfterConsecutiveRuns(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "feed-failures.json")
	cfg := FeedFailureReportConfig{
		TransientEscalateAfterRuns: 3,
		StatePath:                  statePath,
	}
	failures := []*FeedError{
		timeoutFailure("https://rsshub.stefanzhang.com/xiaoyuzhou/podcast/a"),
	}
	now := time.Date(2026, 6, 6, 8, 0, 0, 0, time.UTC)

	var report *FeedFailureReport
	var err error
	for run := range 3 {
		report, err = BuildFeedFailureReport(failures, cfg, now.Add(time.Duration(run)*24*time.Hour))
		require.NoError(t, err)
	}

	require.NotNil(t, report)
	require.Len(t, report.Groups, 1)
	group := report.Groups[0]
	assert.Equal(t, 3, group.MaxConsecutiveRuns)
	assert.True(t, group.Escalated)
	assert.False(t, group.Suppressed)
	assert.Equal(t, 1, report.EscalatedFailures)
}

func timeoutFailure(feedURL string) *FeedError {
	return &FeedError{
		URL: feedURL,
		Err: errors.New(`context deadline exceeded (Client.Timeout exceeded while awaiting headers)`),
	}
}
