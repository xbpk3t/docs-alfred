package wiki

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/service/ghindex"
)

func TestRenderPrompt(t *testing.T) {
	prompt, err := renderPrompt("classify-json.txt", &promptData{
		Title:         "A title",
		URL:           "https://example.com/post",
		Content:       "A summary",
		CandidateTree: "- path: ai/tool/demo | title: Demo | source: test",
	})
	if err != nil {
		t.Fatalf("renderPrompt() error = %v", err)
	}

	for _, want := range []string{"A title", "https://example.com/post", "A summary"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("rendered prompt missing %q:\n%s", want, prompt)
		}
	}
	if strings.Contains(prompt, "{{") {
		t.Fatalf("rendered prompt still contains template marker:\n%s", prompt)
	}
}

func TestParseAIClassificationAcceptsJSONObject(t *testing.T) {
	parsed, err := parseAIClassification(`{"topicPath":"ai/tool/demo","wikiType":"research","contentType":"text","summary":{"overview":"ok","keyPoints":["p1"],"worthNoting":"n"},"confidence":0.9}`)

	require.NoError(t, err)
	assert.Equal(t, "ai/tool/demo", parsed.TopicPath)
	assert.Equal(t, TypeDeepDive, parsed.WikiType)
}

func TestParseAIClassificationRepairsInvalidStringEscapes(t *testing.T) {
	parsed, err := parseAIClassification(`{"topicPath":"ai/tool/demo","wikiType":"research","contentType":"text","summary":{"overview":"1. ok \3. bad","keyPoints":["p1"],"worthNoting":"n"},"confidence":0.9}`)

	require.NoError(t, err)
	assert.Equal(t, `1. ok \3. bad`, parsed.Summary.Overview)
}

func TestParseAIClassificationRejectsInvalidJSON(t *testing.T) {
	_, err := parseAIClassification("not json")

	require.Error(t, err)
}

func TestValidateAIClassificationFallsBackToUncategorized(t *testing.T) {
	classifier := NewClassifier(nil, t.TempDir(), "", WithCandidateLimit(10))
	result, err := classifier.validateAIClassification(&aiClassification{
		TopicPath:   "ai/tool/missing",
		WikiType:    TypeDeepDive,
		ContentType: ContentText,
		Summary:     &StructuredSummary{Overview: "test overview", KeyPoints: []string{"point"}, WorthNoting: "test note"},
		Confidence:  0.9,
	}, []ghindex.TopicCandidate{{Path: "ai/tool/demo"}}, ContentText)

	require.NoError(t, err)
	assert.Equal(t, "zzz/ss/uncategorized", result.TopicPath)
}

func TestValidateAIClassificationRejectsLowConfidence(t *testing.T) {
	classifier := NewClassifier(nil, t.TempDir(), "")
	_, err := classifier.validateAIClassification(&aiClassification{
		TopicPath:   "ai/tool/demo",
		WikiType:    TypeDeepDive,
		ContentType: ContentText,
		Summary:     &StructuredSummary{Overview: "test overview", KeyPoints: []string{"point"}, WorthNoting: "test note"},
		Confidence:  0.1,
	}, []ghindex.TopicCandidate{{Path: "ai/tool/demo"}}, ContentText)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "confidence")
}

func TestRejectedClassifyResultPreservesDiagnostics(t *testing.T) {
	result := rejectedClassifyResult(&aiClassification{
		TopicPath:         "ai/tool/demo",
		WikiType:          TypeInbox,
		ContentType:       ContentText,
		Summary:           &StructuredSummary{Overview: "manual summary", WorthNoting: ""},
		Confidence:        0.42,
		NeedsManualReview: true,
	}, ContentText, assert.AnError)

	require.NotNil(t, result)
	assert.Equal(t, "ai/tool/demo", result.TopicPath)
	assert.Equal(t, TypeInbox, result.WikiType)
	require.NotNil(t, result.Summary)
	assert.Equal(t, "manual summary", result.Summary.Overview)
	assert.Equal(t, 0.42, result.Confidence)
	assert.True(t, result.NeedsManualReview)
	assert.Contains(t, result.RejectReason, assert.AnError.Error())
}

func TestValidateAIClassificationAcceptsCandidate(t *testing.T) {
	classifier := NewClassifier(nil, t.TempDir(), "")
	result, err := classifier.validateAIClassification(&aiClassification{
		TopicPath:   "ai/tool/demo",
		WikiType:    TypeDeepDive,
		ContentType: ContentText,
		Summary:     &StructuredSummary{Overview: "test overview", KeyPoints: []string{"point"}, WorthNoting: "test note"},
		Confidence:  0.9,
	}, []ghindex.TopicCandidate{{Path: "ai/tool/demo"}}, ContentText)

	require.NoError(t, err)
	assert.Equal(t, "ai/tool/demo", result.TopicPath)
	assert.Equal(t, ContentText, result.ContentType)
}

func TestClassificationCandidatesRetriesRemoteCatalogAfterFailure(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, makeTopicDir(root, "local/tool/demo"))
	classifier := NewClassifier(nil, root, "")
	calls := 0
	classifier.loadGHTopics = func() ([]ghindex.TopicCandidate, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("temporary remote failure")
		}

		return []ghindex.TopicCandidate{{Path: "remote/tool/demo", Source: "gh:config"}}, nil
	}

	first, err := classifier.classificationCandidates(context.Background(), "https://example.com", "title", "content")
	require.Error(t, err)
	assertCandidatePath(t, first, "local/tool/demo")
	assertNoCandidatePath(t, first, "remote/tool/demo")

	second, err := classifier.classificationCandidates(context.Background(), "https://example.com", "title", "content")
	require.NoError(t, err)
	assertCandidatePath(t, second, "local/tool/demo")
	assertCandidatePath(t, second, "remote/tool/demo")
	require.Equal(t, 2, calls)
}

func TestClassificationCandidatesKeepLocalFallbackWhenRemoteUnavailable(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, makeTopicDir(root, "local/tool/demo"))
	classifier := NewClassifier(nil, root, "")
	classifier.loadGHTopics = func() ([]ghindex.TopicCandidate, error) {
		return nil, errors.New("remote down")
	}

	candidates, err := classifier.classificationCandidates(context.Background(), "https://example.com", "title", "content")

	require.Error(t, err)
	assertCandidatePath(t, candidates, "local/tool/demo")
}

func TestTruncateKeepsUTF8Valid(t *testing.T) {
	result := truncate(strings.Repeat("你好", 20), 5)

	assert.True(t, utf8.ValidString(result))
	assert.Equal(t, "你...", result)
}

func makeTopicDir(root, topicPath string) error {
	return os.MkdirAll(filepath.Join(root, filepath.FromSlash(topicPath)), 0o700)
}

func assertCandidatePath(t *testing.T, candidates []ghindex.TopicCandidate, want string) {
	t.Helper()
	for _, candidate := range candidates {
		if candidate.Path == want {
			return
		}
	}
	assert.Failf(t, "missing candidate", "want %s in %#v", want, candidates)
}

func assertNoCandidatePath(t *testing.T, candidates []ghindex.TopicCandidate, want string) {
	t.Helper()
	for _, candidate := range candidates {
		if candidate.Path == want {
			assert.Failf(t, "unexpected candidate", "got %s in %#v", want, candidates)
		}
	}
}
