package cmd

import (
	"encoding/json"
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
	stdout := captureStdout(t)

	dfPath := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dfPath, "home", "base"), 0755))

	err := runDotfilesCheck(dfPath, t.TempDir(), "json")
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout()), &result))
	assert.Equal(t, "dotfiles check", result["name"])
}

func TestRunDotfilesCheck_InvalidFormat(t *testing.T) {
	err := runDotfilesCheck(t.TempDir(), t.TempDir(), "yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported output format")
}

func TestRunDotfilesCheck_PassedWithEmptyDirs(t *testing.T) {
	stdout := captureStdout(t)

	dfPath := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dfPath, "home", "base"), 0755))

	err := runDotfilesCheck(dfPath, t.TempDir(), "text")
	require.NoError(t, err)

	out := stdout()
	assert.Contains(t, out, "dotfiles check passed")
	assert.Contains(t, out, "summary: shared=")
}

func TestRunDotfilesCheck_EmptyFormat(t *testing.T) {
	stdout := captureStdout(t)

	dfPath := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dfPath, "home", "base"), 0755))

	err := runDotfilesCheck(dfPath, t.TempDir(), "")
	require.NoError(t, err, "empty format defaults to text")

	out := stdout()
	assert.Contains(t, out, "dotfiles check passed")
}

func TestRunDotfilesCheck_NonExistentDotfilesPath(t *testing.T) {
	err := runDotfilesCheck("/tmp/nonexistent-"+t.Name(), t.TempDir(), "text")
	require.Error(t, err)
}

func TestRunDotfilesCheck_ValidPathInvalidDataDir(t *testing.T) {
	stdout := captureStdout(t)

	dfPath := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dfPath, "home", "base"), 0755))

	err := runDotfilesCheck(dfPath, "/tmp/nonexistent-"+t.Name(), "text")
	require.NoError(t, err, "dotfiles check tolerates non-existent data dir")

	out := stdout()
	assert.Contains(t, out, "dotfiles check passed")
}

func TestRunDotfilesCheck_JSONWithIssues(t *testing.T) {
	stdout := captureStdout(t)

	// valid dotfiles path but no matching data/gh → produces issues
	dfPath := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dfPath, "home", "base"), 0755))

	err := runDotfilesCheck(dfPath, t.TempDir(), "json")
	require.NoError(t, err, "dotfiles check with empty data dir produces warnings, not errors")

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout()), &result))
	assert.Equal(t, "dotfiles check", result["name"])
}
