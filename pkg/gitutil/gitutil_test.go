package gitutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindRepoRoot_NoRepo(t *testing.T) {
	dir := t.TempDir()
	_, err := FindRepoRoot(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no git repository found")
}

func TestFindRepoRoot_FromSubdir(t *testing.T) {
	repoDir := t.TempDir()
	_, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)

	subDir := filepath.Join(repoDir, "sub", "dir")
	require.NoError(t, os.MkdirAll(subDir, 0o755))

	root, err := FindRepoRoot(subDir)
	require.NoError(t, err)
	assert.Equal(t, repoDir, root)
}

func TestSplitLines_Empty(t *testing.T) {
	assert.Nil(t, splitLines(""))
}

func TestSplitLines_SingleLine(t *testing.T) {
	assert.Equal(t, []string{"hello"}, splitLines("hello"))
}
