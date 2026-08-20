package classify

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/internal/docs/wiki/fetch"
	"github.com/xbpk3t/docs-alfred/internal/docs/wiki/prompt"
	"github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
	"github.com/xbpk3t/docs-alfred/internal/gh/index"
	"github.com/xbpk3t/docs-alfred/pkg/ai"
)

func TestRenderPrompt(t *testing.T) {
	prompt, err := prompt.Render("classify-json.txt", &promptData{
		Title:         "A title",
		URL:           "https://example.com/post",
		Content:       "A summary",
		CandidateTree: "- path: ai/tool/demo | title: Demo | source: test",
	})
	require.NoError(t, err, "prompt.Render() error")

	for _, want := range []string{"A title", "https://example.com/post", "A summary"} {
		assert.Contains(t, prompt, want, "rendered prompt should contain %q", want)
	}
	assert.NotContains(t, prompt, "{{", "rendered prompt should not contain template marker")
}

func TestRejectedClassifyResultPreservesDiagnostics(t *testing.T) {
	result := rejectedClassifyResult(&aiClassification{
		TopicPath:         "ai/tool/demo",
		WikiType:          types.TypeInbox,
		ContentType:       types.ContentText,
		Summary:           &types.StructuredSummary{Overview: "manual summary", WorthNoting: ""},
		Confidence:        0.42,
		NeedsManualReview: true,
	}, types.ContentText, assert.AnError)

	require.NotNil(t, result)
	assert.Equal(t, "ai/tool/demo", result.TopicPath)
	assert.Equal(t, types.TypeInbox, result.WikiType)
	require.NotNil(t, result.Summary)
	assert.Equal(t, "manual summary", result.Summary.Overview)
	assert.Equal(t, 0.42, result.Confidence)
	assert.True(t, result.NeedsManualReview)
	assert.Contains(t, result.RejectReason, assert.AnError.Error())
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
		WikiType:          types.TypeDeepDive,
		Confidence:        0.9,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "manual review")
}

func TestValidateAIClassificationBasicsLowConfidence(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	err := c.validateAIClassificationBasics(&aiClassification{
		WikiType:   types.TypeDeepDive,
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
		WikiType:    types.TypeDeepDive,
		ContentType: "invalid",
		Confidence:  0.9,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid content type")
}

func TestValidateAIClassificationBasicsValid(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	err := c.validateAIClassificationBasics(&aiClassification{
		WikiType:    types.TypeDeepDive,
		ContentType: types.ContentText,
		Confidence:  0.9,
	})
	assert.NoError(t, err)
}

func TestValidateAIClassificationBasicsEmptyContentType(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	err := c.validateAIClassificationBasics(&aiClassification{
		WikiType:   types.TypeDeepDive,
		Confidence: 0.9,
	})
	assert.NoError(t, err)
}

// --- rejectedClassifyResult ---

func TestRejectedClassifyResultNilResult(t *testing.T) {
	assert.Nil(t, rejectedClassifyResult(nil, "", nil))
}

func TestRejectedClassifyResultNilError(t *testing.T) {
	result := rejectedClassifyResult(&aiClassification{
		TopicPath:   "ai/tool/demo",
		WikiType:    types.TypeDeepDive,
		ContentType: types.ContentText,
		Confidence:  0.5,
	}, types.ContentText, nil)
	require.NotNil(t, result)
	assert.Equal(t, "classification rejected", result.RejectReason)
}

func TestRejectedClassifyResultEmptyContentType(t *testing.T) {
	result := rejectedClassifyResult(&aiClassification{
		TopicPath:   "ai/tool/demo",
		WikiType:    types.TypeDeepDive,
		ContentType: types.ContentVideo,
		Confidence:  0.5,
	}, "", nil)
	require.NotNil(t, result)
	assert.Equal(t, types.ContentVideo, result.ContentType)
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
	s := &types.StructuredSummary{
		Overview:  "overview",
		KeyPoints: []string{"point"},
		Detail:    "detailed analysis here",
	}
	rendered := RenderStructuredSummary(s)
	assert.Contains(t, rendered, "detail")
	assert.Contains(t, rendered, "detailed analysis here")
}

func TestRenderStructuredSummaryWithActionableAdvice(t *testing.T) {
	s := &types.StructuredSummary{
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
		WikiType:  types.TypeDeepDive,
		Summary: &types.StructuredSummary{
			Overview:  "overview",
			KeyPoints: []string{"point"},
		},
	})
	assert.NoError(t, err)
}

func TestValidateClassifyResultInvalidSummary(t *testing.T) {
	err := validateClassifyResult(&classifyOnlyResult{
		TopicPath: "ai/tool",
		WikiType:  types.TypeDeepDive,
		Summary: &types.StructuredSummary{
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
		WikiType:  types.TypeDeepDive,
		Metadata: &types.EntryMetadata{
			ContentType: "text",
			Tags:        []string{"go", "cli", "tool"},
		},
	})
	assert.NoError(t, err)
}

func TestValidateClassifyResultInvalidMetadata(t *testing.T) {
	err := validateClassifyResult(&classifyOnlyResult{
		TopicPath: "ai/tool",
		WikiType:  types.TypeDeepDive,
		Metadata: &types.EntryMetadata{
			ContentType: "invalid",
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "metadata")
}

// --- NewClassifier with zero defaults ---


func TestNewClassifierZeroMinConfidence(t *testing.T) {
	// MinConfidence <= 0 gets set to 0.45
	c := NewClassifier(nil, t.TempDir(), "")
	assert.Greater(t, c.MinConfidence, 0.0)
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

// --- renderPrompt edge cases ---

func TestRenderPromptAllFields(t *testing.T) {
	prompt, err := prompt.Render("classify-json.txt", &promptData{
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
	assert.Equal(t, types.ContentText, fetch.DetectContentType(""))
}

func TestRenderStructuredSummaryOnlyOverview(t *testing.T) {
	s := &types.StructuredSummary{
		Overview: "just overview",
	}
	rendered := RenderStructuredSummary(s)
	assert.Contains(t, rendered, "overview")
	assert.Contains(t, rendered, "just overview")
}

func TestRenderStructuredSummaryOnlyKeyPoints(t *testing.T) {
	s := &types.StructuredSummary{
		KeyPoints: []string{"point 1"},
	}
	rendered := RenderStructuredSummary(s)
	assert.Contains(t, rendered, "keyPoints")
	assert.Contains(t, rendered, "- point 1")
}

// --- buildClassifyResult ---

func TestBuildClassifyResultManualReviewWithGoodContent(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	result := c.buildClassifyResult(&aiClassification{
		NeedsManualReview: true,
		WikiType:          types.TypeDeepDive,
		Confidence:        0.9,
		Summary:           &types.StructuredSummary{Overview: "good content", KeyPoints: []string{"point"}},
	}, types.ContentText, nil, "https://example.com")
	require.NotNil(t, result)
	assert.True(t, result.NeedsManualReview)
	assert.Empty(t, result.TopicPath)
	assert.Equal(t, types.RouteReasonNoTopicMatch, result.RouteReason)
}

func TestBuildClassifyResultRejectReason(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	result := c.buildClassifyResult(&aiClassification{
		RejectReason: "content not suitable",
		WikiType:     types.TypeDeepDive,
		ContentType:  types.ContentText,
		Confidence:   0.9,
		Summary:      &types.StructuredSummary{Overview: "overview"},
	}, types.ContentText, nil, "https://example.com")
	require.NotNil(t, result)
	assert.Contains(t, result.RejectReason, "content not suitable")
}

func TestBuildClassifyResultValidationFails(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	result := c.buildClassifyResult(&aiClassification{
		WikiType:   "invalid",
		Confidence: 0.9,
		Summary:    &types.StructuredSummary{Overview: "overview", KeyPoints: []string{"point"}},
	}, types.ContentText, nil, "https://example.com")
	require.NotNil(t, result)
	assert.Contains(t, result.RejectReason, "invalid wiki type")
}

func TestBuildClassifyResultTopicValidationFails(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	result := c.buildClassifyResult(&aiClassification{
		TopicPath:   "../escape",
		WikiType:    types.TypeDeepDive,
		ContentType: types.ContentText,
		Confidence:  0.9,
		Summary:     &types.StructuredSummary{Overview: "overview", KeyPoints: []string{"point"}},
	}, types.ContentText, []ghindex.TopicCandidate{{Path: "ai/tool/demo"}}, "https://example.com")
	require.NotNil(t, result)
	assert.Empty(t, result.TopicPath)
	assert.True(t, result.NeedsManualReview)
	assert.Equal(t, types.RouteReasonInvalidTopicPath, result.RouteReason)
	assert.Equal(t, "../escape", result.SuggestedTopic)
}

func TestBuildClassifyResultEmptySummary(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	result := c.buildClassifyResult(&aiClassification{
		TopicPath:   "ai/tool/demo",
		WikiType:    types.TypeDeepDive,
		ContentType: types.ContentText,
		Confidence:  0.9,
		Summary:     nil,
	}, types.ContentText, []ghindex.TopicCandidate{{Path: "ai/tool/demo"}}, "https://example.com")
	assert.Nil(t, result)
}

func TestBuildClassifyResultWhitespaceOverview(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	result := c.buildClassifyResult(&aiClassification{
		TopicPath:   "ai/tool/demo",
		WikiType:    types.TypeDeepDive,
		ContentType: types.ContentText,
		Confidence:  0.9,
		Summary:     &types.StructuredSummary{Overview: "   "},
	}, types.ContentText, []ghindex.TopicCandidate{{Path: "ai/tool/demo"}}, "https://example.com")
	assert.Nil(t, result)
}

func TestBuildClassifyResultValidFullResult(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	result := c.buildClassifyResult(&aiClassification{
		TopicPath:   "ai/tool/demo",
		WikiType:    types.TypeDeepDive,
		ContentType: types.ContentText,
		Confidence:  0.9,
		Summary: &types.StructuredSummary{
			Overview:    "overview",
			KeyPoints:   []string{"point"},
			WorthNoting: "note",
		},
		Metadata: &types.EntryMetadata{
			ContentType: "text",
			Tags:        []string{"go", "cli", "tool"},
		},
	}, types.ContentText, []ghindex.TopicCandidate{{Path: "ai/tool/demo"}}, "https://example.com")
	require.NotNil(t, result)
	assert.Equal(t, "ai/tool/demo", result.TopicPath)
	assert.Equal(t, types.ContentText, result.ContentType)
	assert.Equal(t, 0.9, result.Confidence)
	assert.NotEmpty(t, result.MetadataBlock)
}

func TestBuildClassifyResultWithNeedsManualReview(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	// NMR + valid path + high conf → promote to topic write
	result := c.buildClassifyResult(&aiClassification{
		TopicPath:         "ai/tool/demo",
		WikiType:          types.TypeDeepDive,
		ContentType:       types.ContentText,
		Confidence:        0.9,
		NeedsManualReview: true,
		Summary: &types.StructuredSummary{
			Overview:  "overview",
			KeyPoints: []string{"point"},
		},
	}, types.ContentText, []ghindex.TopicCandidate{{Path: "ai/tool/demo"}}, "https://example.com")
	require.NotNil(t, result)
	assert.False(t, result.NeedsManualReview)
	assert.Equal(t, "ai/tool/demo", result.TopicPath)
}

func TestBuildClassifyResultNMRNoneGoesUncat(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	result := c.buildClassifyResult(&aiClassification{
		TopicPath:         "none",
		WikiType:          types.TypeInbox,
		ContentType:       types.ContentText,
		Confidence:        0.9,
		NeedsManualReview: true,
		Summary: &types.StructuredSummary{
			Overview:  "overview",
			KeyPoints: []string{"point"},
		},
	}, types.ContentText, []ghindex.TopicCandidate{{Path: "ai/tool/demo"}}, "https://example.com")
	require.NotNil(t, result)
	assert.True(t, result.NeedsManualReview)
	assert.Empty(t, result.TopicPath)
	assert.Equal(t, types.RouteReasonNoTopicMatch, result.RouteReason)
}

func TestBuildClassifyResultNMRLowConfKeepsUncat(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "")
	result := c.buildClassifyResult(&aiClassification{
		TopicPath:         "ai/tool/demo",
		WikiType:          types.TypeDeepDive,
		ContentType:       types.ContentText,
		Confidence:        0.1,
		NeedsManualReview: true,
		Summary: &types.StructuredSummary{
			Overview:  "overview",
			KeyPoints: []string{"point"},
		},
	}, types.ContentText, []ghindex.TopicCandidate{{Path: "ai/tool/demo"}}, "https://example.com")
	require.NotNil(t, result)
	assert.True(t, result.NeedsManualReview)
	assert.Empty(t, result.TopicPath)
	assert.Equal(t, types.RouteReasonNeedsManualReview, result.RouteReason)
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

// --- verify pass (LUC-302) ---

func TestRenderStructuredSummaryWithVerify(t *testing.T) {
	s := &types.StructuredSummary{
		Overview:  "overview",
		KeyPoints: []string{"point"},
		Verify:    "- **assertions**\n  - [Fact] default is 15s | risk=med | self-consistent",
	}
	rendered := RenderStructuredSummary(s)
	assert.Contains(t, rendered, "#### verify")
	assert.Contains(t, rendered, "assertions")
	assert.NotContains(t, rendered, "criticalThinking")
}

func TestShouldSkipVerifyRepoMetadata(t *testing.T) {
	assert.True(t, shouldSkipVerify(&aiClassification{
		WikiType: types.TypeDeepDive,
		Metadata: &types.EntryMetadata{ContentType: types.DisplayTypeRepo},
	}))
}

func TestShouldSkipVerifyReviewWikiType(t *testing.T) {
	assert.True(t, shouldSkipVerify(&aiClassification{
		WikiType: types.TypeRepoEval,
		Metadata: &types.EntryMetadata{ContentType: types.DisplayTypeText},
	}))
}

func TestShouldSkipVerifyTextResearch(t *testing.T) {
	assert.False(t, shouldSkipVerify(&aiClassification{
		WikiType: types.TypeDeepDive,
		Metadata: &types.EntryMetadata{ContentType: types.DisplayTypeText},
	}))
}

func TestParseVerifyOutputPlainMarkdown(t *testing.T) {
	in := "- **assertions**\n  - [Fact] x | risk=low | ok"
	assert.Equal(t, in, parseVerifyOutput(in))
}

func TestParseVerifyOutputJSONWrapper(t *testing.T) {
	in := `{"verify":"- **assertions**\n  - [Claim] y | risk=high | check"}`
	got := parseVerifyOutput(in)
	assert.Contains(t, got, "assertions")
	assert.Contains(t, got, "Claim")
}

func TestParseVerifyOutputFencedMarkdown(t *testing.T) {
	in := "```markdown\n- **gaps**\n  - none\n```"
	got := parseVerifyOutput(in)
	assert.Contains(t, got, "gaps")
	assert.NotContains(t, got, "```")
}

func TestMaybeVerifyFillsSummary(t *testing.T) {
	calls := 0
	c := NewClassifier(nil, t.TempDir(), "", WithChatFn(func(ctx context.Context, cfg *ai.ClientConfig, messages []ai.Message) (string, error) {
		calls++
		return "- **assertions**\n  - [Fact] port is 8443 | risk=med | from body\n- **issues**\n  - none\n- **gaps**\n  - none", nil
	}))
	classified := &aiClassification{
		WikiType: types.TypeDeepDive,
		Metadata: &types.EntryMetadata{ContentType: types.DisplayTypeText},
		Summary: &types.StructuredSummary{
			Overview:  "overview about ports",
			KeyPoints: []string{"uses 8443"},
		},
	}
	c.maybeVerify(context.Background(), classified, "https://example.com/a", "Title", types.ContentText, "default port is 8443", 2000)
	require.NotNil(t, classified.Summary)
	assert.Contains(t, classified.Summary.Verify, "assertions")
	assert.Equal(t, 1, calls)
}

func TestMaybeVerifySkipsRepo(t *testing.T) {
	calls := 0
	c := NewClassifier(nil, t.TempDir(), "", WithChatFn(func(ctx context.Context, cfg *ai.ClientConfig, messages []ai.Message) (string, error) {
		calls++
		return "should not run", nil
	}))
	classified := &aiClassification{
		WikiType: types.TypeRepoEval,
		Metadata: &types.EntryMetadata{ContentType: types.DisplayTypeRepo},
		Summary: &types.StructuredSummary{
			Overview:  "repo review",
			KeyPoints: []string{"stars"},
		},
	}
	c.maybeVerify(context.Background(), classified, "https://github.com/o/r", "repo", types.ContentText, "readme", 2000)
	assert.Empty(t, classified.Summary.Verify)
	assert.Equal(t, 0, calls)
}

func TestMaybeVerifyFailureDoesNotBreak(t *testing.T) {
	c := NewClassifier(nil, t.TempDir(), "", WithChatFn(func(ctx context.Context, cfg *ai.ClientConfig, messages []ai.Message) (string, error) {
		return "", errors.New("boom")
	}))
	classified := &aiClassification{
		WikiType: types.TypeDeepDive,
		Metadata: &types.EntryMetadata{ContentType: types.DisplayTypeText},
		Summary: &types.StructuredSummary{
			Overview:  "overview",
			KeyPoints: []string{"point"},
		},
	}
	c.maybeVerify(context.Background(), classified, "https://example.com/b", "Title", types.ContentText, "body", 2000)
	assert.Empty(t, classified.Summary.Verify)
	assert.Equal(t, "overview", classified.Summary.Overview)
}

func TestPromptRenderVerifySimple(t *testing.T) {
	out, err := prompt.Render("verify-simple.txt", &verifyPromptData{
		URL:         "https://example.com",
		Title:       "t",
		ContentType: "text",
		Overview:    "ov",
		KeyPoints:   []string{"k1"},
		Content:     "body",
	})
	require.NoError(t, err)
	assert.Contains(t, out, "https://example.com")
	assert.Contains(t, out, "assertions")
	assert.Contains(t, out, "body")
}
