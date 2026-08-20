package gitutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- PorcelainStatus tests ---

func TestFindRepoRootFromSubdir(t *testing.T) {
	repoDir := t.TempDir()
	_, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)

	subdir := filepath.Join(repoDir, "sub", "dir")
	require.NoError(t, os.MkdirAll(subdir, 0o755))

	root, err := FindRepoRoot(subdir)
	require.NoError(t, err)
	assert.Equal(t, repoDir, root)
}

func TestFindRepoRootFromRoot(t *testing.T) {
	repoDir := t.TempDir()
	_, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)

	root, err := FindRepoRoot(repoDir)
	require.NoError(t, err)
	assert.Equal(t, repoDir, root)
}

func TestFindRepoRootNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := FindRepoRoot(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no git repository found")
}

// --- DiffStat edge cases ---

func TestSplitLines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{name: "empty", input: "", want: 0},
		{name: "single", input: "a", want: 1},
		{name: "two lines", input: "a\nb", want: 2},
		{name: "trailing newline", input: "a\nb\n", want: 2},
		{name: "multiple", input: "a\nb\nc\n", want: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := splitLines(tt.input)
			assert.Len(t, lines, tt.want)
		})
	}
}

func TestCountLines(t *testing.T) {
	assert.Equal(t, 0, countLines(""))
	assert.Equal(t, 1, countLines("a"))
	assert.Equal(t, 2, countLines("a\nb"))
	assert.Equal(t, 2, countLines("a\nb\n"))
}
