package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAIProviderSetsLang(t *testing.T) {
	p := NewAIProvider(AIConfig{Language: "en"})
	assert.Equal(t, "en", p.lang)

	p2 := NewAIProvider(AIConfig{})
	assert.Equal(t, "zh", p2.lang)
}

func TestIsConfigured(t *testing.T) {
	p := NewAIProvider(AIConfig{APIKey: "sk-test"})
	assert.True(t, p.IsConfigured())

	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("LLM_AxonHub", "")
	p2 := NewAIProvider(AIConfig{APIKey: ""})
	assert.False(t, p2.IsConfigured())
}

func TestMorningClassifyRendersPrompt(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("LLM_AxonHub", "")
	p := NewAIProvider(AIConfig{Language: "en", APIKey: ""})
	// No API key => chat returns empty, but prompt rendering should succeed.
	got := p.MorningClassify([]IssueView{
		{Identifier: "LUC-101", Title: "Fix bug", TeamName: "Eng", Priority: "P0"},
	})
	assert.Empty(t, got, "without API key, chat should return empty")
}

func TestMorningSummaryReturnsEmptyWithoutKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("LLM_AxonHub", "")
	p := NewAIProvider(AIConfig{APIKey: ""})
	got := p.MorningSummary([]IssueView{{Identifier: "LUC-1", Title: "Task"}})
	assert.Empty(t, got)
}

func TestMorningStructuredReviewDelegatesToClassify(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("LLM_AxonHub", "")
	p := NewAIProvider(AIConfig{APIKey: ""})
	got := p.MorningStructuredReview([]IssueView{{Identifier: "LUC-1", Title: "Task"}})
	assert.Empty(t, got)
}

func TestMorningDeepAnalysisReturnsEmptyWithoutKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("LLM_AxonHub", "")
	p := NewAIProvider(AIConfig{APIKey: ""})
	got := p.MorningDeepAnalysis([]IssueDetail{{Identifier: "LUC-1", Title: "Task"}})
	assert.Empty(t, got)
}

func TestEveningDeepReviewReturnsEmptyWithoutKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("LLM_AxonHub", "")
	p := NewAIProvider(AIConfig{APIKey: ""})
	got := p.EveningDeepReview([]IssueDetail{{Identifier: "LUC-1", Title: "Task"}})
	assert.Empty(t, got)
}

func TestRenderPromptWithValidTemplate(t *testing.T) {
	p := NewAIProvider(AIConfig{Language: "en"})
	prompt, err := p.renderPrompt("prompts/morning-summary.txt", morningClassifyData{
		Lang: "en",
		Issues: []IssueView{
			{Identifier: "LUC-1", Title: "Test", TeamName: "Eng", Priority: "P0"},
		},
	})
	require.NoError(t, err)
	assert.Contains(t, prompt, "LUC-1")
	assert.Contains(t, prompt, "Test")
}

func TestRenderPromptWithInvalidTemplate(t *testing.T) {
	p := NewAIProvider(AIConfig{})
	_, err := p.renderPrompt("prompts/nonexistent.txt", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse prompt")
}
