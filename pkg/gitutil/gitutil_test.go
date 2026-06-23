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

func TestPorcelainStatus_InvalidRepo(t *testing.T) {
	_, err := PorcelainStatus("/nonexistent/path")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open repo")
}

func TestPorcelainStatus_ValidRepo(t *testing.T) {
	repoDir := t.TempDir()
	_, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)

	status, err := PorcelainStatus(repoDir)
	require.NoError(t, err)
	// Empty repo with no changes
	assert.Equal(t, "", status)
}

func TestChangedFiles_InvalidRepo(t *testing.T) {
	_, err := ChangedFiles("/nonexistent/path")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open repo")
}

func TestChangedFiles_WithChanges(t *testing.T) {
	repoDir := t.TempDir()
	repo, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)

	wt, err := repo.Worktree()
	require.NoError(t, err)

	// Create a committed file
	filePath := filepath.Join(repoDir, "a.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("original\n"), 0o644))
	_, err = wt.Add("a.txt")
	require.NoError(t, err)
	_, err = wt.Commit("initial", &git.CommitOptions{Author: &object.Signature{Name: "test", Email: "test@example.com"}})
	require.NoError(t, err)

	// Modify the file
	require.NoError(t, os.WriteFile(filePath, []byte("modified\n"), 0o644))

	files, err := ChangedFiles(repoDir)
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.Equal(t, "a.txt", files[0].Path)
}

func TestChangedFiles_UntrackedFile(t *testing.T) {
	repoDir := t.TempDir()
	_, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "new.txt"), []byte("data\n"), 0o644))

	files, err := ChangedFiles(repoDir)
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.Equal(t, "new.txt", files[0].Path)
	assert.Equal(t, "??", files[0].Status)
}

func TestDiffStat_InvalidRepoPath(t *testing.T) {
	_, err := DiffStat("/nonexistent/path", "file.txt")
	require.Error(t, err)
}

func TestDiffStat_NoChanges(t *testing.T) {
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

	// No changes to file
	stat, err := DiffStat(repoDir, filePath)
	require.NoError(t, err)
	require.Empty(t, stat)
}

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

func TestCountLineDiff_EmptyBoth(t *testing.T) {
	additions, deletions := countLineDiff("", "")
	assert.Equal(t, 0, additions)
	assert.Equal(t, 0, deletions)
}

func TestNormalizeDiffText_Empty(t *testing.T) {
	assert.Equal(t, "", normalizeDiffText(""))
}

func TestSplitLines_Empty(t *testing.T) {
	assert.Nil(t, splitLines(""))
}

func TestSplitLines_SingleLine(t *testing.T) {
	assert.Equal(t, []string{"hello"}, splitLines("hello"))
}

func TestDiffStat_WorktreeFileDeleted(t *testing.T) {
	repoDir := t.TempDir()
	filePath := filepath.Join(repoDir, "notes.txt")

	require.NoError(t, os.WriteFile(filePath, []byte("a\nb\n"), 0o644))
	repo, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)

	wt, err := repo.Worktree()
	require.NoError(t, err)
	_, err = wt.Add("notes.txt")
	require.NoError(t, err)
	_, err = wt.Commit("initial", &git.CommitOptions{Author: &object.Signature{Name: "test", Email: "test@example.com"}})
	require.NoError(t, err)

	// Delete the worktree file — triggers "read worktree file" error
	require.NoError(t, os.Remove(filePath))

	_, err = DiffStat(repoDir, filePath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read worktree file")
}

func TestDiffStat_NewFileAddedToEmptyRepo(t *testing.T) {
	repoDir := t.TempDir()
	repo, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)

	filePath := filepath.Join(repoDir, "newfile.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("content\n"), 0o644))

	wt, err := repo.Worktree()
	require.NoError(t, err)
	_, err = wt.Add("newfile.txt")
	require.NoError(t, err)
	_, err = wt.Commit("add file", &git.CommitOptions{Author: &object.Signature{Name: "test", Email: "test@example.com"}})
	require.NoError(t, err)

	// Modify the committed file
	require.NoError(t, os.WriteFile(filePath, []byte("modified\n"), 0o644))

	stat, err := DiffStat(repoDir, filePath)
	require.NoError(t, err)
	require.NotEmpty(t, stat)
	assert.Contains(t, stat, "newfile.txt")
}

func TestPorcelainStatus_WithModifiedFile(t *testing.T) {
	repoDir := t.TempDir()
	repo, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)

	wt, err := repo.Worktree()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "f.txt"), []byte("v1\n"), 0o644))
	_, err = wt.Add("f.txt")
	require.NoError(t, err)
	_, err = wt.Commit("init", &git.CommitOptions{Author: &object.Signature{Name: "test", Email: "test@example.com"}})
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "f.txt"), []byte("v2\n"), 0o644))

	status, err := PorcelainStatus(repoDir)
	require.NoError(t, err)
	assert.Contains(t, status, "f.txt")
}

func TestDiffStat_HeadlessRepo(t *testing.T) {
	// A repo with no commits has no HEAD, triggering ErrReferenceNotFound path
	repoDir := t.TempDir()
	_, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)

	filePath := filepath.Join(repoDir, "file.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("content\n"), 0o644))

	stat, err := DiffStat(repoDir, filePath)
	require.NoError(t, err)
	require.Empty(t, stat, "headless repo should return empty diff")
}

func TestDiffStat_SubDirectoryRepoPath(t *testing.T) {
	// Test DiffStat when repoPath is a subdirectory of the repo
	repoDir := t.TempDir()
	filePath := filepath.Join(repoDir, "sub", "file.txt")
	require.NoError(t, os.MkdirAll(filepath.Dir(filePath), 0o755))
	require.NoError(t, os.WriteFile(filePath, []byte("a\nb\n"), 0o644))

	repo, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)

	wt, err := repo.Worktree()
	require.NoError(t, err)
	_, err = wt.Add("sub/file.txt")
	require.NoError(t, err)
	_, err = wt.Commit("init", &git.CommitOptions{Author: &object.Signature{Name: "test", Email: "test@example.com"}})
	require.NoError(t, err)

	// Modify the file
	require.NoError(t, os.WriteFile(filePath, []byte("a\nc\n"), 0o644))

	// DiffStat with subDir as repoPath (FindRepoRoot will walk up to repoDir)
	stat, err := DiffStat(filepath.Join(repoDir, "sub"), filePath)
	require.NoError(t, err)
	require.NotEmpty(t, stat)
}

func TestChangedFiles_EmptyRepo(t *testing.T) {
	repoDir := t.TempDir()
	_, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)

	files, err := ChangedFiles(repoDir)
	require.NoError(t, err)
	require.Empty(t, files)
}

func TestPorcelainStatus_EmptyRepo(t *testing.T) {
	repoDir := t.TempDir()
	_, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)

	status, err := PorcelainStatus(repoDir)
	require.NoError(t, err)
	assert.Equal(t, "", status)
}

func TestPorcelainStatus_BareRepo(t *testing.T) {
	repoDir := t.TempDir()
	_, err := git.PlainInit(repoDir, true) // bare repo
	require.NoError(t, err)

	_, err = PorcelainStatus(repoDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "worktree")
}

func TestChangedFiles_BareRepo(t *testing.T) {
	repoDir := t.TempDir()
	_, err := git.PlainInit(repoDir, true) // bare repo
	require.NoError(t, err)

	_, err = ChangedFiles(repoDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "worktree")
}

func TestDiffStat_CorruptTree(t *testing.T) {
	// Create a repo with a commit, then corrupt the tree object
	repoDir := t.TempDir()
	filePath := filepath.Join(repoDir, "file.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("a\n"), 0o644))

	repo, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)

	wt, err := repo.Worktree()
	require.NoError(t, err)
	_, err = wt.Add("file.txt")
	require.NoError(t, err)
	_, err = wt.Commit("init", &git.CommitOptions{Author: &object.Signature{Name: "test", Email: "test@example.com"}})
	require.NoError(t, err)

	// Find the tree hash by reading the commit
	head, err := repo.Head()
	require.NoError(t, err)
	commit, err := object.GetCommit(repo.Storer, head.Hash())
	require.NoError(t, err)
	treeHash := commit.TreeHash

	// Corrupt the tree object
	objDir := filepath.Join(repoDir, ".git", "objects", treeHash.String()[:2])
	objFile := filepath.Join(objDir, treeHash.String()[2:])
	require.NoError(t, os.Chmod(objFile, 0o644))
	require.NoError(t, os.Remove(objFile))
	require.NoError(t, os.WriteFile(objFile, []byte("corrupt"), 0o444))

	// Modify the worktree file
	require.NoError(t, os.WriteFile(filePath, []byte("b\n"), 0o644))

	_, err = DiffStat(repoDir, filePath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read HEAD file")
}

func TestDiffStat_CorruptBlob(t *testing.T) {
	// Create a repo, then corrupt the blob object for the file
	repoDir := t.TempDir()
	filePath := filepath.Join(repoDir, "file.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("content\n"), 0o644))

	repo, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)

	wt, err := repo.Worktree()
	require.NoError(t, err)
	_, err = wt.Add("file.txt")
	require.NoError(t, err)
	_, err = wt.Commit("init", &git.CommitOptions{Author: &object.Signature{Name: "test", Email: "test@example.com"}})
	require.NoError(t, err)

	// Find the blob hash by looking at the tree
	head, err := repo.Head()
	require.NoError(t, err)
	commit, err := object.GetCommit(repo.Storer, head.Hash())
	require.NoError(t, err)
	tree, err := commit.Tree()
	require.NoError(t, err)
	f, err := tree.File("file.txt")
	require.NoError(t, err)
	blobHash := f.Hash

	// Corrupt the blob object
	objDir := filepath.Join(repoDir, ".git", "objects", blobHash.String()[:2])
	objFile := filepath.Join(objDir, blobHash.String()[2:])
	require.NoError(t, os.Chmod(objFile, 0o644))
	require.NoError(t, os.Remove(objFile))
	require.NoError(t, os.WriteFile(objFile, []byte("bad"), 0o444))

	// Modify the worktree file
	require.NoError(t, os.WriteFile(filePath, []byte("changed\n"), 0o644))

	_, err = DiffStat(repoDir, filePath)
	require.Error(t, err)
}

func TestDiffStat_FakeGitDir(t *testing.T) {
	// Create a directory with a .git dir but no valid git repo inside.
	// FindRepoRoot will find .git, but PlainOpen will fail.
	repoDir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(repoDir, ".git"), 0o755))
	// No HEAD file, no objects - PlainOpen should fail

	filePath := filepath.Join(repoDir, "file.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("data\n"), 0o644))

	_, err := DiffStat(repoDir, filePath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open repo")
}

func TestPorcelainStatus_CorruptIndex(t *testing.T) {
	repoDir := t.TempDir()
	repo, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)

	wt, err := repo.Worktree()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "f.txt"), []byte("v1\n"), 0o644))
	_, err = wt.Add("f.txt")
	require.NoError(t, err)
	_, err = wt.Commit("init", &git.CommitOptions{Author: &object.Signature{Name: "test", Email: "test@example.com"}})
	require.NoError(t, err)

	// Corrupt the index file
	indexPath := filepath.Join(repoDir, ".git", "index")
	require.NoError(t, os.Remove(indexPath))
	require.NoError(t, os.WriteFile(indexPath, []byte("corrupt index"), 0o644))

	_, err = PorcelainStatus(repoDir)
	require.Error(t, err)
}

func TestChangedFiles_CorruptIndex(t *testing.T) {
	repoDir := t.TempDir()
	repo, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)

	wt, err := repo.Worktree()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "f.txt"), []byte("v1\n"), 0o644))
	_, err = wt.Add("f.txt")
	require.NoError(t, err)
	_, err = wt.Commit("init", &git.CommitOptions{Author: &object.Signature{Name: "test", Email: "test@example.com"}})
	require.NoError(t, err)

	// Corrupt the index file
	indexPath := filepath.Join(repoDir, ".git", "index")
	require.NoError(t, os.Remove(indexPath))
	require.NoError(t, os.WriteFile(indexPath, []byte("corrupt index"), 0o644))

	_, err = ChangedFiles(repoDir)
	require.Error(t, err)
}

func TestDiffStat_CorruptCommit(t *testing.T) {
	// Create a repo with a commit, then corrupt the commit object
	repoDir := t.TempDir()
	filePath := filepath.Join(repoDir, "file.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("a\n"), 0o644))

	repo, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)

	wt, err := repo.Worktree()
	require.NoError(t, err)
	_, err = wt.Add("file.txt")
	require.NoError(t, err)
	hash, err := wt.Commit("init", &git.CommitOptions{Author: &object.Signature{Name: "test", Email: "test@example.com"}})
	require.NoError(t, err)

	// Corrupt the commit object in the objects directory
	objDir := filepath.Join(repoDir, ".git", "objects", hash.String()[:2])
	objFile := filepath.Join(objDir, hash.String()[2:])
	require.NoError(t, os.Chmod(objFile, 0o644))
	require.NoError(t, os.Remove(objFile))
	require.NoError(t, os.WriteFile(objFile, []byte("corrupt"), 0o444))

	// Modify the worktree file
	require.NoError(t, os.WriteFile(filePath, []byte("b\n"), 0o644))

	_, err = DiffStat(repoDir, filePath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read HEAD file")
}
