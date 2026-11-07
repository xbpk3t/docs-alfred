package wikiingest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	wikitypes "github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
	wikiwrite "github.com/xbpk3t/docs-alfred/internal/docs/wiki/write"
)

func TestWriteLineResultsKeepsParentWhenChildRetryable(t *testing.T) {
	deps := newFakeDeps()
	parent := pendingURLWrite{
		URL:  questionURL,
		Kind: pendingSummary,
		Item: &wikitypes.ClassifyItem{
			URL:       questionURL,
			TopicPath: "AI/agent/agent-security",
			Type:      wikitypes.TypeDeepDive,
		},
	}
	child := pendingAIErrorWrite(answerOne, wikitypes.FailureReasonAI, "AI down")

	results := writeLineResults(deps.dependencies(), t.TempDir(), []pendingURLWrite{parent, child}, false)

	require.Len(t, results, 2)
	require.Len(t, deps.writer.summaries, 1)
	assert.Equal(t, questionURL, deps.writer.summaries[0].url)
	assert.True(t, deps.writer.summaries[0].skipHistory,
		"parent must not enter success history while a child is retryable")
	assert.False(t, results[1].Handled, "parent keeps the inbox line for a re-expand")
}

func TestWriteLineResultsLogsParentWhenChildrenTerminal(t *testing.T) {
	deps := newFakeDeps()
	parent := pendingURLWrite{
		URL:  questionURL,
		Kind: pendingSummary,
		Item: &wikitypes.ClassifyItem{
			URL:       questionURL,
			TopicPath: "AI/agent/agent-security",
			Type:      wikitypes.TypeDeepDive,
		},
	}
	child := pendingExtractFailureWrite(
		&wikitypes.ClassifyItem{URL: answerOne},
		wikitypes.FailureReasonNotFound,
		"404",
	)

	results := writeLineResults(deps.dependencies(), t.TempDir(), []pendingURLWrite{parent, child}, false)

	require.Len(t, results, 2)
	require.Len(t, deps.writer.summaries, 1)
	assert.False(t, deps.writer.summaries[0].skipHistory)
	assert.True(t, results[1].Handled, "a permanent child failure lets the line drain")
}

func TestRunDigestKeepsMultiItemLineForRetryableChild(t *testing.T) {
	deps := multiItemDeps()
	deps.classifier.errs[answerTwo] = errors.New("AI down")
	deps.inbox.entries = []wikiwrite.InboxEntry{{URL: questionURL, LineIndex: 1}}

	cfg := testConfig(t)
	cfg.Wiki.Concurrency = 1
	require.NoError(t, os.WriteFile(
		filepath.Join(cfg.Wiki.WikiRoot, "inbox.md"), []byte("- "+questionURL+"\n"), 0o600))

	result, err := RunDigest(context.Background(), DigestInput{Config: cfg, deps: deps.dependencies()})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.OK(), "a retryable child is not an unhandled error")

	require.Len(t, deps.writer.summaries, 2, "the parent synthesis and the successful answer")
	var parentCall *writeCall
	for i := range deps.writer.summaries {
		if deps.writer.summaries[i].url == questionURL {
			parentCall = &deps.writer.summaries[i]
		}
	}
	require.NotNil(t, parentCall)
	assert.True(t, parentCall.skipHistory)
	// The parent URL must stay in the inbox so the next run re-expands the
	// thread and retries the failed answer.
	assert.NotContains(t, deps.inbox.flushed[1], questionURL)
	require.Len(t, result.URLResults, 3)
	assert.Equal(t, questionURL, result.URLResults[2].URL)
	assert.False(t, result.URLResults[2].Handled)
}

func TestRunDigestAIBreakerStopsRemainingURLs(t *testing.T) {
	deps := newFakeDeps()
	cfg := testConfig(t)
	cfg.Wiki.Concurrency = 1

	const urls = maxConsecutiveAIFailures + 3
	entries := make([]wikiwrite.InboxEntry, 0, urls)
	for i := 0; i < urls; i++ {
		url := fmt.Sprintf("https://example.com/%d", i)
		entries = append(entries, wikiwrite.InboxEntry{URL: url, LineIndex: i})
		deps.classifier.errs[url] = errors.New("AI down")
		deps.fetcher.results[url] = &wikitypes.ContentFetchResult{Title: url, Body: "body"}
	}
	deps.inbox.entries = entries
	require.NoError(t, os.WriteFile(
		filepath.Join(cfg.Wiki.WikiRoot, "inbox.md"), []byte("inbox\n"), 0o600))

	result, err := RunDigest(context.Background(), DigestInput{Config: cfg, deps: deps.dependencies()})
	require.ErrorIs(t, err, errAIBreaker)
	require.NotNil(t, result)
	assert.Equal(t, "consecutive_ai_failures", result.AbortReason)
	assert.Len(t, deps.writer.failures, maxConsecutiveAIFailures,
		"remaining URLs must not be fetched, classified or logged")
	assert.Len(t, result.URLResults, maxConsecutiveAIFailures)
	assert.Zero(t, result.Flushed, "AI failures are retryable; no line is handled")
	assert.Equal(t, maxConsecutiveAIFailures, result.Summary()["retryableFailures"])

	// The run-level summary is written even on the abort path.
	data, readErr := os.ReadFile(filepath.Join(cfg.Wiki.WikiRoot, "digest-runs.jsonl"))
	require.NoError(t, readErr)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	require.Len(t, lines, 1)
	var run wikitypes.DigestRunEntry
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &run))
	assert.Equal(t, "consecutive_ai_failures", run.AbortReason)
	assert.Equal(t, maxConsecutiveAIFailures, run.Retryable)
	assert.Contains(t, run.Error, "aborted")
}

func TestRunDigestRequiresR2WhenConfigured(t *testing.T) {
	cfg := testConfig(t)
	cfg.R2.Bucket = "bucket"
	cfg.R2.PublicBaseURL = "https://dw.test"

	for _, name := range []string{"R2_AK", "CF_R2_AK", "R2_SK", "R2_KEY", "CF_R2_SK", "R2_ENDPOINT", "CF_ACCOUNT"} {
		t.Setenv(name, "")
	}
	require.NoError(t, os.WriteFile(
		filepath.Join(cfg.Wiki.WikiRoot, "inbox.md"), []byte("inbox\n"), 0o600))

	_, err := RunDigest(context.Background(), DigestInput{Config: cfg})
	require.ErrorContains(t, err, "R2 client is unavailable")
}
