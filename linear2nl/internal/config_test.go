package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigEnvOverrides(t *testing.T) {
	configPath := writeTestConfig(t, `
theme: dark
linear:
  apiKey: yaml-linear-key
  teamKeys: []
morning:
  strategy: all_assigned
ai:
  model: yaml-model
  language: zh
resend:
  token: yaml-resend-token
  mailTo: [me@example.com]
`)

	t.Setenv("LINEAR_API_KEY", "env-linear-key")
	t.Setenv("RESEND_TOKEN", "env-resend-token")
	t.Setenv("LINEAR2NL_MORNING_STRATEGY", "focused")
	t.Setenv("LINEAR2NL_AI_MODEL", "env-model")

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.Linear.APIKey != "env-linear-key" {
		t.Fatalf("Linear.APIKey = %q, want env override", cfg.Linear.APIKey)
	}
	if cfg.Resend.Token != "env-resend-token" {
		t.Fatalf("Resend.Token = %q, want env override", cfg.Resend.Token)
	}
	if cfg.Morning.Strategy != "focused" {
		t.Fatalf("Morning.Strategy = %q, want env override", cfg.Morning.Strategy)
	}
	if cfg.AI.Model != "env-model" {
		t.Fatalf("AI.Model = %q, want env override", cfg.AI.Model)
	}
}

func TestLoadConfigEmptyEnvDoesNotOverride(t *testing.T) {
	configPath := writeTestConfig(t, `
theme: dark
linear:
  apiKey: yaml-linear-key
  teamKeys: []
morning:
  strategy: all_assigned
ai:
  model: yaml-model
  language: zh
resend:
  token: yaml-resend-token
  mailTo: [me@example.com]
`)

	t.Setenv("LINEAR_API_KEY", "")
	t.Setenv("RESEND_TOKEN", "")

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.Linear.APIKey != "yaml-linear-key" {
		t.Fatalf("Linear.APIKey = %q, want YAML value", cfg.Linear.APIKey)
	}
	if cfg.Resend.Token != "yaml-resend-token" {
		t.Fatalf("Resend.Token = %q, want YAML value", cfg.Resend.Token)
	}
}

func writeTestConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "linear2nl.yml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	return path
}
