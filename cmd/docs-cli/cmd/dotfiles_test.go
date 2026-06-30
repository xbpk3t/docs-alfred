package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- flag defaults ---

func TestDotfilesCheckCommandDefaults(t *testing.T) {
	dfCheck, _, err := newRootCmd().Find([]string{cmdDotfiles, cmdCheck})
	require.NoError(t, err)

	path, _ := dfCheck.Flags().GetString("path")
	assert.Equal(t, cmdDotfiles, path)

	dataDir, _ := dfCheck.Flags().GetString("data-dir")
	assert.Equal(t, "data/gh", dataDir)
}

// --- runDotfilesCheck error propagation ---

func TestRunDotfilesCheck_NonExistentDataDir(t *testing.T) {
	err := runDotfilesCheck(t.TempDir(), "/tmp/nonexistent-"+t.Name(), "text")
	require.Error(t, err)
}

func TestRunDotfilesCheck_JSONFormat(t *testing.T) {
	dfPath := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dfPath, "home", "base"), 0755))

	err := runDotfilesCheck(dfPath, t.TempDir(), "json")
	require.Error(t, err, "missing gh data is now fatal")
}

func TestRunDotfilesCheck_InvalidFormat(t *testing.T) {
	err := runDotfilesCheck(t.TempDir(), t.TempDir(), "yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no gh data found")
}

func TestRunDotfilesCheck_PassedWithEmptyDirs(t *testing.T) {
	dfPath := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dfPath, "home", "base"), 0755))

	err := runDotfilesCheck(dfPath, t.TempDir(), "text")
	require.Error(t, err, "missing gh data is now fatal")
}

func TestRunDotfilesCheck_EmptyFormat(t *testing.T) {
	dfPath := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dfPath, "home", "base"), 0755))

	err := runDotfilesCheck(dfPath, t.TempDir(), "")
	require.Error(t, err, "missing gh data is now fatal")
}

func TestRunDotfilesCheck_NonExistentDotfilesPath(t *testing.T) {
	err := runDotfilesCheck("/tmp/nonexistent-"+t.Name(), t.TempDir(), "text")
	require.Error(t, err)
}

func TestRunDotfilesCheck_ValidPathInvalidDataDir(t *testing.T) {
	dfPath := t.TempDir()
	// Create a dotfiles category that won't exist in gh data — causes df-only error
	require.NoError(t, os.MkdirAll(filepath.Join(dfPath, "home", "base", "myapp"), 0755))

	err := runDotfilesCheck(dfPath, "/tmp/nonexistent-"+t.Name(), "text")
	require.Error(t, err)
}

func TestRunDotfilesCheck_JSONWithIssues(t *testing.T) {
	dfPath := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dfPath, "home", "base"), 0755))

	err := runDotfilesCheck(dfPath, t.TempDir(), "json")
	require.Error(t, err, "missing gh data is now fatal")
}
