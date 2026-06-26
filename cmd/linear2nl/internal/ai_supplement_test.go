package internal

import (
	"embed"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newMockOpenAIServer creates an httptest server that mimics the OpenAI
// chat/completions endpoint. It returns the given content string for every
// request, or a 500 error if content is empty.
func newMockOpenAIServer(content string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			http.NotFound(w, r)

			return
		}

		if content == "" {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{"message": "mock error", "type": "server_error"},
			})

			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":    "chatcmpl-mock",
			"model": "test-model",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": content,
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		})
	}))
}

// --- chat() method tests ---

func TestChatReturnsEmptyWithoutAPIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("LLM_AxonHub", "")

	p := NewAIProvider(AIConfig{APIKey: ""})
	require.False(t, p.IsConfigured())

	got := p.chat("hello")
	assert.Empty(t, got)
}

func TestChatErrorPath(t *testing.T) {
	// Server that always returns 500. The retry loop in chat() will attempt
	// 3 times (with backoff), and then return "".
	srv := newMockOpenAIServer("")
	t.Cleanup(srv.Close)

	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("LLM_AxonHub", "")

	p := NewAIProvider(AIConfig{
		APIKey:  "sk-test",
		BaseURL: srv.URL,
		Model:   "test-model",
	})
	require.True(t, p.IsConfigured())

	got := p.chat("test prompt")
	assert.Empty(t, got, "chat should return empty after retry exhaustion")
}

func TestChatSuccessPath(t *testing.T) {
	srv := newMockOpenAIServer("  hello world  ")
	t.Cleanup(srv.Close)

	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("LLM_AxonHub", "")

	p := NewAIProvider(AIConfig{
		APIKey:  "sk-test",
		BaseURL: srv.URL,
		Model:   "test-model",
	})
	require.True(t, p.IsConfigured())

	got := p.chat("test prompt")
	assert.Equal(t, "hello world", got, "chat should TrimSpace the response")
}

// --- renderPrompt tests ---

func TestRenderPromptPlanTemplate(t *testing.T) {
	p := NewAIProvider(AIConfig{Language: "en"})

	prompt, err := p.renderPrompt("prompts/plan.txt", planPromptData{
		Lang: "en",
		Issues: []IssueDetail{
			{
				Identifier:  "LUC-50",
				Title:       "Deep task",
				Description: "Some description here",
				StateName:   "In Progress",
				TeamName:    "Platform",
				Priority:    "P0",
				Comments: []Comment{
					{Body: "needs attention", UserName: "alice", CreatedAt: "2024-06-01"},
					{Body: "already on it", UserName: "bob", CreatedAt: "2024-06-02"},
				},
			},
		},
	})
	require.NoError(t, err)
	assert.Contains(t, prompt, "LUC-50")
	assert.Contains(t, prompt, "Deep task")
	assert.Contains(t, prompt, "Some description here")
	assert.Contains(t, prompt, "alice")
	assert.Contains(t, prompt, "needs attention")
}

func TestRenderPromptSummaryTemplate(t *testing.T) {
	p := NewAIProvider(AIConfig{Language: "en"})

	prompt, err := p.renderPrompt("prompts/summary.txt", summaryPromptData{
		Lang: "en",
		Issues: []IssueDetail{
			{
				Identifier:  "LUC-60",
				Title:       "Evening review task",
				Description: "A detailed description",
				StateName:   "Done",
				TeamName:    "Eng",
				Priority:    "P1",
				Comments: []Comment{
					{Body: "completed successfully", UserName: "charlie", CreatedAt: "2024-06-03"},
				},
			},
		},
	})
	require.NoError(t, err)
	assert.Contains(t, prompt, "LUC-60")
	assert.Contains(t, prompt, "Evening review task")
	assert.Contains(t, prompt, "A detailed description")
	assert.Contains(t, prompt, "charlie")
}

func TestRenderPromptWithEmptyIssues(t *testing.T) {
	p := NewAIProvider(AIConfig{Language: "zh"})

	t.Run("plan", func(t *testing.T) {
		prompt, err := p.renderPrompt("prompts/plan.txt", planPromptData{
			Lang:   "zh",
			Issues: []IssueDetail{},
		})
		require.NoError(t, err)
		assert.NotEmpty(t, prompt, "template header should still render")
	})

	t.Run("summary", func(t *testing.T) {
		prompt, err := p.renderPrompt("prompts/summary.txt", summaryPromptData{
			Lang:   "zh",
			Issues: []IssueDetail{},
		})
		require.NoError(t, err)
		assert.NotEmpty(t, prompt)
	})
}

// --- Public methods with broken prompts (empty embed.FS) ---
// These test the renderPrompt error branches in each public method.

func TestMorningPlanRenderPromptError(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("LLM_AxonHub", "")

	p := NewAIProvider(AIConfig{Language: "en"})
	p.prompts = embed.FS{} // empty FS: ParseFS will fail

	got := p.MorningPlan([]IssueDetail{{Identifier: "LUC-1", Title: "Task"}})
	assert.Empty(t, got)
}

func TestEveningDeepReviewRenderPromptError(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("LLM_AxonHub", "")

	p := NewAIProvider(AIConfig{Language: "en"})
	p.prompts = embed.FS{}

	got := p.EveningDeepReview([]IssueDetail{{Identifier: "LUC-1", Title: "Task"}})
	assert.Empty(t, got)
}

// --- Public methods with mock API (full success paths) ---

func TestMorningPlanWithMockAPI(t *testing.T) {
	srv := newMockOpenAIServer(`{"reviews":[{"identifier":"LUC-102","title":"Task"}]}`)
	t.Cleanup(srv.Close)

	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("LLM_AxonHub", "")

	p := NewAIProvider(AIConfig{
		APIKey:   "sk-test",
		BaseURL:  srv.URL,
		Model:    "test-model",
		Language: "en",
	})

	got := p.MorningPlan([]IssueDetail{
		{
			Identifier: "LUC-102", Title: "Task",
			Description: "Do something", StateName: "Todo",
			TeamName: "Eng", Priority: "P0",
		},
	})
	assert.Contains(t, got, "LUC-102")
}

func TestEveningDeepReviewWithMockAPI(t *testing.T) {
	srv := newMockOpenAIServer("review: task completed well")
	t.Cleanup(srv.Close)

	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("LLM_AxonHub", "")

	p := NewAIProvider(AIConfig{
		APIKey:   "sk-test",
		BaseURL:  srv.URL,
		Model:    "test-model",
		Language: "en",
	})

	got := p.EveningDeepReview([]IssueDetail{
		{
			Identifier: "LUC-103", Title: "Evening task",
			Description: "Something done", StateName: "Done",
			TeamName: "Eng", Priority: "P1",
			Comments: []Comment{
				{Body: "shipped it", UserName: "dev", CreatedAt: "2024-06-23"},
			},
		},
	})
	assert.Equal(t, "review: task completed well", got)
}

// --- IsConfigured edge cases ---

func TestIsConfiguredWithEnvAPIKey(t *testing.T) {
	t.Run("OPENAI_API_KEY set in env", func(t *testing.T) {
		t.Setenv("OPENAI_API_KEY", "sk-from-env")
		t.Setenv("LLM_AxonHub", "")

		p := NewAIProvider(AIConfig{APIKey: ""})
		assert.True(t, p.IsConfigured(), "should pick up API key from env")
	})

	t.Run("LLM_AxonHub set in env", func(t *testing.T) {
		t.Setenv("OPENAI_API_KEY", "")
		t.Setenv("LLM_AxonHub", "sk-from-axonhub")

		p := NewAIProvider(AIConfig{APIKey: ""})
		assert.True(t, p.IsConfigured(), "should pick up LLM_AxonHub fallback")
	})

	t.Run("explicit key overrides env", func(t *testing.T) {
		t.Setenv("OPENAI_API_KEY", "sk-env")
		t.Setenv("LLM_AxonHub", "")

		p := NewAIProvider(AIConfig{APIKey: "sk-explicit"})
		assert.True(t, p.IsConfigured())
		assert.Equal(t, "sk-explicit", p.clientCfg.APIKey)
	})
}

func TestNewAIProviderDefaultLanguage(t *testing.T) {
	p := NewAIProvider(AIConfig{})
	assert.Equal(t, "zh", p.lang, "default language should be zh")
}

func TestNewAIProviderCustomLanguage(t *testing.T) {
	p := NewAIProvider(AIConfig{Language: "en"})
	assert.Equal(t, "en", p.lang)
}
