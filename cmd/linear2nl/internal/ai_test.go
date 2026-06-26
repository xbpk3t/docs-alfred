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

func TestMorningPlanReturnsEmptyWithoutKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("LLM_AxonHub", "")
	p := NewAIProvider(AIConfig{APIKey: ""})
	got := p.MorningPlan([]IssueDetail{{Identifier: "LUC-1", Title: "Task"}})
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
	prompt, err := p.renderPrompt("prompts/plan.txt", planPromptData{
		Lang: "en",
		Issues: []IssueDetail{
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
