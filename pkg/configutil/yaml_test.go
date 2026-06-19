package configutil

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadYAMLBytes(t *testing.T) {
	k, err := LoadYAMLBytes([]byte("server:\n  port: 8080\n"))
	if err != nil {
		t.Fatalf("LoadYAMLBytes() error = %v", err)
	}

	if got := k.Int("server.port"); got != 8080 {
		t.Fatalf("server.port = %d, want 8080", got)
	}
}

func TestLoadYAMLBytesRejectsInvalidYAML(t *testing.T) {
	_, err := LoadYAMLBytes([]byte("server: ["))
	if err == nil {
		t.Fatal("LoadYAMLBytes() error = nil, want parse error")
	}
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

	if err != nil {
		t.Fatalf("LoadYAMLConfig() error = %v", err)
	}
	if got.Name != "default" || got.Count != 2 {
		t.Fatalf("config = %+v, want default name and count 2", got)
	}
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

	if err != nil {
		t.Fatalf("LoadYAMLConfig() error = %v", err)
	}
	if got.Name != "default" {
		t.Fatalf("Name = %q, want default", got.Name)
	}
}

func TestLoadYAMLConfigEmptyPathUsesInitialValue(t *testing.T) {
	type config struct {
		Name string `yaml:"name"`
	}

	got, err := LoadYAMLConfig(LoadYAMLConfigOptions[config]{
		Initial: config{Name: "initial"},
	})

	if err != nil {
		t.Fatalf("LoadYAMLConfig() error = %v", err)
	}
	if got.Name != "initial" {
		t.Fatalf("Name = %q, want initial", got.Name)
	}
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
	if err == nil {
		t.Fatal("LoadYAMLConfig() error = nil, want validation error")
	}
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
	if err != nil {
		t.Fatalf("LoadYAMLConfig() error = %v", err)
	}
	if got.Name != "env" || got.Count != 7 || !got.Enabled || got.Timeout != 30*time.Second {
		t.Fatalf("config = %+v, want env overrides applied", got)
	}
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
	if err != nil {
		t.Fatalf("LoadYAMLConfig() error = %v", err)
	}
	if got.Name != "yaml" {
		t.Fatalf("Name = %q, want YAML value preserved", got.Name)
	}
}

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	return path
}
