package rss

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewConfigPreservesExplicitFalseDebug(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rss2nl.yml")
	content := []byte(`newsletter:
  schedule: daily
env:
  debug: false
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := NewConfig(path)
	if err != nil {
		t.Fatalf("NewConfig() error = %v", err)
	}
	if cfg.EnvConfig.Debug {
		t.Fatal("EnvConfig.Debug = true, want explicit false")
	}
}
