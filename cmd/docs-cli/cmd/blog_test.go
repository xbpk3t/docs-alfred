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

func TestBlogCheckCommandDefaults(t *testing.T) {
	blogCheck, _, err := newRootCmd().Find([]string{"blog", cmdCheck})
	require.NoError(t, err)

	dataDir, _ := blogCheck.Flags().GetString("data-dir")
	assert.Equal(t, "data/gh", dataDir)

	blogDir, _ := blogCheck.Flags().GetString("blog-dir")
	assert.Equal(t, "blog", blogDir)
}

// --- runBlogCheck error propagation ---

func TestRunBlogCheck_NonExistentDataDir(t *testing.T) {
	err := runBlogCheck("/tmp/nonexistent-"+t.Name(), t.TempDir(), "text")
	require.Error(t, err)
}

func TestRunBlogCheck_NonExistentBlogDir(t *testing.T) {
	stdout := captureStdout(t)

	// blog check tolerates non-existent blog dir — walks nothing, passes cleanly
	err := runBlogCheck(t.TempDir(), "/tmp/nonexistent-"+t.Name(), "text")
	require.NoError(t, err)

	out := stdout()
	assert.Contains(t, out, "blog check passed")
}

func TestRunBlogCheck_BothDirsNonExistent(t *testing.T) {
	err := runBlogCheck("/tmp/nonexistent-a-"+t.Name(), "/tmp/nonexistent-b-"+t.Name(), "text")
	require.Error(t, err)
}

// --- runBlogCheck JSON format ---

func TestRunBlogCheck_JSONEmptyDirs(t *testing.T) {
	stdout := captureStdout(t)

	err := runBlogCheck(t.TempDir(), t.TempDir(), "json")
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout()), &result))
	assert.Equal(t, "blog check", result["name"])
	assert.Equal(t, true, result["ok"])
}

func TestRunBlogCheck_JSONWithIssues(t *testing.T) {
	stdout := captureStdout(t)

	// blog dir with subdirs that have no matching data/gh → issues found
	blogDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(blogDir, "algo", "tree"), 0755))

	err := runBlogCheck(t.TempDir(), blogDir, "json")
	require.Error(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout()), &result))
	assert.Equal(t, "blog check", result["name"])
	assert.Equal(t, false, result["ok"])
}

// --- runBlogCheck text format ---

func TestRunBlogCheck_TextEmptyDirs(t *testing.T) {
	stdout := captureStdout(t)

	err := runBlogCheck(t.TempDir(), t.TempDir(), "text")
	require.NoError(t, err)

	out := stdout()
	assert.Contains(t, out, "blog check passed")
	assert.Contains(t, out, "summary:")
}

func TestRunBlogCheck_TextWithIssues(t *testing.T) {
	stdout := captureStdout(t)

	blogDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(blogDir, "algo", "tree"), 0755))

	err := runBlogCheck(t.TempDir(), blogDir, "text")
	require.Error(t, err)

	out := stdout()
	assert.Contains(t, out, "blog check failed")
	assert.Contains(t, out, "algo/tree")
}

// --- runBlogCheck format edge cases ---

func TestRunBlogCheck_EmptyFormat(t *testing.T) {
	stdout := captureStdout(t)

	err := runBlogCheck(t.TempDir(), t.TempDir(), "")
	require.NoError(t, err, "empty format defaults to text")

	out := stdout()
	assert.Contains(t, out, "blog check passed")
}

func TestRunBlogCheck_InvalidFormat(t *testing.T) {
	err := runBlogCheck(t.TempDir(), t.TempDir(), "xml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported output format")
}
