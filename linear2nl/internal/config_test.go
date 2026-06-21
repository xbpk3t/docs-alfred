package internal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
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
	require.NoError(t, err)
	require.Equal(t, "env-linear-key", cfg.Linear.APIKey)
	require.Equal(t, "env-resend-token", cfg.Resend.Token)
	require.Equal(t, "focused", cfg.Morning.Strategy)
	require.Equal(t, "env-model", cfg.AI.Model)
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
	require.NoError(t, err)
	require.Equal(t, "yaml-linear-key", cfg.Linear.APIKey)
	require.Equal(t, "yaml-resend-token", cfg.Resend.Token)
}

func writeTestConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "linear2nl.yml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))

	return path
}
