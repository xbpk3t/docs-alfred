package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadWikiConfigPreservesDefaultsWithPartialFile(t *testing.T) {
	oldConfigFile := wikiConfigFile
	oldWikiRootOpt := wikiRootOpt
	t.Cleanup(func() {
		wikiConfigFile = oldConfigFile
		wikiRootOpt = oldWikiRootOpt
	})

	configPath := filepath.Join(t.TempDir(), "wiki.yml")
	err := os.WriteFile(configPath, []byte("wiki:\n  concurrency: 2\n"), 0o600)
	require.NoError(t, err)

	wikiConfigFile = configPath
	wikiRootOpt = ""

	cfg, err := loadWikiConfig()
	require.NoError(t, err)
	require.Equal(t, 2, cfg.Wiki.Concurrency)
	require.Equal(t, "wiki", cfg.Wiki.WikiRoot)
	require.Equal(t, "https://docs.lucc.dev/gh.yml", cfg.Wiki.GhTopicsURL)
	require.Equal(t, "deepseek-v4-flash", cfg.Ai.Model)
}
