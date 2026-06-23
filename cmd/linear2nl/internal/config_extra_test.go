package internal

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/pkg/configutil"
)

func TestFormatConfigLoadErrorRead(t *testing.T) {
	err := formatConfigLoadError(&configutil.LoadError{
		Stage: configutil.StageRead,
		Err:   errors.New("file not found"),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read config")
	assert.Contains(t, err.Error(), "file not found")
}

func TestFormatConfigLoadErrorParse(t *testing.T) {
	err := formatConfigLoadError(&configutil.LoadError{
		Stage: configutil.StageParse,
		Err:   errors.New("bad yaml"),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse config")
}

func TestFormatConfigLoadErrorUnmarshal(t *testing.T) {
	err := formatConfigLoadError(&configutil.LoadError{
		Stage: configutil.StageUnmarshal,
		Err:   errors.New("type mismatch"),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal config")
}

func TestFormatConfigLoadErrorValidate(t *testing.T) {
	err := formatConfigLoadError(&configutil.LoadError{
		Stage: configutil.StageValidate,
		Err:   errors.New("missing required field"),
	})
	require.Error(t, err)
	// StageValidate returns the error directly
	assert.Contains(t, err.Error(), "missing required field")
}

func TestFormatConfigLoadErrorDefault(t *testing.T) {
	err := formatConfigLoadError(&configutil.LoadError{
		Stage: "unknown",
		Err:   errors.New("something"),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "something")
}

func TestFormatConfigLoadErrorNonLoadError(t *testing.T) {
	original := errors.New("plain error")
	err := formatConfigLoadError(original)
	assert.Equal(t, original, err)
}

func TestLoadConfigInvalidPath(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/config.yml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read config")
}

func TestLoadConfigInvalidYAML(t *testing.T) {
	path := writeTestConfig(t, `invalid: yaml: content: [`)
	_, err := LoadConfig(path)
	require.Error(t, err)
}

func TestLoadConfigValidationFailure(t *testing.T) {
	// Missing required fields (linear.apiKey, resend.token, resend.mailTo)
	path := writeTestConfig(t, `
theme: dark
morning:
  strategy: all_assigned
ai:
  model: test
`)
	_, err := LoadConfig(path)
	require.Error(t, err)
}

func TestApplyDefaults(t *testing.T) {
	cfg := &Config{}
	applyDefaults(cfg)
	assert.Equal(t, "dark", cfg.Theme)
	assert.Equal(t, "all_assigned", cfg.Morning.Strategy)
	assert.Equal(t, "deepseek-v4-flash", cfg.AI.Model)
	assert.Equal(t, "zh", cfg.AI.Language)
}

func TestValidateConfig(t *testing.T) {
	cfg := &Config{
		Linear:  LinearConfig{APIKey: "key"},
		Resend:  ResendConfig{Token: "token", MailTo: []string{"test@example.com"}},
		Morning: MorningConfig{Strategy: "all_assigned"},
	}
	err := validateConfig(cfg)
	assert.NoError(t, err)
}

func TestValidateConfigMissingRequired(t *testing.T) {
	cfg := &Config{
		Morning: MorningConfig{Strategy: "all_assigned"},
	}
	err := validateConfig(cfg)
	assert.Error(t, err)
}
