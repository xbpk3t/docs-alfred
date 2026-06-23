package cmd

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/pkg/ai"
	"github.com/xbpk3t/docs-alfred/pkg/configutil"
)

func TestBuildAIConfig_NilConfig(t *testing.T) {
	result := buildAIConfig(nil)

	require.NotNil(t, result, "should return non-nil config even for nil input")
	// With nil config, defaults from ai.DefaultConfig are used.
	defaults := ai.DefaultConfig()
	require.Equal(t, defaults.BaseURL, result.BaseURL)
	require.Equal(t, defaults.Model, result.Model)
	// APIKey depends on env; just verify it matches defaults.
	require.Equal(t, defaults.APIKey, result.APIKey)
}

func TestBuildAIConfig_PartialConfig(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("LLM_AxonHub", "")
	t.Setenv("OPENAI_BASE_URL", "")
	t.Setenv("LLM_MODEL", "")

	cfg := &exportConfig{
		AI: exportAIConfig{
			BaseURL: "https://custom.example/v1",
		},
	}

	result := buildAIConfig(cfg)

	require.NotNil(t, result)
	require.Equal(t, "https://custom.example/v1", result.BaseURL)
	require.Empty(t, result.APIKey, "APIKey should be empty when not set in config")
	// Model should fall through to default since empty in config.
	require.Equal(t, "deepseek-v4-flash", result.Model)
}

func TestBuildAIConfig_FullConfig(t *testing.T) {
	cfg := &exportConfig{
		AI: exportAIConfig{
			APIKey:  "sk-test-key",
			BaseURL: "https://full.example/v1",
			Model:   "gpt-4o",
		},
	}

	result := buildAIConfig(cfg)

	require.NotNil(t, result)
	require.Equal(t, "sk-test-key", result.APIKey)
	require.Equal(t, "https://full.example/v1", result.BaseURL)
	require.Equal(t, "gpt-4o", result.Model)
}

func TestBuildAIConfig_EmptyAIConfig(t *testing.T) {
	cfg := &exportConfig{}

	result := buildAIConfig(cfg)

	require.NotNil(t, result)
	defaults := ai.DefaultConfig()
	require.Equal(t, defaults.APIKey, result.APIKey)
	require.Equal(t, defaults.BaseURL, result.BaseURL)
	require.Equal(t, defaults.Model, result.Model)
}

func TestDefaultExportConfig(t *testing.T) {
	cfg := defaultExportConfig()

	require.Equal(t, "wiki", cfg.WikiRoot, "default WikiRoot should be 'wiki'")
	require.Equal(t, "https://api.lucc.dev/v1", cfg.AI.BaseURL, "default AI BaseURL")
	require.Equal(t, "deepseek-v4-flash", cfg.AI.Model, "default AI Model")
	require.Empty(t, cfg.AI.APIKey, "default APIKey should be empty")
}

func TestFormatExportConfigError_ReadStage(t *testing.T) {
	loadErr := &configutil.LoadError{
		Stage: configutil.StageRead,
		Err:   errors.New("file not found"),
	}

	result := formatExportConfigError(loadErr)

	require.Error(t, result)
	require.Contains(t, result.Error(), "read config")
	require.Contains(t, result.Error(), "file not found")
	// Verify the original error is wrapped.
	require.True(t, errors.Is(result, loadErr.Err))
}

func TestFormatExportConfigError_ParseStage(t *testing.T) {
	loadErr := &configutil.LoadError{
		Stage: configutil.StageParse,
		Err:   errors.New("invalid YAML syntax"),
	}

	result := formatExportConfigError(loadErr)

	require.Error(t, result)
	require.Contains(t, result.Error(), "parse config")
	require.Contains(t, result.Error(), "invalid YAML syntax")
	require.True(t, errors.Is(result, loadErr.Err))
}

func TestFormatExportConfigError_UnmarshalStage(t *testing.T) {
	loadErr := &configutil.LoadError{
		Stage: configutil.StageUnmarshal,
		Err:   errors.New("type mismatch"),
	}

	result := formatExportConfigError(loadErr)

	require.Error(t, result)
	require.Contains(t, result.Error(), "unmarshal config")
	require.Contains(t, result.Error(), "type mismatch")
	require.True(t, errors.Is(result, loadErr.Err))
}

func TestFormatExportConfigError_ValidateStage(t *testing.T) {
	loadErr := &configutil.LoadError{
		Stage: configutil.StageValidate,
		Err:   errors.New("validation failed"),
	}

	result := formatExportConfigError(loadErr)

	// Validate stage falls through to default case, returning original error.
	require.Error(t, result)
	require.Equal(t, loadErr, result)
}

func TestFormatExportConfigError_UnknownStage(t *testing.T) {
	loadErr := &configutil.LoadError{
		Stage: "unknown",
		Err:   errors.New("something happened"),
	}

	result := formatExportConfigError(loadErr)

	// Unknown stage falls through to default case, returning original error.
	require.Error(t, result)
	require.Equal(t, loadErr, result)
}

func TestFormatExportConfigError_NonLoadError(t *testing.T) {
	plainErr := errors.New("plain error, not a LoadError")

	result := formatExportConfigError(plainErr)

	require.Error(t, result)
	require.Equal(t, plainErr, result, "non-LoadError should be returned as-is")
}

func TestFormatExportConfigError_NilLoadError(t *testing.T) {
	// A *LoadError with nil Err edge case.
	loadErr := &configutil.LoadError{
		Stage: configutil.StageRead,
		Err:   nil,
	}

	result := formatExportConfigError(loadErr)

	require.Error(t, result)
	require.Contains(t, result.Error(), "read config")
}

func TestLoadExportConfig_InvalidYAML(t *testing.T) {
	path := writeCCXConfig(t, "ai:\n  - this is not valid\n  yaml: [unclosed")

	_, err := loadExportConfig(path, exportConfigOverrides{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "parse config")
}

func TestLoadExportConfig_NonexistentFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.yml")

	_, err := loadExportConfig(path, exportConfigOverrides{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "read config")
}

func TestLoadExportConfig_EmptyPathUsesDefaults(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("CCX_WIKI_ROOT", "")
	t.Setenv("CCX_AI_BASE_URL", "")
	t.Setenv("CCX_AI_MODEL", "")

	cfg, err := loadExportConfig("", exportConfigOverrides{})
	require.NoError(t, err)
	require.Equal(t, "wiki", cfg.WikiRoot)
	require.Equal(t, "https://api.lucc.dev/v1", cfg.AI.BaseURL)
	require.Equal(t, "deepseek-v4-flash", cfg.AI.Model)
}

func TestLoadExportConfig_OverridePartial(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("CCX_WIKI_ROOT", "")
	t.Setenv("CCX_AI_BASE_URL", "")
	t.Setenv("CCX_AI_MODEL", "")

	path := writeCCXConfig(t, "wikiRoot: from-yaml\nai:\n  model: yaml-model\n")

	cfg, err := loadExportConfig(path, exportConfigOverrides{
		WikiRoot: "override-wiki",
	})
	require.NoError(t, err)
	require.Equal(t, "override-wiki", cfg.WikiRoot, "flag override should take precedence")
	require.Equal(t, "yaml-model", cfg.AI.Model, "YAML value should be preserved for non-overridden fields")
}
