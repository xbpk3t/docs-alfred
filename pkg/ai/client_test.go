package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tmc/langchaingo/llms"
)

func TestDefaultConfig_Defaults(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_BASE_URL", "")
	t.Setenv("LLM_AxonHub", "")
	t.Setenv("LLM_MODEL", "")

	cfg := DefaultConfig()
	assert.NotNil(t, cfg)
	assert.Empty(t, cfg.APIKey, "no env var → empty key")
	assert.Equal(t, "https://api.lucc.dev/v1", cfg.BaseURL, "default base URL")
	assert.Equal(t, "deepseek-v4-flash", cfg.Model, "default model")
	assert.True(t, cfg.Streaming, "streaming on by default to bypass CF 524")
}

func TestDefaultConfig_StreamingTrue(t *testing.T) {
	cfg := DefaultConfig()
	assert.True(t, cfg.Streaming)
	assert.True(t, ConfigWithOverrides("k", "https://x/v1", "m").Streaming)
}

func TestDefaultConfig_WithEnv(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test-key")
	t.Setenv("OPENAI_BASE_URL", "https://custom.api.com/v1")
	t.Setenv("LLM_AxonHub", "")
	t.Setenv("LLM_MODEL", "deepseek-v4-flash")

	cfg := DefaultConfig()
	assert.Equal(t, "sk-test-key", cfg.APIKey)
	assert.Equal(t, "https://custom.api.com/v1", cfg.BaseURL)
	assert.Equal(t, "deepseek-v4-flash", cfg.Model)
}

func TestDefaultConfig_LLMAxonHubFallback(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("LLM_AxonHub", "sk-axon-fallback-key")

	cfg := DefaultConfig()
	assert.Equal(t, "sk-axon-fallback-key", cfg.APIKey, "LLM_AxonHub fallback key")
}

func TestChat_NoAPIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("LLM_AxonHub", "")

	cfg := DefaultConfig()
	_, err := Chat(cfg, []Message{{Role: RoleUser, Content: "hello"}})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "OPENAI_API_KEY not set")
}

func TestChat_Content(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer sk-test", r.Header.Get("Authorization"))
		var req ChatRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "test-model", req.Model)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"hello"}}]}`))
	}))
	defer server.Close()

	got, err := Chat(&ClientConfig{
		APIKey:  "sk-test",
		BaseURL: server.URL,
		Model:   "test-model",
		Timeout: time.Second,
	}, []Message{{Role: RoleUser, Content: "hello"}})

	require.NoError(t, err)
	assert.Equal(t, "hello", got)
}

func TestChat_ReasoningContentNotFalledBack(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"reasoning_content":"reasoning"}}]}`))
	}))
	defer server.Close()

	_, err := Chat(&ClientConfig{
		APIKey:  "sk-test",
		BaseURL: server.URL,
		Model:   "test-model",
		Timeout: time.Second,
	}, []Message{{Role: RoleUser, Content: "hello"}})

	// reasoning_content alone is not the response — it's chain-of-thought.
	require.Error(t, err)
}

func TestChat_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"rate limited"}}`))
	}))
	defer server.Close()

	_, err := Chat(&ClientConfig{
		APIKey:  "sk-test",
		BaseURL: server.URL,
		Model:   "test-model",
		Timeout: time.Second,
	}, []Message{{Role: RoleUser, Content: "hello"}})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "API returned unexpected status code: 429")
	assert.Contains(t, err.Error(), "rate limited")
}

func TestChat_EmptyChoicesPreservesError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer server.Close()

	_, err := Chat(&ClientConfig{
		APIKey:  "sk-test",
		BaseURL: server.URL,
		Model:   "test-model",
		Timeout: time.Second,
	}, []Message{{Role: RoleUser, Content: "hello"}})

	require.Error(t, err)
	assert.Equal(t, "no choices returned", err.Error())
}

func TestMessage_Struct(t *testing.T) {
	m := Message{Role: RoleUser, Content: "test message"}
	assert.Equal(t, RoleUser, m.Role)
	assert.Equal(t, "test message", m.Content)
}

func TestChatRequest_Struct(t *testing.T) {
	r := ChatRequest{
		Model: "test-model",
		Messages: []Message{
			{Role: RoleUser, Content: "hello"},
		},
	}
	assert.Equal(t, "test-model", r.Model)
	assert.Len(t, r.Messages, 1)
}

func TestChatResponse_Struct(t *testing.T) {
	r := ChatResponse{
		Choices: []struct {
			Message struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content,omitempty"`
			} `json:"message"`
		}{
			{
				Message: struct {
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content,omitempty"`
				}{
					Content:          "response",
					ReasoningContent: "reasoning",
				},
			},
		},
	}
	assert.Len(t, r.Choices, 1)
	assert.Equal(t, "response", r.Choices[0].Message.Content)
	assert.Equal(t, "reasoning", r.Choices[0].Message.ReasoningContent)
}

func TestChatResponse_EmptyChoices(t *testing.T) {
	r := ChatResponse{}
	assert.Empty(t, r.Choices)
}

func TestChatResponse_Error(t *testing.T) {
	r := ChatResponse{
		Error: &struct {
			Message string `json:"message"`
		}{Message: "rate limit exceeded"},
	}
	require.NotNil(t, r.Error)
	assert.Equal(t, "rate limit exceeded", r.Error.Message)
}

func TestConfigWithOverrides_AllSet(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("LLM_AxonHub", "")

	cfg := ConfigWithOverrides("sk-override", "https://custom.api.com/v1", "custom-model")
	assert.Equal(t, "sk-override", cfg.APIKey)
	assert.Equal(t, "https://custom.api.com/v1", cfg.BaseURL)
	assert.Equal(t, "custom-model", cfg.Model)
}

func TestConfigWithOverrides_EmptyOverridesUseDefaults(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-env")
	t.Setenv("OPENAI_BASE_URL", "")
	t.Setenv("LLM_AxonHub", "")
	t.Setenv("LLM_MODEL", "")

	cfg := ConfigWithOverrides("", "", "")
	assert.Equal(t, "sk-env", cfg.APIKey)
	assert.Equal(t, "https://api.lucc.dev/v1", cfg.BaseURL)
	assert.Equal(t, "deepseek-v4-flash", cfg.Model)
}

func TestConfigWithOverrides_PartialOverrides(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("LLM_AxonHub", "")
	t.Setenv("OPENAI_BASE_URL", "")
	t.Setenv("LLM_MODEL", "")

	cfg := ConfigWithOverrides("sk-partial", "", "new-model")
	assert.Equal(t, "sk-partial", cfg.APIKey)
	assert.Equal(t, "https://api.lucc.dev/v1", cfg.BaseURL)
	assert.Equal(t, "new-model", cfg.Model)
}

func TestToLLMSRole_System(t *testing.T) {
	assert.Equal(t, llms.ChatMessageTypeSystem, toLLMSRole(RoleSystem))
}

func TestToLLMSRole_Assistant(t *testing.T) {
	assert.Equal(t, llms.ChatMessageTypeAI, toLLMSRole(RoleAssistant))
}

func TestToLLMSRole_User(t *testing.T) {
	assert.Equal(t, llms.ChatMessageTypeHuman, toLLMSRole(RoleUser))
}

func TestToLLMSRole_Unknown(t *testing.T) {
	assert.Equal(t, llms.ChatMessageTypeGeneric, toLLMSRole("unknown"))
}

func TestExtractContentAndValidate_NilResponse(t *testing.T) {
	_, err := extractContentAndValidate(nil)
	require.Error(t, err)
	assert.Equal(t, "no choices returned", err.Error())
}

func TestExtractContentAndValidate_EmptyChoices(t *testing.T) {
	_, err := extractContentAndValidate(&llms.ContentResponse{Choices: []*llms.ContentChoice{}})
	require.Error(t, err)
	assert.Equal(t, "no choices returned", err.Error())
}

func TestExtractContentAndValidate_EmptyContent(t *testing.T) {
	_, err := extractContentAndValidate(&llms.ContentResponse{
		Choices: []*llms.ContentChoice{{Content: "", ReasoningContent: ""}},
	})
	require.Error(t, err)
	assert.Equal(t, "empty response from AI model", err.Error())
}

func TestExtractContentAndValidate_ValidContent(t *testing.T) {
	content, err := extractContentAndValidate(&llms.ContentResponse{
		Choices: []*llms.ContentChoice{{Content: "hello"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "hello", content)
}

func TestExtractContentAndValidate_ReasoningOnly(t *testing.T) {
	_, err := extractContentAndValidate(&llms.ContentResponse{
		Choices: []*llms.ContentChoice{{Content: "", ReasoningContent: "thinking"}},
	})
	// reasoning_content is not the response — return empty error.
	require.Error(t, err)
}

func TestExtractChoiceContent_ContentPreferred(t *testing.T) {
	assert.Equal(t, "content", extractChoiceContent(&llms.ContentChoice{
		Content:          "content",
		ReasoningContent: "reasoning",
	}))
}

func TestExtractChoiceContent_Empty(t *testing.T) {
	assert.Empty(t, extractChoiceContent(&llms.ContentChoice{}))
}

func TestChat_WithTemperature(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"temp result"}}]}`))
	}))
	defer server.Close()

	got, err := Chat(&ClientConfig{
		APIKey:      "sk-test",
		BaseURL:     server.URL,
		Model:       "test-model",
		Timeout:     time.Second,
		Temperature: 0.7,
	}, []Message{{Role: RoleUser, Content: "hello"}})

	require.NoError(t, err)
	assert.Equal(t, "temp result", got)
}

func TestChat_DefaultTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer server.Close()

	// Zero timeout should use default (not fail)
	got, err := Chat(&ClientConfig{
		APIKey:  "sk-test",
		BaseURL: server.URL,
		Model:   "test-model",
		Timeout: 0, // should default to DefaultAITimeout
	}, []Message{{Role: RoleUser, Content: "hello"}})

	require.NoError(t, err)
	assert.Equal(t, "ok", got)
}

func TestChat_SystemMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Len(t, req.Messages, 2)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"response"}}]}`))
	}))
	defer server.Close()

	got, err := Chat(&ClientConfig{
		APIKey:  "sk-test",
		BaseURL: server.URL,
		Model:   "test-model",
		Timeout: time.Second,
	}, []Message{
		{Role: RoleSystem, Content: "You are helpful"},
		{Role: RoleUser, Content: "hello"},
	})

	require.NoError(t, err)
	assert.Equal(t, "response", got)
}

func TestChat_EmptyBothContentAndReasoning(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{}}]}`))
	}))
	defer server.Close()

	_, err := Chat(&ClientConfig{
		APIKey:  "sk-test",
		BaseURL: server.URL,
		Model:   "test-model",
		Timeout: time.Second,
	}, []Message{{Role: RoleUser, Content: "hello"}})

	require.Error(t, err)
	assert.Equal(t, "empty response from AI model", err.Error())
}

// writeSSE writes a minimal OpenAI-compatible chat completion stream.
func writeSSE(w http.ResponseWriter, deltas ...map[string]any) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	flusher, _ := w.(http.Flusher)
	for _, delta := range deltas {
		payload, _ := json.Marshal(map[string]any{
			"choices": []map[string]any{{
				"index":         0,
				"delta":         delta,
				"finish_reason": nil,
			}},
		})
		_, _ = w.Write([]byte("data: "))
		_, _ = w.Write(payload)
		_, _ = w.Write([]byte("\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
	}
	finish, _ := json.Marshal(map[string]any{
		"choices": []map[string]any{{
			"index":         0,
			"delta":         map[string]any{},
			"finish_reason": "stop",
		}},
	})
	_, _ = w.Write([]byte("data: "))
	_, _ = w.Write(finish)
	_, _ = w.Write([]byte("\n\ndata: [DONE]\n\n"))
	if flusher != nil {
		flusher.Flush()
	}
}

func TestChat_Streaming_SendsStreamTrue(t *testing.T) {
	var sawStream bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		if s, ok := body["stream"].(bool); ok && s {
			sawStream = true
		}
		writeSSE(w, map[string]any{"content": "streamed"})
	}))
	defer server.Close()

	got, err := Chat(&ClientConfig{
		APIKey:    "sk-test",
		BaseURL:   server.URL,
		Model:     "test-model",
		Timeout:   time.Second,
		Streaming: true,
	}, []Message{{Role: RoleUser, Content: "hello"}})

	require.NoError(t, err)
	assert.True(t, sawStream, "request must set stream=true")
	assert.Equal(t, "streamed", got)
}

func TestChat_Streaming_ReasoningThenContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeSSE(w,
			map[string]any{"reasoning_content": "thinking hard"},
			map[string]any{"content": `{"ok":true}`},
		)
	}))
	defer server.Close()

	got, err := Chat(&ClientConfig{
		APIKey:    "sk-test",
		BaseURL:   server.URL,
		Model:     "test-model",
		Timeout:   time.Second,
		Streaming: true,
	}, []Message{{Role: RoleUser, Content: "hello"}})

	require.NoError(t, err)
	assert.Equal(t, `{"ok":true}`, got, "only content is returned; reasoning is ignored")
}

func TestChatContext_NoDeadline_GetsDefaultTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(300 * time.Millisecond)
		writeSSE(w, map[string]any{"content": "late"})
	}))
	defer server.Close()

	// Background ctx has no deadline; ensureDeadline must apply cfg.Timeout.
	_, err := ChatContext(context.Background(), &ClientConfig{
		APIKey:    "sk-test",
		BaseURL:   server.URL,
		Model:     "test-model",
		Timeout:   50 * time.Millisecond,
		Streaming: true,
	}, []Message{{Role: RoleUser, Content: "hello"}})

	require.Error(t, err, "short timeout must cancel hung streaming call")
}

func TestEnsureDeadline_PreservesExistingDeadline(t *testing.T) {
	parent, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	got, gotCancel := ensureDeadline(parent, 50*time.Millisecond)
	defer gotCancel()

	parentDL, ok := parent.Deadline()
	require.True(t, ok)
	gotDL, ok := got.Deadline()
	require.True(t, ok)
	assert.Equal(t, parentDL, gotDL, "must not shrink an existing deadline")
}
