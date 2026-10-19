package wikiingest

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	wikiclassify "github.com/xbpk3t/docs-alfred/internal/docs/wiki/classify"
	wikitypes "github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
	wikiwrite "github.com/xbpk3t/docs-alfred/internal/docs/wiki/write"
)

const (
	questionURL = "https://www.zhihu.com/question/35129528"
	answerOne   = "https://www.zhihu.com/question/35129528/answer/111"
	answerTwo   = "https://www.zhihu.com/question/35129528/answer/222"
)

func classified(topic string, wikiType wikitypes.ClassifyType) *wikitypes.ClassifyResult {
	return &wikitypes.ClassifyResult{
		TopicPath:   topic,
		WikiType:    wikiType,
		ContentType: wikitypes.ContentText,
		Summary:     &wikitypes.StructuredSummary{Overview: "overview", KeyPoints: []string{"point"}},
	}
}

// multiItemDeps wires a question page that advertises two answer permalinks.
func multiItemDeps() *fakeDeps {
	deps := newFakeDeps()
	deps.fetcher.results[questionURL] = &wikitypes.ContentFetchResult{
		Title:     "问题",
		Body:      "问题综述正文",
		ChildURLs: []string{answerOne, answerTwo},
	}
	deps.classifier.results[questionURL] = classified("AI/agent/agent-security", wikitypes.TypeDeepDive)

	for _, u := range []string{answerOne, answerTwo} {
		deps.fetcher.results[u] = &wikitypes.ContentFetchResult{Title: "问题 - 作者", Body: "回答全文"}
		deps.classifier.results[u] = classified("AI/agent/agent-security", wikitypes.TypeDeepDive)
	}

	return deps
}

func TestExpandChildPostsWritesSynthesisAndChildren(t *testing.T) {
	deps := multiItemDeps()

	pending := expandChildPosts(context.Background(), deps.dependencies(), questionURL, deps.fetcher.results[questionURL])

	require.Len(t, pending, 3, "one synthesis entry plus one entry per answer")
	assert.Equal(t, questionURL, pending[0].URL)
	assert.Equal(t, pendingSummary, pending[0].Kind)
	assert.Equal(t, answerOne, pending[1].URL)
	assert.Equal(t, answerTwo, pending[2].URL)
	assert.Equal(t, pendingSummary, pending[1].Kind)
	assert.Equal(t, pendingSummary, pending[2].Kind)
}

// The whole point of splitting: each answer is its own entry, addressed by its
// own permalink, so URL-level dedup and failure logs work per answer.
func TestExpandChildPostsChildrenCarryOwnURL(t *testing.T) {
	deps := multiItemDeps()

	pending := expandChildPosts(context.Background(), deps.dependencies(), questionURL, deps.fetcher.results[questionURL])

	for _, p := range pending[1:] {
		require.NotNil(t, p.Item)
		assert.Equal(t, p.URL, p.Item.URL)
		assert.NotEqual(t, questionURL, p.Item.URL)
	}
}

func TestExpandChildPostsChildrenInheritSynthesisTopic(t *testing.T) {
	deps := multiItemDeps()

	expandChildPosts(context.Background(), deps.dependencies(), questionURL, deps.fetcher.results[questionURL])

	want := wikiclassify.TopicPin{
		TopicPath: "AI/agent/agent-security",
		WikiType:  wikitypes.TypeDeepDive,
	}
	assert.Equal(t, want, deps.classifier.pins[answerOne])
	assert.Equal(t, want, deps.classifier.pins[answerTwo])
	// The synthesis itself chooses its own topic.
	assert.Equal(t, wikiclassify.TopicPin{}, deps.classifier.pins[questionURL])
}

// A synthesis that lands in uncat has no topic to inherit, so the children
// choose their own rather than being pinned to an empty path.
func TestExpandChildPostsUncategorisedSynthesisLeavesChildrenUnpinned(t *testing.T) {
	deps := multiItemDeps()
	deps.classifier.results[questionURL] = &wikitypes.ClassifyResult{
		TopicPath:         "",
		WikiType:          wikitypes.TypeInbox,
		ContentType:       wikitypes.ContentText,
		Summary:           &wikitypes.StructuredSummary{Overview: "overview", KeyPoints: []string{"point"}},
		NeedsManualReview: true,
	}

	pending := expandChildPosts(context.Background(), deps.dependencies(), questionURL, deps.fetcher.results[questionURL])

	require.Len(t, pending, 3)
	assert.True(t, pending[0].Item.NeedsManualReview)
	assert.Equal(t, wikiclassify.TopicPin{}, deps.classifier.pins[answerOne])
}

func TestExpandChildPostsSkipsAlreadyDigestedChild(t *testing.T) {
	deps := multiItemDeps()
	deps.history = newDigestHistory()
	deps.history.keys[answerOne] = true

	pending := expandChildPosts(context.Background(), deps.dependencies(), questionURL, deps.fetcher.results[questionURL])

	require.Len(t, pending, 2, "the already-digested answer produces no entry")
	assert.Equal(t, questionURL, pending[0].URL)
	assert.Equal(t, answerTwo, pending[1].URL)
}

func TestExpandChildPostsClaimsChildURLs(t *testing.T) {
	deps := multiItemDeps()
	deps.history = newDigestHistory()

	expandChildPosts(context.Background(), deps.dependencies(), questionURL, deps.fetcher.results[questionURL])

	assert.True(t, deps.history.keys[answerOne], "child URLs are claimed so they are never re-digested")
	assert.True(t, deps.history.keys[answerTwo])
}

// One unfetchable answer must not sink the thread: it becomes its own failure
// entry while the rest still land.
func TestExpandChildPostsChildFetchFailureIsIsolated(t *testing.T) {
	deps := multiItemDeps()
	deps.fetcher.results[answerOne] = &wikitypes.ContentFetchResult{
		Error:       "opencli: HTTP 403",
		FailureKind: wikitypes.FailureResolve,
	}

	pending := expandChildPosts(context.Background(), deps.dependencies(), questionURL, deps.fetcher.results[questionURL])

	require.Len(t, pending, 3)
	assert.Equal(t, pendingFetchFailure, pending[1].Kind)
	assert.Equal(t, answerOne, pending[1].URL)
	assert.Equal(t, wikitypes.FailureResolve, pending[1].FailureType)
	assert.Equal(t, pendingSummary, pending[2].Kind)
}

func TestTopicPinFromPending(t *testing.T) {
	tests := []struct {
		name string
		in   pendingURLWrite
		want wikiclassify.TopicPin
	}{
		{
			name: "summary with topic",
			in: pendingURLWrite{
				Kind: pendingSummary,
				Item: &wikitypes.ClassifyItem{TopicPath: "AI/LLM/x", Type: wikitypes.TypeDeepDive},
			},
			want: wikiclassify.TopicPin{TopicPath: "AI/LLM/x", WikiType: wikitypes.TypeDeepDive},
		},
		{
			name: "manual review carries no topic",
			in: pendingURLWrite{
				Kind: pendingSummary,
				Item: &wikitypes.ClassifyItem{
					TopicPath:         "AI/LLM/x",
					Type:              wikitypes.TypeDeepDive,
					NeedsManualReview: true,
				},
			},
			want: wikiclassify.TopicPin{},
		},
		{
			name: "empty topic",
			in: pendingURLWrite{
				Kind: pendingSummary,
				Item: &wikitypes.ClassifyItem{Type: wikitypes.TypeDeepDive},
			},
			want: wikiclassify.TopicPin{},
		},
		{
			name: "non-summary pending",
			in:   pendingURLWrite{Kind: pendingAIError, Item: &wikitypes.ClassifyItem{TopicPath: "a/b/c", Type: wikitypes.TypeDeepDive}},
			want: wikiclassify.TopicPin{},
		},
		{
			name: "nil item",
			in:   pendingURLWrite{Kind: pendingSummary},
			want: wikiclassify.TopicPin{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, topicPinFromPending(&tt.in))
		})
	}
}

// --- end to end through the inbox path ---

func TestRunDigestExpandsMultiItemPage(t *testing.T) {
	cfg := testConfig(t)
	require.NoError(t, os.WriteFile(filepath.Join(cfg.Wiki.WikiRoot, "inbox.md"), []byte("inbox\n"), 0o600))

	deps := multiItemDeps()
	deps.history = newDigestHistory()
	deps.inbox.entries = []wikiwrite.InboxEntry{{URL: questionURL, LineIndex: 0}}

	result, err := RunDigest(context.Background(), DigestInput{Config: cfg, deps: deps.dependencies()})
	require.NoError(t, err)

	require.Len(t, result.URLResults, 3)
	assert.ElementsMatch(t,
		[]string{questionURL, answerOne, answerTwo},
		[]string{result.URLResults[0].URL, result.URLResults[1].URL, result.URLResults[2].URL},
	)
	// Every result stays on the inbox line it came from, so the line flushes once.
	for _, r := range result.URLResults {
		assert.Equal(t, 0, r.LineIndex)
	}
	assert.Len(t, deps.writer.summaries, 3)
}

func TestProcessAddURLExpandsMultiItemPage(t *testing.T) {
	deps := multiItemDeps()

	results := processAddURL(context.Background(), deps.dependencies(), t.TempDir(), questionURL, false)

	require.Len(t, results, 3)
	assert.Equal(t, questionURL, results[0].URL)
	assert.Equal(t, answerOne, results[1].URL)
	assert.Equal(t, answerTwo, results[2].URL)
	for _, r := range results {
		assert.Equal(t, StatusSummaryWritten, r.Status)
	}
}

// --- inboxConfig plumbing stays unchanged for single-item pages ---

func TestPrepareInboxEntrySingleItemStaysOneEntry(t *testing.T) {
	deps := newFakeDeps()
	deps.fetcher.results["https://example.com/a"] = &wikitypes.ContentFetchResult{Title: "A", Body: "body"}
	deps.classifier.results["https://example.com/a"] = classified("AI/LLM/x", wikitypes.TypeDeepDive)

	pending := prepareInboxEntry(
		context.Background(),
		deps.dependencies(),
		wikiwrite.InboxEntry{URL: "https://example.com/a"},
		inboxConfig{concurrency: 1, perURLTimeout: time.Minute},
	)

	require.Len(t, pending, 1)
	assert.Equal(t, pendingSummary, pending[0].Kind)
}
