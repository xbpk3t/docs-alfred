package configutil

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadYAMLBytes(t *testing.T) {
	k, err := LoadYAMLBytes([]byte("server:\n  port: 8080\n"))
	require.NoError(t, err)
	require.Equal(t, 8080, k.Int("server.port"))
}

func TestLoadYAMLBytesRejectsInvalidYAML(t *testing.T) {
	_, err := LoadYAMLBytes([]byte("server: ["))
	require.Error(t, err, "LoadYAMLBytes should return parse error")
}

func TestLoadYAMLConfigPreservesInitialDefaults(t *testing.T) {
	type config struct {
		Name  string `yaml:"name"`
		Count int    `yaml:"count"`
	}
	path := writeConfigFile(t, "count: 2\n")

	got, err := LoadYAMLConfig(LoadYAMLConfigOptions[config]{
		Path:    path,
		Initial: config{Name: "default", Count: 1},
	})

	require.NoError(t, err)
	require.Equal(t, "default", got.Name)
	require.Equal(t, 2, got.Count)
}

func TestLoadYAMLConfigAppliesAfterUnmarshalAndValidate(t *testing.T) {
	type config struct {
		Name string `yaml:"name"`
	}
	path := writeConfigFile(t, "{}\n")

	got, err := LoadYAMLConfig(LoadYAMLConfigOptions[config]{
		Path: path,
		AfterUnmarshal: func(cfg *config) error {
			if cfg.Name == "" {
				cfg.Name = "default"
			}

			return nil
		},
		Validate: func(cfg *config) error {
			if cfg.Name == "" {
				return errors.New("name is required")
			}

			return nil
		},
	})

	require.NoError(t, err)
	require.Equal(t, "default", got.Name)
}

func TestLoadYAMLConfigEmptyPathUsesInitialValue(t *testing.T) {
	type config struct {
		Name string `yaml:"name"`
	}

	got, err := LoadYAMLConfig(LoadYAMLConfigOptions[config]{
		Initial: config{Name: "initial"},
	})

	require.NoError(t, err)
	require.Equal(t, "initial", got.Name)
}

func TestLoadYAMLConfigReturnsValidationError(t *testing.T) {
	type config struct {
		Name string `yaml:"name"`
	}

	_, err := LoadYAMLConfig(LoadYAMLConfigOptions[config]{
		Validate: func(cfg *config) error {
			return errors.New("invalid")
		},
	})
	require.Error(t, err, "LoadYAMLConfig should return validation error")
}

func TestLoadYAMLConfigAppliesEnvOverrides(t *testing.T) {
	type config struct {
		Name    string        `yaml:"name"`
		Count   int           `yaml:"count"`
		Enabled bool          `yaml:"enabled"`
		Timeout time.Duration `yaml:"timeout"`
	}
	path := writeConfigFile(t, "name: yaml\ncount: 2\nenabled: false\ntimeout: 5s\n")
	t.Setenv("TEST_NAME", "env")
	t.Setenv("TEST_COUNT", "7")
	t.Setenv("TEST_ENABLED", "true")
	t.Setenv("TEST_TIMEOUT", "30s")

	got, err := LoadYAMLConfig(LoadYAMLConfigOptions[config]{
		Path: path,
		EnvOverrides: []EnvOverride{
			{Name: "TEST_NAME", Path: "name"},
			{Name: "TEST_COUNT", Path: "count"},
			{Name: "TEST_ENABLED", Path: "enabled"},
			{Name: "TEST_TIMEOUT", Path: "timeout"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "env", got.Name)
	require.Equal(t, 7, got.Count)
	require.True(t, got.Enabled)
	require.Equal(t, 30*time.Second, got.Timeout)
}

func TestLoadYAMLConfigIgnoresEmptyEnvOverrides(t *testing.T) {
	type config struct {
		Name string `yaml:"name"`
	}
	path := writeConfigFile(t, "name: yaml\n")
	t.Setenv("TEST_NAME", "")

	got, err := LoadYAMLConfig(LoadYAMLConfigOptions[config]{
		Path:         path,
		EnvOverrides: []EnvOverride{{Name: "TEST_NAME", Path: "name"}},
	})
	require.NoError(t, err)
	require.Equal(t, "yaml", got.Name, "YAML value should be preserved when env is empty")
}

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	return path
}
