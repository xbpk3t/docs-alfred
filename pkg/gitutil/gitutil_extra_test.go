package gitutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- PorcelainStatus tests ---

func TestPorcelainStatusCleanRepo(t *testing.T) {
	repoDir := t.TempDir()
	_, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)

	status, err := PorcelainStatus(repoDir)
	require.NoError(t, err)
	assert.Empty(t, status)
}

func TestPorcelainStatusWithChanges(t *testing.T) {
	repoDir := t.TempDir()
	repo, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "new.txt"), []byte("hello"), 0o644))

	wt, err := repo.Worktree()
	require.NoError(t, err)
	_, err = wt.Add("new.txt")
	require.NoError(t, err)

	status, err := PorcelainStatus(repoDir)
	require.NoError(t, err)
	assert.NotEmpty(t, status)
}

func TestPorcelainStatusInvalidPath(t *testing.T) {
	_, err := PorcelainStatus("/nonexistent/path")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open repo")
}

// --- ChangedFiles tests ---

func TestChangedFilesCleanRepo(t *testing.T) {
	repoDir := t.TempDir()
	_, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)

	files, err := ChangedFiles(repoDir)
	require.NoError(t, err)
	assert.Empty(t, files)
}

func TestChangedFilesWithChanges(t *testing.T) {
	repoDir := t.TempDir()
	repo, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "new.txt"), []byte("hello"), 0o644))

	wt, err := repo.Worktree()
	require.NoError(t, err)
	_, err = wt.Add("new.txt")
	require.NoError(t, err)

	files, err := ChangedFiles(repoDir)
	require.NoError(t, err)
	assert.NotEmpty(t, files)
}

func TestChangedFilesInvalidPath(t *testing.T) {
	_, err := ChangedFiles("/nonexistent/path")
	require.Error(t, err)
}

// --- FindRepoRoot tests ---

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

func TestDiffStatNewRepo(t *testing.T) {
	repoDir := t.TempDir()
	repo, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)
	wt, err := repo.Worktree()
	require.NoError(t, err)
	// Create a dummy file so we can commit
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "init.txt"), []byte("init"), 0o644))
	_, err = wt.Add("init.txt")
	require.NoError(t, err)
	_, err = wt.Commit("init", &git.CommitOptions{Author: &object.Signature{Name: "t", Email: "t@t.com"}})
	require.NoError(t, err)

	filePath := filepath.Join(repoDir, "new.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("hello\n"), 0o644))

	stat, err := DiffStat(repoDir, filePath)
	require.NoError(t, err)
	// New untracked file returns empty
	assert.Empty(t, stat)
}

func TestDiffStatNoChanges(t *testing.T) {
	repoDir := t.TempDir()
	filePath := filepath.Join(repoDir, "notes.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("same\n"), 0o644))
	repo, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)
	wt, err := repo.Worktree()
	require.NoError(t, err)
	_, err = wt.Add("notes.txt")
	require.NoError(t, err)
	_, err = wt.Commit("init", &git.CommitOptions{Author: &object.Signature{Name: "t", Email: "t@t.com"}})
	require.NoError(t, err)

	// File unchanged
	stat, err := DiffStat(repoDir, filePath)
	require.NoError(t, err)
	assert.Empty(t, stat)
}

func TestDiffStatInvalidRepoPath(t *testing.T) {
	_, err := DiffStat("/nonexistent/path", "file.txt")
	require.Error(t, err)
}

func TestDiffStatNoHEAD(t *testing.T) {
	// A repo with no commits has no HEAD reference
	repoDir := t.TempDir()
	_, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)

	filePath := filepath.Join(repoDir, "file.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("hello\n"), 0o644))

	stat, err := DiffStat(repoDir, filePath)
	require.NoError(t, err)
	assert.Empty(t, stat)
}

func TestDiffStatDeletedWorktreeFile(t *testing.T) {
	repoDir := t.TempDir()
	filePath := filepath.Join(repoDir, "file.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("hello\n"), 0o644))
	repo, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)
	wt, err := repo.Worktree()
	require.NoError(t, err)
	_, err = wt.Add("file.txt")
	require.NoError(t, err)
	_, err = wt.Commit("init", &git.CommitOptions{Author: &object.Signature{Name: "t", Email: "t@t.com"}})
	require.NoError(t, err)

	// Delete the worktree file
	require.NoError(t, os.Remove(filePath))

	_, err = DiffStat(repoDir, filePath)
	require.Error(t, err)
}

// --- Helper function tests ---

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

func TestNormalizeDiffText(t *testing.T) {
	assert.Empty(t, normalizeDiffText(""))
	assert.Equal(t, "a\n", normalizeDiffText("a"))
	assert.Equal(t, "a\nb\n", normalizeDiffText("a\nb"))
	assert.Equal(t, "a\nb\n", normalizeDiffText("a\nb\n"))
}

func TestCountLineDiffBothEmpty(t *testing.T) {
	add, del := countLineDiff("", "")
	assert.Equal(t, 0, add)
	assert.Equal(t, 0, del)
}

func TestCountLineDiffBothEmptyNewlines(t *testing.T) {
	add, del := countLineDiff("\n", "\n")
	assert.Equal(t, 0, add)
	assert.Equal(t, 0, del)
}

func TestChangedFilesWithWorktreeChanges(t *testing.T) {
	repoDir := t.TempDir()
	repo, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)
	wt, err := repo.Worktree()
	require.NoError(t, err)

	// Create and commit a file
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "file.txt"), []byte("original"), 0o644))
	_, err = wt.Add("file.txt")
	require.NoError(t, err)
	_, err = wt.Commit("init", &git.CommitOptions{Author: &object.Signature{Name: "t", Email: "t@t.com"}})
	require.NoError(t, err)

	// Modify the file in worktree
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "file.txt"), []byte("modified"), 0o644))

	files, err := ChangedFiles(repoDir)
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.Equal(t, "file.txt", files[0].Path)
	assert.Contains(t, files[0].Status, "M")
}
