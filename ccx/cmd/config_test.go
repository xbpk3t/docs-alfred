package cmd

import (
	"os"
	"path/filepath"
	"testing"

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
	if err != nil {
		t.Fatalf("loadExportConfig() error = %v", err)
	}
	if got.WikiRoot != "flag-wiki" {
		t.Fatalf("WikiRoot = %q, want flag override", got.WikiRoot)
	}
	if got.AI.APIKey != "env-key" {
		t.Fatalf("APIKey = %q, want env value", got.AI.APIKey)
	}
	if got.AI.BaseURL != "https://flag.example/v1" {
		t.Fatalf("BaseURL = %q, want flag override", got.AI.BaseURL)
	}
	if got.AI.Model != "flag-model" {
		t.Fatalf("Model = %q, want flag override", got.AI.Model)
	}
}

func TestLoadExportConfigWithoutFileUsesEnvDefaults(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "env-key")
	t.Setenv("CCX_AI_BASE_URL", "https://env.example/v1")
	t.Setenv("CCX_AI_MODEL", "env-model")
	t.Setenv("CCX_WIKI_ROOT", "env-wiki")

	got, err := loadExportConfig("", exportConfigOverrides{})
	if err != nil {
		t.Fatalf("loadExportConfig() error = %v", err)
	}
	if got.WikiRoot != "env-wiki" {
		t.Fatalf("WikiRoot = %q, want env value", got.WikiRoot)
	}
	if got.AI.APIKey != "env-key" || got.AI.BaseURL != "https://env.example/v1" || got.AI.Model != "env-model" {
		t.Fatalf("AI config = %+v, want env values", got.AI)
	}
}

func writeCCXConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ccx.yml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	return path
}
