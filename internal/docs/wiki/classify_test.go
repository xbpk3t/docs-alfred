package wiki

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/internal/gh/index"
)

func TestRenderPrompt(t *testing.T) {
	prompt, err := renderPrompt("classify-json.txt", &promptData{
		Title:         "A title",
		URL:           "https://example.com/post",
		Content:       "A summary",
		CandidateTree: "- path: ai/tool/demo | title: Demo | source: test",
	})
	require.NoError(t, err, "renderPrompt() error")

	for _, want := range []string{"A title", "https://example.com/post", "A summary"} {
		assert.Contains(t, prompt, want, "rendered prompt should contain %q", want)
	}
	assert.NotContains(t, prompt, "{{", "rendered prompt should not contain template marker")
}

func TestParseAIClassificationAcceptsJSONObject(t *testing.T) {
	parsed, err := parseAIClassification(`{"topicPath":"ai/tool/demo","wikiType":"research","contentType":"text","summary":{"overview":"ok","keyPoints":["p1"],"worthNoting":"n"},"confidence":0.9}`)

	require.NoError(t, err)
	assert.Equal(t, "ai/tool/demo", parsed.TopicPath)
	assert.Equal(t, TypeDeepDive, parsed.WikiType)
}

func TestParseAIClassificationRejectsInvalidStringEscapes(t *testing.T) {
	_, err := parseAIClassification(`{"topicPath":"ai/tool/demo","wikiType":"research","contentType":"text","summary":{"overview":"1. ok \3. bad","keyPoints":["p1"],"worthNoting":"n"},"confidence":0.9}`)

	require.Error(t, err)
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
	assert.Empty(t, result.TopicPath)
	assert.True(t, result.NeedsManualReview)
	assert.Equal(t, RouteReasonInvalidTopicPath, result.RouteReason)
	assert.Equal(t, "ai/tool/missing", result.SuggestedTopic)
}

func TestValidateAIClassificationRejectsLowConfidence(t *testing.T) {
	classifier := NewClassifier(nil, t.TempDir(), "")
	result, err := classifier.validateAIClassification(&aiClassification{
		TopicPath:   "ai/tool/demo",
		WikiType:    TypeDeepDive,
		ContentType: ContentText,
		Summary:     &StructuredSummary{Overview: "test overview", KeyPoints: []string{"point"}, WorthNoting: "test note"},
		Confidence:  0.1,
	}, []ghindex.TopicCandidate{{Path: "ai/tool/demo"}}, ContentText)

	// Low conf + good summary → uncat triage (not hard reject).
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.NeedsManualReview)
	assert.Equal(t, RouteReasonNeedsManualReview, result.RouteReason)
	assert.Equal(t, "ai/tool/demo", result.SuggestedTopic)
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
	classifier := NewClassifier(nil, t.TempDir(), "")
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
	assert.Nil(t, first)

	second, err := classifier.classificationCandidates(context.Background(), "https://example.com", "title", "content")
	require.NoError(t, err)
	assertCandidatePath(t, second, "remote/tool/demo")
	require.Equal(t, 2, calls)
}

func TestClassificationCandidatesReturnsErrorWhenRemoteUnavailable(t *testing.T) {
	classifier := NewClassifier(nil, t.TempDir(), "")
	classifier.loadGHTopics = func() ([]ghindex.TopicCandidate, error) {
		return nil, errors.New("remote down")
	}

	candidates, err := classifier.classificationCandidates(context.Background(), "https://example.com", "title", "content")

	require.Error(t, err)
	assert.Nil(t, candidates)
}

func TestTruncateKeepsUTF8Valid(t *testing.T) {
	result := truncate(strings.Repeat("你好", 20), 5)

	assert.True(t, utf8.ValidString(result))
	assert.Equal(t, "你...", result)
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


// --- validateAIClassificationBasics ---

func TestValidateAIClassificationBasicsNilResult(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	err := c.validateAIClassificationBasics(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

func TestValidateAIClassificationBasicsNeedsManualReview(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	err := c.validateAIClassificationBasics(&aiClassification{
		NeedsManualReview: true,
		WikiType:          TypeDeepDive,
		Confidence:        0.9,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "manual review")
}

func TestValidateAIClassificationBasicsLowConfidence(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	err := c.validateAIClassificationBasics(&aiClassification{
		WikiType:   TypeDeepDive,
		Confidence: 0.01,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "confidence")
}

func TestValidateAIClassificationBasicsInvalidType(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	err := c.validateAIClassificationBasics(&aiClassification{
		WikiType:   "invalid",
		Confidence: 0.9,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid wiki type")
}

func TestValidateAIClassificationBasicsInvalidContentType(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	err := c.validateAIClassificationBasics(&aiClassification{
		WikiType:    TypeDeepDive,
		ContentType: "invalid",
		Confidence:  0.9,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid content type")
}

func TestValidateAIClassificationBasicsValid(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	err := c.validateAIClassificationBasics(&aiClassification{
		WikiType:    TypeDeepDive,
		ContentType: ContentText,
		Confidence:  0.9,
	})
	assert.NoError(t, err)
}

func TestValidateAIClassificationBasicsEmptyContentType(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	err := c.validateAIClassificationBasics(&aiClassification{
		WikiType:   TypeDeepDive,
		Confidence: 0.9,
	})
	assert.NoError(t, err)
}

// --- validateAIClassificationSummary ---

func TestValidateAIClassificationSummaryNil(t *testing.T) {
	_, err := validateAIClassificationSummary(&aiClassification{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty summary")
}

func TestValidateAIClassificationSummaryEmptyOverview(t *testing.T) {
	_, err := validateAIClassificationSummary(&aiClassification{
		Summary: &StructuredSummary{Overview: "  "},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty summary")
}

func TestValidateAIClassificationSummaryValid(t *testing.T) {
	summary, err := validateAIClassificationSummary(&aiClassification{
		Summary: &StructuredSummary{Overview: "good overview", KeyPoints: []string{"point"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "good overview", summary.Overview)
}

// --- validateAIClassificationTopic ---

func TestValidateAIClassificationTopicEmptyPath(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	topicPath, err := c.validateAIClassificationTopic(&aiClassification{
		TopicPath: "",
	}, nil)
	require.NoError(t, err)
	assert.Empty(t, topicPath)
}

func TestValidateAIClassificationTopicNone(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	topicPath, err := c.validateAIClassificationTopic(&aiClassification{
		TopicPath: "none",
	}, nil)
	require.NoError(t, err)
	assert.Empty(t, topicPath)
}

func TestValidateAIClassificationTopicInbox(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	topicPath, err := c.validateAIClassificationTopic(&aiClassification{
		TopicPath: "inbox",
	}, nil)
	require.NoError(t, err)
	assert.Empty(t, topicPath)
}

func TestValidateAIClassificationTopicValidCandidate(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	candidates := []ghindex.TopicCandidate{{Path: "ai/tool/demo"}}
	topicPath, err := c.validateAIClassificationTopic(&aiClassification{
		TopicPath: "ai/tool/demo",
	}, candidates)
	require.NoError(t, err)
	assert.Equal(t, "ai/tool/demo", topicPath)
}

func TestValidateAIClassificationTopicNotInCandidates(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	candidates := []ghindex.TopicCandidate{{Path: "ai/tool/demo"}}
	topicPath, err := c.validateAIClassificationTopic(&aiClassification{
		TopicPath: "ai/tool/other",
	}, candidates)
	require.NoError(t, err)
	assert.Empty(t, topicPath)
}

func TestValidateAIClassificationTopicInvalidPath(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	topicPath, err := c.validateAIClassificationTopic(&aiClassification{
		TopicPath: "../escape",
	}, nil)
	require.NoError(t, err)
	assert.Empty(t, topicPath)
}

// --- fallbackUncategorized ---

func TestFallbackUncategorizedWithCandidates(t *testing.T) {
	candidates := []ghindex.TopicCandidate{{Path: "zzz/ss/uncategorized"}}
	assert.Empty(t, fallbackUncategorized("/wiki", candidates))
}

func TestFallbackUncategorizedWithoutCandidates(t *testing.T) {
	assert.Empty(t, fallbackUncategorized("/wiki", nil))
}

func TestFallbackUncategorizedEmptyCandidates(t *testing.T) {
	assert.Empty(t, fallbackUncategorized("/wiki", []ghindex.TopicCandidate{}))
}

// --- rejectedClassifyResult ---

func TestRejectedClassifyResultNilResult(t *testing.T) {
	assert.Nil(t, rejectedClassifyResult(nil, "", nil))
}

func TestRejectedClassifyResultNilError(t *testing.T) {
	result := rejectedClassifyResult(&aiClassification{
		TopicPath:   "ai/tool/demo",
		WikiType:    TypeDeepDive,
		ContentType: ContentText,
		Confidence:  0.5,
	}, ContentText, nil)
	require.NotNil(t, result)
	assert.Equal(t, "classification rejected", result.RejectReason)
}

func TestRejectedClassifyResultEmptyContentType(t *testing.T) {
	result := rejectedClassifyResult(&aiClassification{
		TopicPath:   "ai/tool/demo",
		WikiType:    TypeDeepDive,
		ContentType: ContentVideo,
		Confidence:  0.5,
	}, "", nil)
	require.NotNil(t, result)
	assert.Equal(t, ContentVideo, result.ContentType)
}

// --- jsonKey ---

func TestJsonKeyDashTag(t *testing.T) {
	type S struct {
		Field string `json:"-"`
	}
	field := reflect.TypeFor[S]().Field(0)
	assert.Empty(t, jsonKey(&field))
}

func TestJsonKeyEmptyTag(t *testing.T) {
	type S struct {
		Field string
	}
	field := reflect.TypeFor[S]().Field(0)
	assert.Empty(t, jsonKey(&field))
}

func TestJsonKeyWithOptions(t *testing.T) {
	type S struct {
		Field string `json:"name,omitempty"`
	}
	field := reflect.TypeFor[S]().Field(0)
	assert.Equal(t, "name", jsonKey(&field))
}

// --- RenderStructuredSummary with detail and actionableAdvice ---

func TestRenderStructuredSummaryWithDetail(t *testing.T) {
	s := &StructuredSummary{
		Overview:  "overview",
		KeyPoints: []string{"point"},
		Detail:    "detailed analysis here",
	}
	rendered := RenderStructuredSummary(s)
	assert.Contains(t, rendered, "detail")
	assert.Contains(t, rendered, "detailed analysis here")
}

func TestRenderStructuredSummaryWithActionableAdvice(t *testing.T) {
	s := &StructuredSummary{
		Overview:         "overview",
		KeyPoints:        []string{"point"},
		ActionableAdvice: []string{"advice 1", "advice 2"},
	}
	rendered := RenderStructuredSummary(s)
	assert.Contains(t, rendered, "actionableAdvice")
	assert.Contains(t, rendered, "- advice 1")
	assert.Contains(t, rendered, "- advice 2")
}

// --- ClassifyURL ---

func TestClassifyURLEmptyContent(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	result := c.ClassifyURL(context.Background(), "https://example.com", "Title", "")
	assert.Nil(t, result)
}

func TestClassifyURLVideoContentTooShort(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	// Video content under 600 runes should be rejected
	result := c.ClassifyURL(context.Background(), "https://www.youtube.com/watch?v=abc", "Title", "short")
	assert.Nil(t, result)
}

func TestClassifyURLNoCandidates(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	c.loadGHTopics = func() ([]ghindex.TopicCandidate, error) {
		return nil, errors.New("no remote")
	}
	result := c.ClassifyURL(context.Background(), "https://example.com", "Title", "some content that is long enough")
	assert.Nil(t, result)
}

// --- validateClassifyResult with valid result ---

func TestValidateClassifyResultValidSummary(t *testing.T) {
	err := validateClassifyResult(&classifyOnlyResult{
		TopicPath: "ai/tool",
		WikiType:  TypeDeepDive,
		Summary: &StructuredSummary{
			Overview:  "overview",
			KeyPoints: []string{"point"},
		},
	})
	assert.NoError(t, err)
}

func TestValidateClassifyResultInvalidSummary(t *testing.T) {
	err := validateClassifyResult(&classifyOnlyResult{
		TopicPath: "ai/tool",
		WikiType:  TypeDeepDive,
		Summary: &StructuredSummary{
			Overview:  "",
			KeyPoints: []string{},
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "summary")
}

func TestValidateClassifyResultWithMetadata(t *testing.T) {
	err := validateClassifyResult(&classifyOnlyResult{
		TopicPath: "ai/tool",
		WikiType:  TypeDeepDive,
		Metadata: &EntryMetadata{
			ContentType: "text",
			Tags:        []string{"go", "cli", "tool"},
		},
	})
	assert.NoError(t, err)
}

func TestValidateClassifyResultInvalidMetadata(t *testing.T) {
	err := validateClassifyResult(&classifyOnlyResult{
		TopicPath: "ai/tool",
		WikiType:  TypeDeepDive,
		Metadata: &EntryMetadata{
			ContentType: "invalid",
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "metadata")
}

// --- NewClassifier with zero defaults ---

func TestNewClassifierZeroCandidateLimit(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "", WithCandidateLimit(0))
	assert.Equal(t, 120, c.CandidateLimit)
}

func TestNewClassifierZeroMinConfidence(t *testing.T) {
	// MinConfidence <= 0 gets set to 0.45
	c := NewClassifier(nil, t.TempDir(), "")
	assert.Greater(t, c.MinConfidence, 0.0)
}

// --- appendUniqueTopicCandidates edge cases ---

func TestAppendUniqueTopicCandidatesEmptyPath(t *testing.T) {
	items := []ghindex.TopicCandidate{{Path: "  ", Display: "empty"}}
	result := appendUniqueTopicCandidates(nil, make(map[string]bool), items)
	assert.Empty(t, result)
}

func TestAppendUniqueTopicCandidatesDuplicatePath(t *testing.T) {
	seen := map[string]bool{"ai/tool/demo": true}
	items := []ghindex.TopicCandidate{{Path: "ai/tool/demo"}}
	result := appendUniqueTopicCandidates(nil, seen, items)
	assert.Empty(t, result)
}

// --- validateAIClassification (full pipeline) ---

func TestValidateAIClassificationManualReviewWithGoodContent(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	result, err := c.validateAIClassification(&aiClassification{
		NeedsManualReview: true,
		Summary:           &StructuredSummary{Overview: "good overview", KeyPoints: []string{"point"}},
		WikiType:          TypeDeepDive,
		Confidence:        0.9,
	}, nil, ContentText)
	require.NoError(t, err)
	assert.True(t, result.NeedsManualReview)
	assert.Empty(t, result.TopicPath)
	assert.Equal(t, RouteReasonNoTopicMatch, result.RouteReason)
}

func TestValidateAIClassificationManualReviewPromotesValidTopic(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	result, err := c.validateAIClassification(&aiClassification{
		TopicPath:         "ai/tool/demo",
		NeedsManualReview: true,
		Summary:           &StructuredSummary{Overview: "good overview", KeyPoints: []string{"point"}},
		WikiType:          TypeInbox,
		Confidence:        0.9,
	}, []ghindex.TopicCandidate{{Path: "ai/tool/demo"}}, ContentText)
	require.NoError(t, err)
	assert.False(t, result.NeedsManualReview)
	assert.Equal(t, "ai/tool/demo", result.TopicPath)
	assert.Equal(t, TypeDeepDive, result.WikiType)
	assert.Equal(t, "ai/tool/demo", result.SuggestedTopic)
}

func TestValidateAIClassificationRejectReason(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	result, err := c.validateAIClassification(&aiClassification{
		RejectReason: "content not suitable",
		WikiType:     TypeDeepDive,
		ContentType:  ContentText,
		Confidence:   0.9,
		Summary:      &StructuredSummary{Overview: "overview"},
	}, nil, ContentText)
	require.Error(t, err)
	require.NotNil(t, result)
	assert.Contains(t, result.RejectReason, "content not suitable")
}

func TestValidateAIClassificationEmptySummary(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	result, err := c.validateAIClassification(&aiClassification{
		TopicPath:   "ai/tool/demo",
		WikiType:    TypeDeepDive,
		ContentType: ContentText,
		Confidence:  0.9,
		Summary:     nil,
	}, []ghindex.TopicCandidate{{Path: "ai/tool/demo"}}, ContentText)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "unavailable")
}

func TestValidateAIClassificationWhitespaceOverview(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	result, err := c.validateAIClassification(&aiClassification{
		TopicPath:   "ai/tool/demo",
		WikiType:    TypeDeepDive,
		ContentType: ContentText,
		Confidence:  0.9,
		Summary:     &StructuredSummary{Overview: "   "},
	}, []ghindex.TopicCandidate{{Path: "ai/tool/demo"}}, ContentText)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "unavailable")
}

// --- ensureWithinWikiRoot ---

func TestEnsureWithinWikiRoot(t *testing.T) {
	root := t.TempDir()
	err := ensureWithinWikiRoot(root, "topic/path")
	assert.NoError(t, err)
}

func TestEnsureWithinWikiRootTraversal(t *testing.T) {
	root := t.TempDir()
	err := ensureWithinWikiRoot(root, "../escape")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path traversal")
}

// --- formatTopicCandidates with empty display ---

func TestFormatTopicCandidatesEmptyDisplay(t *testing.T) {
	candidates := []ghindex.TopicCandidate{
		{Path: "ai/tool/demo", Display: "", Source: "wiki"},
	}
	result := formatTopicCandidates(candidates)
	assert.Contains(t, result, "path: ai/tool/demo | source: wiki")
	assert.NotContains(t, result, "title:")
}

// --- scanTypeCandidates edge cases ---

func TestScanTypeCandidatesSkipsNonDir(t *testing.T) {
	root := t.TempDir()
	topDir := filepath.Join(root, "tech", "research")
	require.NoError(t, os.MkdirAll(topDir, 0o700))
	// Create a file (not directory) in the type level
	require.NoError(t, os.WriteFile(filepath.Join(topDir, "file.txt"), []byte("content"), 0o600))

	entries, err := os.ReadDir(topDir)
	require.NoError(t, err)

	var candidates []ghindex.TopicCandidate
	for _, entry := range entries {
		candidates = scanTypeCandidates(topDir, "tech", entry, candidates)
	}
	assert.Empty(t, candidates)
}

func TestScanTypeCandidatesSkipsHiddenDirs(t *testing.T) {
	root := t.TempDir()
	topDir := filepath.Join(root, "tech", "research")
	require.NoError(t, os.MkdirAll(filepath.Join(topDir, ".hidden", "topic"), 0o700))

	entries, err := os.ReadDir(topDir)
	require.NoError(t, err)

	var candidates []ghindex.TopicCandidate
	for _, entry := range entries {
		candidates = scanTypeCandidates(topDir, "tech", entry, candidates)
	}
	assert.Empty(t, candidates)
}

// --- scanTopLevelCandidates edge cases ---

func TestScanTopLevelCandidatesSkipsHidden(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".hidden", "type", "topic"), 0o700))

	entries, err := os.ReadDir(root)
	require.NoError(t, err)

	var candidates []ghindex.TopicCandidate
	for _, entry := range entries {
		candidates = scanTopLevelCandidates(root, entry, candidates)
	}
	assert.Empty(t, candidates)
}

func TestScanTopLevelCandidatesSkipsWikiPrototype(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "wiki-prototype", "type", "topic"), 0o700))

	entries, err := os.ReadDir(root)
	require.NoError(t, err)

	var candidates []ghindex.TopicCandidate
	for _, entry := range entries {
		candidates = scanTopLevelCandidates(root, entry, candidates)
	}
	assert.Empty(t, candidates)
}

func TestScanTopLevelCandidatesSkipsFailed(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "failed", "type", "topic"), 0o700))

	entries, err := os.ReadDir(root)
	require.NoError(t, err)

	var candidates []ghindex.TopicCandidate
	for _, entry := range entries {
		candidates = scanTopLevelCandidates(root, entry, candidates)
	}
	assert.Empty(t, candidates)
}

// --- appendUniqueTopicCandidates edge cases ---

func TestAppendUniqueTopicCandidatesInvalidPath(t *testing.T) {
	items := []ghindex.TopicCandidate{{Path: "../escape"}}
	result := appendUniqueTopicCandidates(nil, make(map[string]bool), items)
	assert.Empty(t, result)
}

// --- rankTopicCandidates edge cases ---

func TestRankTopicCandidatesLimitExceedsLength(t *testing.T) {
	candidates := []ghindex.TopicCandidate{
		{Path: "a/b/c", Display: "c"},
	}
	result := rankTopicCandidates(candidates, "c", 100)
	assert.Len(t, result, 1)
}

// --- scoreTopicCandidate edge cases ---

func TestScoreTopicCandidateShortTokens(t *testing.T) {
	candidate := ghindex.TopicCandidate{Path: "a/b/c", Display: "c"}
	// Single char tokens should not score
	score := scoreTopicCandidate(candidate, "a b c")
	assert.Equal(t, 0, score)
}

// --- renderPrompt edge cases ---

func TestRenderPromptAllFields(t *testing.T) {
	prompt, err := renderPrompt("classify-json.txt", &promptData{
		Title:         "test",
		URL:           "https://example.com",
		Content:       "content",
		CandidateTree: "candidates",
		ContentType:   "text",
	})
	require.NoError(t, err)
	assert.Contains(t, prompt, "test")
	assert.Contains(t, prompt, "https://example.com")
}

// --- ghTopicCatalog edge cases ---

func TestGhTopicCatalogCachedResult(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	c.loadGHTopics = func() ([]ghindex.TopicCandidate, error) {
		return []ghindex.TopicCandidate{{Path: "cached/topic"}}, nil
	}

	// First call loads
	result1, err := c.ghTopicCatalog()
	require.NoError(t, err)
	assert.Len(t, result1, 1)

	// Second call uses cache
	result2, err := c.ghTopicCatalog()
	require.NoError(t, err)
	assert.Len(t, result2, 1)
}

func TestGhTopicCatalogErrorThenRecovery(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	callCount := 0
	c.loadGHTopics = func() ([]ghindex.TopicCandidate, error) {
		callCount++
		if callCount == 1 {
			return nil, errors.New("network error")
		}

		return []ghindex.TopicCandidate{{Path: "recovered/topic"}}, nil
	}

	// First call fails
	_, err := c.ghTopicCatalog()
	require.Error(t, err)

	// Second call recovers
	result, err := c.ghTopicCatalog()
	require.NoError(t, err)
	assert.Len(t, result, 1)
}

// --- DetectContentType edge cases ---

func TestDetectContentTypeEmptyURL(t *testing.T) {
	assert.Equal(t, ContentText, DetectContentType(""))
}

func TestDetectContentTypeWhitespaceURL(t *testing.T) {
	assert.Equal(t, ContentText, DetectContentType("  "))
}

// --- NewClassifier with negative CandidateLimit ---

func TestNewClassifierNegativeCandidateLimit(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "", WithCandidateLimit(-1))
	assert.Equal(t, 120, c.CandidateLimit)
}

// --- RenderStructuredSummary with only string fields ---

func TestRenderStructuredSummaryOnlyOverview(t *testing.T) {
	s := &StructuredSummary{
		Overview: "just overview",
	}
	rendered := RenderStructuredSummary(s)
	assert.Contains(t, rendered, "overview")
	assert.Contains(t, rendered, "just overview")
}

func TestRenderStructuredSummaryOnlyKeyPoints(t *testing.T) {
	s := &StructuredSummary{
		KeyPoints: []string{"point 1"},
	}
	rendered := RenderStructuredSummary(s)
	assert.Contains(t, rendered, "keyPoints")
	assert.Contains(t, rendered, "- point 1")
}

// --- validateAIClassification with valid full result ---

func TestValidateAIClassificationValidFullResult(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	result, err := c.validateAIClassification(&aiClassification{
		TopicPath:   "ai/tool/demo",
		WikiType:    TypeDeepDive,
		ContentType: ContentText,
		Confidence:  0.9,
		Summary: &StructuredSummary{
			Overview:    "overview",
			KeyPoints:   []string{"point"},
			WorthNoting: "note",
		},
	}, []ghindex.TopicCandidate{{Path: "ai/tool/demo"}}, ContentText)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "ai/tool/demo", result.TopicPath)
	assert.Equal(t, ContentText, result.ContentType)
	assert.Equal(t, 0.9, result.Confidence)
}

// --- buildClassifyResult ---

func TestBuildClassifyResultManualReviewWithGoodContent(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	result := c.buildClassifyResult(&aiClassification{
		NeedsManualReview: true,
		WikiType:          TypeDeepDive,
		Confidence:        0.9,
		Summary:           &StructuredSummary{Overview: "good content", KeyPoints: []string{"point"}},
	}, ContentText, nil, "https://example.com")
	require.NotNil(t, result)
	assert.True(t, result.NeedsManualReview)
	assert.Empty(t, result.TopicPath)
	assert.Equal(t, RouteReasonNoTopicMatch, result.RouteReason)
}

func TestBuildClassifyResultRejectReason(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	result := c.buildClassifyResult(&aiClassification{
		RejectReason: "content not suitable",
		WikiType:     TypeDeepDive,
		ContentType:  ContentText,
		Confidence:   0.9,
		Summary:      &StructuredSummary{Overview: "overview"},
	}, ContentText, nil, "https://example.com")
	require.NotNil(t, result)
	assert.Contains(t, result.RejectReason, "content not suitable")
}

func TestBuildClassifyResultValidationFails(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	result := c.buildClassifyResult(&aiClassification{
		WikiType:   "invalid",
		Confidence: 0.9,
		Summary:    &StructuredSummary{Overview: "overview", KeyPoints: []string{"point"}},
	}, ContentText, nil, "https://example.com")
	require.NotNil(t, result)
	assert.Contains(t, result.RejectReason, "invalid wiki type")
}

func TestBuildClassifyResultTopicValidationFails(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	result := c.buildClassifyResult(&aiClassification{
		TopicPath:   "../escape",
		WikiType:    TypeDeepDive,
		ContentType: ContentText,
		Confidence:  0.9,
		Summary:     &StructuredSummary{Overview: "overview", KeyPoints: []string{"point"}},
	}, ContentText, []ghindex.TopicCandidate{{Path: "ai/tool/demo"}}, "https://example.com")
	require.NotNil(t, result)
	assert.Empty(t, result.TopicPath)
	assert.True(t, result.NeedsManualReview)
	assert.Equal(t, RouteReasonInvalidTopicPath, result.RouteReason)
	assert.Equal(t, "../escape", result.SuggestedTopic)
}

func TestBuildClassifyResultEmptySummary(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	result := c.buildClassifyResult(&aiClassification{
		TopicPath:   "ai/tool/demo",
		WikiType:    TypeDeepDive,
		ContentType: ContentText,
		Confidence:  0.9,
		Summary:     nil,
	}, ContentText, []ghindex.TopicCandidate{{Path: "ai/tool/demo"}}, "https://example.com")
	assert.Nil(t, result)
}

func TestBuildClassifyResultWhitespaceOverview(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	result := c.buildClassifyResult(&aiClassification{
		TopicPath:   "ai/tool/demo",
		WikiType:    TypeDeepDive,
		ContentType: ContentText,
		Confidence:  0.9,
		Summary:     &StructuredSummary{Overview: "   "},
	}, ContentText, []ghindex.TopicCandidate{{Path: "ai/tool/demo"}}, "https://example.com")
	assert.Nil(t, result)
}

func TestBuildClassifyResultValidFullResult(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	result := c.buildClassifyResult(&aiClassification{
		TopicPath:   "ai/tool/demo",
		WikiType:    TypeDeepDive,
		ContentType: ContentText,
		Confidence:  0.9,
		Summary: &StructuredSummary{
			Overview:    "overview",
			KeyPoints:   []string{"point"},
			WorthNoting: "note",
		},
		Metadata: &EntryMetadata{
			ContentType: "text",
			Tags:        []string{"go", "cli", "tool"},
		},
	}, ContentText, []ghindex.TopicCandidate{{Path: "ai/tool/demo"}}, "https://example.com")
	require.NotNil(t, result)
	assert.Equal(t, "ai/tool/demo", result.TopicPath)
	assert.Equal(t, ContentText, result.ContentType)
	assert.Equal(t, 0.9, result.Confidence)
	assert.NotEmpty(t, result.MetadataBlock)
}

func TestBuildClassifyResultWithNeedsManualReview(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	// NMR + valid path + high conf → promote to topic write
	result := c.buildClassifyResult(&aiClassification{
		TopicPath:         "ai/tool/demo",
		WikiType:          TypeDeepDive,
		ContentType:       ContentText,
		Confidence:        0.9,
		NeedsManualReview: true,
		Summary: &StructuredSummary{
			Overview:  "overview",
			KeyPoints: []string{"point"},
		},
	}, ContentText, []ghindex.TopicCandidate{{Path: "ai/tool/demo"}}, "https://example.com")
	require.NotNil(t, result)
	assert.False(t, result.NeedsManualReview)
	assert.Equal(t, "ai/tool/demo", result.TopicPath)
}

func TestBuildClassifyResultNMRNoneGoesUncat(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	result := c.buildClassifyResult(&aiClassification{
		TopicPath:         "none",
		WikiType:          TypeInbox,
		ContentType:       ContentText,
		Confidence:        0.9,
		NeedsManualReview: true,
		Summary: &StructuredSummary{
			Overview:  "overview",
			KeyPoints: []string{"point"},
		},
	}, ContentText, []ghindex.TopicCandidate{{Path: "ai/tool/demo"}}, "https://example.com")
	require.NotNil(t, result)
	assert.True(t, result.NeedsManualReview)
	assert.Empty(t, result.TopicPath)
	assert.Equal(t, RouteReasonNoTopicMatch, result.RouteReason)
}

func TestBuildClassifyResultNMRLowConfKeepsUncat(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	result := c.buildClassifyResult(&aiClassification{
		TopicPath:         "ai/tool/demo",
		WikiType:          TypeDeepDive,
		ContentType:       ContentText,
		Confidence:        0.1,
		NeedsManualReview: true,
		Summary: &StructuredSummary{
			Overview:  "overview",
			KeyPoints: []string{"point"},
		},
	}, ContentText, []ghindex.TopicCandidate{{Path: "ai/tool/demo"}}, "https://example.com")
	require.NotNil(t, result)
	assert.True(t, result.NeedsManualReview)
	assert.Empty(t, result.TopicPath)
	assert.Equal(t, RouteReasonNeedsManualReview, result.RouteReason)
	assert.Equal(t, "ai/tool/demo", result.SuggestedTopic)
}

// --- NewClassifier with negative MinConfidence ---

func TestNewClassifierNegativeMinConfidence(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	// MinConfidence is set to 0.30 in constructor, then 0.45 if <= 0
	assert.Greater(t, c.MinConfidence, 0.0)
}


func TestNormalizeTopicLeafKey(t *testing.T) {
	assert.Equal(t, "golang代码常用写法", normalizeTopicLeafKey("***golang代码常用写法***（comma-ok, options模式）"))
	assert.Equal(t, "quic", normalizeTopicLeafKey("QUIC"))
}

func TestFuzzyMatchTopicPathQUICWrongParent(t *testing.T) {
	cands := []ghindex.TopicCandidate{
		{Path: "kernel/HTTP/QUIC", Display: "QUIC"},
		{Path: "kernel/NP/UDP", Display: "UDP"},
	}
	got, ok := fuzzyMatchTopicPath("kernel/NP/QUIC", cands)
	require.True(t, ok)
	assert.Equal(t, "kernel/HTTP/QUIC", got)
}

func TestFuzzyMatchTopicPathGolangDecoratedLeaf(t *testing.T) {
	cands := []ghindex.TopicCandidate{
		{Path: "langs/golang/***golang代码常用写法***（comma-ok, options模式, builder模式, private-struct(avoid call), Callback as param）", Display: "***golang代码常用写法***（comma-ok, options模式, builder模式, private-struct(avoid call), Callback as param）"},
		{Path: "langs/golang/slice", Display: "slice"},
	}
	got, ok := fuzzyMatchTopicPath("langs/golang/golang代码常用写法", cands)
	require.True(t, ok)
	assert.Contains(t, got, "golang代码常用写法")
}

func TestResolveWritableTopicPathUsesFuzzy(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	cands := []ghindex.TopicCandidate{{Path: "kernel/HTTP/QUIC", Display: "QUIC"}}
	got, ok := c.resolveWritableTopicPath("kernel/NP/QUIC", cands)
	require.True(t, ok)
	assert.Equal(t, "kernel/HTTP/QUIC", got)
}
