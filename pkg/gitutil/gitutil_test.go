package gitutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/stretchr/testify/require"
)

func TestCountLineDiff(t *testing.T) {
	tests := []struct {
		name      string
		old       string
		new       string
		additions int
		deletions int
	}{
		{name: "same content", old: "a\nb\n", new: "a\nb\n", additions: 0, deletions: 0},
		{name: "only additions", old: "a\n", new: "a\nb\nc\n", additions: 2, deletions: 0},
		{name: "only deletions", old: "a\nb\nc\n", new: "a\n", additions: 0, deletions: 2},
		{name: "single replacement", old: "a\nb\nc\n", new: "a\nx\nc\n", additions: 1, deletions: 1},
		{name: "reordered lines", old: "a\nb\nc\n", new: "b\na\nc\n", additions: 1, deletions: 1},
		{name: "no trailing newline", old: "a\nb", new: "a\nb\nc", additions: 1, deletions: 0},
		{name: "empty old", old: "", new: "a\n", additions: 1, deletions: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			additions, deletions := countLineDiff(tt.old, tt.new)
			require.Equal(t, tt.additions, additions)
			require.Equal(t, tt.deletions, deletions)
		})
	}
}

func TestDiffStat(t *testing.T) {
	repoDir := t.TempDir()
	filePath := filepath.Join(repoDir, "notes.txt")

	require.NoError(t, os.WriteFile(filePath, []byte("a\nb\n"), 0o644))
	repo, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)

	wt, err := repo.Worktree()
	require.NoError(t, err)
	_, err = wt.Add("notes.txt")
	require.NoError(t, err)
	_, err = wt.Commit("initial commit", &git.CommitOptions{Author: &object.Signature{Name: "test", Email: "test@example.com"}})
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filePath, []byte("a\nx\nc\n"), 0o644))

	stat, err := DiffStat(repoDir, filePath)
	require.NoError(t, err)
	require.Equal(t, " notes.txt | 3 ++-", stat)
}

func TestDiffStatReturnsEmptyForUntrackedFile(t *testing.T) {
	repoDir := t.TempDir()
	repo, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)
	_, err = repo.Worktree()
	require.NoError(t, err)

	filePath := filepath.Join(repoDir, "new.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("hello\n"), 0o644))

	stat, err := DiffStat(repoDir, filePath)
	require.NoError(t, err)
	require.Empty(t, stat)
}
