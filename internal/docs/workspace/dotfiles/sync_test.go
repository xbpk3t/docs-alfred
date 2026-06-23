package dotfiles

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapToGhNonHomeBasePrefix(t *testing.T) {
	assert.Nil(t, mapToGh("other/path/file.txt"))
}

func TestMapToGhNoCategory(t *testing.T) {
	assert.Nil(t, mapToGh("home/base/"))
}

func TestMapToGhEmptyCategory(t *testing.T) {
	assert.Nil(t, mapToGh("home/base//file.txt"))
}

func TestMapToGhNoGHFiles(t *testing.T) {
	// Category exists but no gh files found (data/gh dir doesn't exist)
	gh := mapToGh("home/base/nonexistent/file.txt")
	assert.Nil(t, gh)
}

func TestRunSyncRecordNonExistentPath(t *testing.T) {
	result := RunSyncRecord(SyncRecordOptions{DotfilesPath: "/tmp/nonexistent-12345"})
	assert.False(t, result.OK)
	assert.Contains(t, result.Error, "not found")
}

func TestRunSyncRecordNotGitRepo(t *testing.T) {
	result := RunSyncRecord(SyncRecordOptions{DotfilesPath: t.TempDir()})
	assert.False(t, result.OK)
	assert.Contains(t, result.Error, "not a git repository")
}

func TestSyncRecordResultFields(t *testing.T) {
	result := &SyncRecordResult{
		DotfilesPath: "/tmp/dotfiles",
		OK:           true,
		ChangedFiles: []ChangeFile{{Path: "file.txt", Status: "M"}},
	}
	assert.True(t, result.OK)
	assert.Len(t, result.ChangedFiles, 1)
}

func TestChangeFileStruct(t *testing.T) {
	cf := ChangeFile{
		Path:   "home/base/tech/go/file.nix",
		Status: "M",
		Gh: &GhMap{
			Category: "tech",
			GhDir:    "data/gh/tech",
			GhFiles:  []string{"go.yml"},
		},
	}
	assert.Equal(t, "M", cf.Status)
	assert.NotNil(t, cf.Gh)
	assert.Equal(t, "tech", cf.Gh.Category)
}

func TestMapToGh_WithGHFiles(t *testing.T) {
	// Create a data/gh/<category> dir with YAML files
	dir := t.TempDir()
	ghDir := filepath.Join(dir, "data", "gh", "tech")
	require.NoError(t, os.MkdirAll(ghDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(ghDir, "go.yml"), []byte("- type: lang"), 0o600))

	// Temporarily change working directory for mapToGh
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	gh := mapToGh("home/base/tech/go/file.nix")
	require.NotNil(t, gh)
	assert.Equal(t, "tech", gh.Category)
	assert.Equal(t, "data/gh/tech", gh.GhDir)
	assert.NotEmpty(t, gh.GhFiles)
}

func TestRunSyncRecord_WithGitDir(t *testing.T) {
	// Create a dir with .git - OK will be true but ChangedFiles may be nil
	// since git commands fail in a non-real git repo
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0o700))

	result := RunSyncRecord(SyncRecordOptions{DotfilesPath: dir})
	assert.True(t, result.OK)
	assert.Empty(t, result.Error)
}

func TestGetChangedFiles_InvalidRepo(t *testing.T) {
	dir := t.TempDir()
	// No .git dir, but we call getChangedFiles directly
	files := getChangedFiles(dir)
	assert.Nil(t, files)
}

func TestGetChangedFiles_WithGitDir(t *testing.T) {
	dir := t.TempDir()
	// Create a .git dir but no actual git repo
	gitDir := filepath.Join(dir, ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0o700))

	files := getChangedFiles(dir)
	// Will be nil since there's no actual git repo
	assert.Nil(t, files)
}

func TestRunSyncRecord_WithRealGitRepo(t *testing.T) {
	dir := t.TempDir()

	// Initialize a real git repo
	require.NoError(t, os.Chdir(dir))
	defer func() { _ = os.Chdir(os.TempDir()) }()

	// Use git init
	cmd := exec.Command("git", "init", dir)
	require.NoError(t, cmd.Run())

	// Create home/base structure
	baseDir := filepath.Join(dir, "home", "base", "tech")
	require.NoError(t, os.MkdirAll(baseDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(baseDir, "config.nix"), []byte("{}"), 0o600))

	// Create data/gh structure
	ghDir := filepath.Join(dir, "data", "gh", "tech")
	require.NoError(t, os.MkdirAll(ghDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(ghDir, "go.yml"), []byte("- type: lang"), 0o600))

	// Stage and commit
	require.NoError(t, exec.Command("git", "-C", dir, "add", ".").Run())
	require.NoError(t, exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run())
	require.NoError(t, exec.Command("git", "-C", dir, "config", "user.name", "test").Run())
	require.NoError(t, exec.Command("git", "-C", dir, "commit", "-m", "init").Run())

	// Modify a file to create a change
	require.NoError(t, os.WriteFile(filepath.Join(baseDir, "config.nix"), []byte("{ modified = true; }"), 0o600))

	result := RunSyncRecord(SyncRecordOptions{DotfilesPath: dir})
	assert.True(t, result.OK)
	assert.Empty(t, result.Error)
	assert.NotEmpty(t, result.ChangedFiles)
}

func TestMapToGh_SinglePartPath(t *testing.T) {
	// "home/base/" with nothing after → empty parts
	gh := mapToGh("home/base/")
	assert.Nil(t, gh)
}

func TestSyncRecordResult_EmptyChangedFiles(t *testing.T) {
	result := &SyncRecordResult{
		DotfilesPath: "/tmp/test",
		OK:           true,
		ChangedFiles: nil,
	}
	assert.True(t, result.OK)
	assert.Nil(t, result.ChangedFiles)
}

func TestGhMap_Fields(t *testing.T) {
	gh := &GhMap{
		Category: "tech",
		GhDir:    "data/gh/tech",
		GhFiles:  []string{"go.yml", "python.yml"},
	}
	assert.Equal(t, "tech", gh.Category)
	assert.Equal(t, "data/gh/tech", gh.GhDir)
	assert.Len(t, gh.GhFiles, 2)
}
