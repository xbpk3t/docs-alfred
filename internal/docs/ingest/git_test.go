package wikiingest

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- gitOutput ---

func TestGitOutputNilCommandRunner(t *testing.T) {
	_, err := gitOutput(context.Background(), t.TempDir(), nil, "status")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CommandRunner not provided")
}

func TestGitOutputCommandError(t *testing.T) {
	runCmd := func(_ context.Context, _, _ string, _ ...string) ([]byte, error) {
		return []byte("fatal: not a git repo"), errors.New("exit status 128")
	}
	_, err := gitOutput(context.Background(), t.TempDir(), runCmd, "rev-parse", "--show-toplevel")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git rev-parse --show-toplevel")
	assert.Contains(t, err.Error(), "not a git repo")
}

func TestGitOutputSuccess(t *testing.T) {
	runCmd := func(_ context.Context, _, _ string, _ ...string) ([]byte, error) {
		return []byte("/some/repo\n"), nil
	}
	out, err := gitOutput(context.Background(), t.TempDir(), runCmd, "rev-parse", "--show-toplevel")
	require.NoError(t, err)
	assert.Equal(t, "/some/repo\n", out)
}

// --- evalSymlinksOrOriginal ---

func TestEvalSymlinksOrOriginalNonexistent(t *testing.T) {
	path := "/tmp/nonexistent-symlink-path-12345"
	result := evalSymlinksOrOriginal(path)
	assert.Equal(t, path, result)
}

func TestEvalSymlinksOrOriginalRealPath(t *testing.T) {
	dir := t.TempDir()
	result := evalSymlinksOrOriginal(dir)
	assert.NotEmpty(t, result)
}

// --- appendChangedMarkdownPaths ---

func TestAppendChangedMarkdownPathsNonMD(t *testing.T) {
	dir := t.TempDir()
	// Create a non-.md file
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content"), 0o600))

	seen := make(map[string]bool)
	var paths []string
	paths = appendChangedMarkdownPaths(paths, seen, dir, "file.txt\n")
	assert.Empty(t, paths)
}

func TestAppendChangedMarkdownPathsMD(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.md"), []byte("content"), 0o600))

	seen := make(map[string]bool)
	var paths []string
	paths = appendChangedMarkdownPaths(paths, seen, dir, "file.md\n")
	assert.Len(t, paths, 1)
	assert.Contains(t, paths[0], "file.md")
}

func TestAppendChangedMarkdownPathsDuplicate(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.md"), []byte("content"), 0o600))

	seen := make(map[string]bool)
	var paths []string
	paths = appendChangedMarkdownPaths(paths, seen, dir, "file.md\nfile.md\n")
	assert.Len(t, paths, 1)
}

func TestAppendChangedMarkdownPathsNonexistentFile(t *testing.T) {
	dir := t.TempDir()
	seen := make(map[string]bool)
	var paths []string
	paths = appendChangedMarkdownPaths(paths, seen, dir, "missing.md\n")
	assert.Empty(t, paths)
}

// --- changedWikiMarkdownPaths ---

func TestChangedWikiMarkdownPathsNilCommandRunner(t *testing.T) {
	_, err := changedWikiMarkdownPaths(context.Background(), t.TempDir(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CommandRunner not provided")
}

func TestChangedWikiMarkdownPathsGitError(t *testing.T) {
	runCmd := func(_ context.Context, _, _ string, _ ...string) ([]byte, error) {
		return []byte("fatal: not a git repo"), errors.New("exit status 128")
	}
	_, err := changedWikiMarkdownPaths(context.Background(), t.TempDir(), runCmd)
	require.Error(t, err)
}

// --- changedWikiGitRoots ---

func TestChangedWikiGitRootsEmptyGitRoot(t *testing.T) {
	runCmd := func(_ context.Context, _, _ string, _ ...string) ([]byte, error) {
		return []byte(""), nil
	}
	_, err := changedWikiGitRoots(context.Background(), t.TempDir(), runCmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty git root")
}

func TestChangedWikiGitRootsSuccess(t *testing.T) {
	dir := t.TempDir()
	// evalSymlinksOrOriginal resolves symlinks (e.g. /var -> /private/var on macOS)
	resolvedDir := evalSymlinksOrOriginal(dir)
	runCmd := func(_ context.Context, _, name string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "rev-parse" {
			return []byte(resolvedDir + "\n"), nil
		}
		return []byte(""), nil
	}
	roots, err := changedWikiGitRoots(context.Background(), dir, runCmd)
	require.NoError(t, err)
	assert.Equal(t, resolvedDir, roots.repoRoot)
}
