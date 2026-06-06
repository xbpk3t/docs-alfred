package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/xbpk3t/docs-alfred/pkg/httputil"
)

// DefaultAITimeout is the default HTTP timeout for AI chat requests.
const DefaultAITimeout = 3 * time.Minute

// ClientConfig holds the AI client configuration.
type ClientConfig struct {
	APIKey  string
	BaseURL string
	Model   string
	Timeout time.Duration // HTTP client timeout; 0 uses default 3 min
}

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest is the request body for chat completions.
type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

// ChatResponse is the response body from chat completions.
type ChatResponse struct {
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
	Choices []struct {
		Message struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content,omitempty"`
		} `json:"message"`
	} `json:"choices"`
}

// DefaultConfig creates a client config from environment variables.
// LLM_AxonHub is a fallback API key (same as TS behavior), NOT a model name.
// Model and BaseURL can be overridden via ConfigOpts or YAML config.
func DefaultConfig() *ClientConfig {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("LLM_AxonHub")
	}

	cfg := &ClientConfig{
		APIKey:  apiKey,
		BaseURL: os.Getenv("OPENAI_BASE_URL"),
		Model:   os.Getenv("LLM_MODEL"),
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.lucc.dev/v1"
	}
	if cfg.Model == "" {
		cfg.Model = "deepseek-v4-flash"
	}

	return cfg
}

// ConfigWithOverrides creates a client config with explicit overrides.
// Env vars still take precedence over provided values.
func ConfigWithOverrides(apiKey, baseURL, model string) *ClientConfig {
	cfg := DefaultConfig()
	if apiKey != "" {
		cfg.APIKey = apiKey
	}
	if baseURL != "" {
		cfg.BaseURL = baseURL
	}
	if model != "" {
		cfg.Model = model
	}

	return cfg
}

// Chat sends a chat completion request and returns the response content.
// Handles DeepSeek's non-standard `reasoning_content` field.
func Chat(cfg *ClientConfig, messages []Message) (string, error) {
	return ChatContext(context.Background(), cfg, messages)
}

// ChatContext sends a chat completion request with caller-controlled context.
// Handles DeepSeek's non-standard `reasoning_content` field.
func ChatContext(ctx context.Context, cfg *ClientConfig, messages []Message) (string, error) {
	if cfg.APIKey == "" {
		return "", errors.New("OPENAI_API_KEY not set")
	}

	requestBody := ChatRequest{
		Model:    cfg.Model,
		Messages: messages,
	}

	url := strings.TrimRight(cfg.BaseURL, "/") + "/chat/completions"

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = DefaultAITimeout
	}

	respBody, err := httputil.PostJSONWithResult(ctx, url, requestBody, nil, httputil.RequestOptions{
		Timeout:    timeout,
		MaxRetries: 1,
		Headers: map[string]string{
			"Authorization": "Bearer " + cfg.APIKey,
		},
	})
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}

	return parseChatResponseBody(respBody)
}

func parseChatResponseBody(respBody []byte) (string, error) {
	var resp ChatResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if resp.Error != nil && resp.Error.Message != "" {
		return "", fmt.Errorf("API error: %s", resp.Error.Message)
	}

	if len(resp.Choices) == 0 {
		return "", errors.New("no choices returned")
	}

	msg := resp.Choices[0].Message

	// Priority: content > reasoning_content > error
	if msg.Content != "" {
		return msg.Content, nil
	}
	if msg.ReasoningContent != "" {
		return msg.ReasoningContent, nil
	}

	return "", errors.New("empty response from AI model")
}
