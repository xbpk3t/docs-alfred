package ai

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Package-level HTTP client for reuse across requests.
var httpClient = &http.Client{Timeout: 60 * time.Second}

// ClientConfig holds the AI client configuration.
type ClientConfig struct {
	APIKey  string
	BaseURL string
	Model   string
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
	if cfg.APIKey == "" {
		return "", errors.New("OPENAI_API_KEY not set")
	}

	req := ChatRequest{
		Model:    cfg.Model,
		Messages: messages,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := strings.TrimRight(cfg.BaseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	httpResp, err := httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	return parseChatResponse(httpResp)
}

func parseChatResponse(httpResp *http.Response) (string, error) {
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error (HTTP %d): %s", httpResp.StatusCode, string(respBody))
	}

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
