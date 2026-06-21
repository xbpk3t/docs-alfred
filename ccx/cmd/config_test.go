package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/pkg/ai"
)

func TestLoadExportConfigPrefersFlagsThenEnvThenYAML(t *testing.T) {
	path := writeCCXConfig(t, "wikiRoot: yaml-wiki\nai:\n  baseUrl: https://yaml.example/v1\n  model: yaml-model\n")
	t.Setenv("OPENAI_API_KEY", "env-key")
	t.Setenv("CCX_AI_BASE_URL", "https://env.example/v1")
	t.Setenv("CCX_AI_MODEL", "env-model")
	t.Setenv("CCX_WIKI_ROOT", "env-wiki")

	got, err := loadExportConfig(path, exportConfigOverrides{
		WikiRoot: "flag-wiki",
		AI: &ai.ClientConfig{
			BaseURL: "https://flag.example/v1",
			Model:   "flag-model",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "flag-wiki", got.WikiRoot, "WikiRoot should use flag override")
	require.Equal(t, "env-key", got.AI.APIKey, "APIKey should use env value")
	require.Equal(t, "https://flag.example/v1", got.AI.BaseURL, "BaseURL should use flag override")
	require.Equal(t, "flag-model", got.AI.Model, "Model should use flag override")
}

func TestLoadExportConfigWithoutFileUsesEnvDefaults(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "env-key")
	t.Setenv("CCX_AI_BASE_URL", "https://env.example/v1")
	t.Setenv("CCX_AI_MODEL", "env-model")
	t.Setenv("CCX_WIKI_ROOT", "env-wiki")

	got, err := loadExportConfig("", exportConfigOverrides{})
	require.NoError(t, err)
	require.Equal(t, "env-wiki", got.WikiRoot, "WikiRoot should use env value")
	require.Equal(t, "env-key", got.AI.APIKey)
	require.Equal(t, "https://env.example/v1", got.AI.BaseURL)
	require.Equal(t, "env-model", got.AI.Model)
}

func writeCCXConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ccx.yml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	return path
}
