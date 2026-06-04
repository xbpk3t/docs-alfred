package ai

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig_Defaults(t *testing.T) {
	// Save env and restore
	oldKey := os.Getenv("OPENAI_API_KEY")
	oldBase := os.Getenv("OPENAI_BASE_URL")
	oldAxon := os.Getenv("LLM_AxonHub")
	oldModel := os.Getenv("LLM_MODEL")
	defer func() {
		os.Setenv("OPENAI_API_KEY", oldKey)
		os.Setenv("OPENAI_BASE_URL", oldBase)
		os.Setenv("LLM_AxonHub", oldAxon)
		os.Setenv("LLM_MODEL", oldModel)
	}()

	os.Unsetenv("OPENAI_API_KEY")
	os.Unsetenv("OPENAI_BASE_URL")
	os.Unsetenv("LLM_AxonHub")
	os.Unsetenv("LLM_MODEL")

	cfg := DefaultConfig()
	assert.NotNil(t, cfg)
	assert.Equal(t, "", cfg.APIKey, "no env var → empty key")
	assert.Equal(t, "https://api.lucc.dev/v1", cfg.BaseURL, "default base URL")
	assert.Equal(t, "deepseek-v4-flash", cfg.Model, "default model")
}

func TestDefaultConfig_WithEnv(t *testing.T) {
	oldKey := os.Getenv("OPENAI_API_KEY")
	oldBase := os.Getenv("OPENAI_BASE_URL")
	oldAxon := os.Getenv("LLM_AxonHub")
	oldModel := os.Getenv("LLM_MODEL")
	defer func() {
		os.Setenv("OPENAI_API_KEY", oldKey)
		os.Setenv("OPENAI_BASE_URL", oldBase)
		os.Setenv("LLM_AxonHub", oldAxon)
		os.Setenv("LLM_MODEL", oldModel)
	}()

	os.Setenv("OPENAI_API_KEY", "sk-test-key")
	os.Setenv("OPENAI_BASE_URL", "https://custom.api.com/v1")
	os.Setenv("LLM_MODEL", "deepseek-v4-flash")

	cfg := DefaultConfig()
	assert.Equal(t, "sk-test-key", cfg.APIKey)
	assert.Equal(t, "https://custom.api.com/v1", cfg.BaseURL)
	assert.Equal(t, "deepseek-v4-flash", cfg.Model)
}

func TestDefaultConfig_LLMAxonHubFallback(t *testing.T) {
	oldKey := os.Getenv("OPENAI_API_KEY")
	oldAxon := os.Getenv("LLM_AxonHub")
	defer func() {
		os.Setenv("OPENAI_API_KEY", oldKey)
		os.Setenv("LLM_AxonHub", oldAxon)
	}()

	os.Unsetenv("OPENAI_API_KEY")
	os.Setenv("LLM_AxonHub", "sk-axon-fallback-key")

	cfg := DefaultConfig()
	assert.Equal(t, "sk-axon-fallback-key", cfg.APIKey, "LLM_AxonHub fallback key")
}

func TestChat_NoAPIKey(t *testing.T) {
	oldKey := os.Getenv("OPENAI_API_KEY")
	oldAxon := os.Getenv("LLM_AxonHub")
	defer func() {
		os.Setenv("OPENAI_API_KEY", oldKey)
		os.Setenv("LLM_AxonHub", oldAxon)
	}()
	os.Unsetenv("OPENAI_API_KEY")
	os.Unsetenv("LLM_AxonHub")

	cfg := DefaultConfig()
	_, err := Chat(cfg, []Message{{Role: "user", Content: "hello"}})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "OPENAI_API_KEY not set")
}

func TestMessage_Struct(t *testing.T) {
	m := Message{Role: "user", Content: "test message"}
	assert.Equal(t, "user", m.Role)
	assert.Equal(t, "test message", m.Content)
}

func TestChatRequest_Struct(t *testing.T) {
	r := ChatRequest{
		Model: "test-model",
		Messages: []Message{
			{Role: "user", Content: "hello"},
		},
	}
	assert.Equal(t, "test-model", r.Model)
	assert.Equal(t, 1, len(r.Messages))
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
	assert.Equal(t, 1, len(r.Choices))
	assert.Equal(t, "response", r.Choices[0].Message.Content)
	assert.Equal(t, "reasoning", r.Choices[0].Message.ReasoningContent)
}

func TestChatResponse_EmptyChoices(t *testing.T) {
	r := ChatResponse{}
	assert.Equal(t, 0, len(r.Choices))
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
