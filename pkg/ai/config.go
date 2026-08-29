package ai

import (
	"context"
	"os"
)

// DefaultConfig creates a client config from environment variables, decoupled
// from any single CLI. Honors the same env conventions as the rest of the repo
// (OPENAI_API_KEY / LLM_AxonHub fallback / OPENAI_BASE_URL / LLM_MODEL). Effort
// defaults to DefaultEffort ("max") unless LLM_EFFORT is set — matching the
// plan: reasoning depth is on by default for every MAF-backed call.
func DefaultConfig() *ClientConfig {
	cfg := &ClientConfig{
		APIKey:  os.Getenv("OPENAI_API_KEY"),
		BaseURL: os.Getenv("OPENAI_BASE_URL"),
		Model:   os.Getenv("LLM_MODEL"),
		Effort:  os.Getenv("LLM_EFFORT"),
		// Streaming is ignored on the MAF path but kept true so legacy comments/
		// tests (and any config row) match the previous default contracts.
		Streaming: true,
	}
	if cfg.APIKey == "" {
		cfg.APIKey = os.Getenv("LLM_AxonHub")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	if cfg.Model == "" {
		cfg.Model = "deepseek-v4-flash"
	}
	return cfg
}

// ConfigWithOverrides creates a config with explicit overrides; env vars still
// take precedence over provided values (see DefaultConfig).
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

// Chat is a context-free convenience wrapper over ChatContext.
func Chat(cfg *ClientConfig, messages []Message) (string, error) {
	return ChatContext(context.Background(), cfg, messages)
}
