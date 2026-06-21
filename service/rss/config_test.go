package rss

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewConfigPreservesExplicitFalseDebug(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rss2nl.yml")
	content := []byte(`newsletter:
  schedule: daily
env:
  debug: false
`)
	require.NoError(t, os.WriteFile(path, content, 0o600))

	cfg, err := NewConfig(path)
	require.NoError(t, err)
	require.False(t, cfg.EnvConfig.Debug, "EnvConfig.Debug should be false")
}
