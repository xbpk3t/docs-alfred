package ai

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

// DefaultAITimeout is the default HTTP timeout for AI chat requests.
const DefaultAITimeout = 3 * time.Minute

// Role constants for chat messages.
const (
	RoleUser      = "user"
	RoleSystem    = "system"
	RoleAssistant = "assistant"
)

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

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = DefaultAITimeout
	}

	model, err := openai.New(
		openai.WithToken(cfg.APIKey),
		openai.WithBaseURL(cfg.BaseURL),
		openai.WithModel(cfg.Model),
		openai.WithHTTPClient(&http.Client{Timeout: timeout}),
	)
	if err != nil {
		return "", fmt.Errorf("create AI client: %w", err)
	}

	callOptions := []llms.CallOption{}
	if cfg.Model != "" {
		callOptions = append(callOptions, llms.WithModel(cfg.Model))
	}

	resp, err := model.GenerateContent(ctx, toLLMSMessages(messages), callOptions...)
	if err != nil {
		if isEmptyResponseError(err) {
			return "", errors.New("no choices returned")
		}

		return "", fmt.Errorf("generate content: %w", err)
	}
	if resp == nil || len(resp.Choices) == 0 {
		return "", errors.New("no choices returned")
	}

	content := extractChoiceContent(resp.Choices[0])
	if content == "" {
		return "", errors.New("empty response from AI model")
	}

	return content, nil
}

func extractChoiceContent(choice *llms.ContentChoice) string {
	if choice.Content != "" {
		return choice.Content
	}
	if choice.ReasoningContent != "" {
		return choice.ReasoningContent
	}

	return ""
}

func isEmptyResponseError(err error) bool {
	return errors.Is(err, openai.ErrEmptyResponse) || err.Error() == "empty response"
}

func toLLMSMessages(messages []Message) []llms.MessageContent {
	converted := make([]llms.MessageContent, 0, len(messages))
	for _, msg := range messages {
		converted = append(converted, llms.TextParts(toLLMSRole(msg.Role), msg.Content))
	}

	return converted
}

func toLLMSRole(role string) llms.ChatMessageType {
	switch role {
	case RoleSystem:
		return llms.ChatMessageTypeSystem
	case RoleAssistant:
		return llms.ChatMessageTypeAI
	case RoleUser:
		return llms.ChatMessageTypeHuman
	default:
		return llms.ChatMessageTypeGeneric
	}
}
