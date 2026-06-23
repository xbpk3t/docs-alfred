package dotfiles

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

func TestRunCheckMissingBase(t *testing.T) {
	result, err := RunCheck(t.TempDir(), t.TempDir())
	require.NoError(t, err)
	assert.NotEmpty(t, result.Issues)
	assert.Contains(t, result.Issues[0].Message, "home/base not found")
}

func TestRunCheckBaseNotDir(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "home"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(root, "home", "base"), []byte("file"), 0o600))

	result, err := RunCheck(root, t.TempDir())
	require.NoError(t, err)
	assert.NotEmpty(t, result.Issues)
	assert.Contains(t, result.Issues[0].Message, "not a directory")
}

func TestRunCheckSharedCategory(t *testing.T) {
	dotfilesDir := t.TempDir()
	ghDir := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(dotfilesDir, "home", "base", "tech"), 0o700))
	require.NoError(t, os.MkdirAll(filepath.Join(dotfilesDir, "home", "base", "tech", "sub"), 0o700))
	require.NoError(t, os.MkdirAll(filepath.Join(ghDir, "tech"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(ghDir, "tech", "go.yml"), []byte("- isDotfiles: true"), 0o600))

	result, err := RunCheck(dotfilesDir, ghDir)
	require.NoError(t, err)
	assert.Equal(t, 1, result.SharedCount)
}

func TestRunCheckDfOnlyCategory(t *testing.T) {
	dotfilesDir := t.TempDir()
	ghDir := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(dotfilesDir, "home", "base", "dfonly"), 0o700))

	result, err := RunCheck(dotfilesDir, ghDir)
	require.NoError(t, err)
	assert.Equal(t, 1, result.DfOnlyCount)
	assert.NotEmpty(t, result.Issues)
	assert.Contains(t, result.Issues[0].Message, "exists in dotfiles but not in data/gh")
}

func TestRunCheckGhOnlyCategoryWithNoDotfilesFlag(t *testing.T) {
	dotfilesDir := t.TempDir()
	ghDir := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(dotfilesDir, "home", "base"), 0o700))
	require.NoError(t, os.MkdirAll(filepath.Join(ghDir, "ghonly"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(ghDir, "ghonly", "type.yml"), []byte("- isDotfiles: false\n"), 0o600))

	result, err := RunCheck(dotfilesDir, ghDir)
	require.NoError(t, err)
	assert.Equal(t, 1, result.GhOnlyCount)
	// All types marked isDotfiles: false, so no issue
	assert.Empty(t, result.Issues)
}

func TestRunCheckGhOnlyCategoryWithoutNoDotfilesFlag(t *testing.T) {
	dotfilesDir := t.TempDir()
	ghDir := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(dotfilesDir, "home", "base"), 0o700))
	require.NoError(t, os.MkdirAll(filepath.Join(ghDir, "ghonly"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(ghDir, "ghonly", "type.yml"), []byte("- isDotfiles: true\n"), 0o600))

	result, err := RunCheck(dotfilesDir, ghDir)
	require.NoError(t, err)
	assert.Equal(t, 1, result.GhOnlyCount)
	assert.NotEmpty(t, result.Issues)
	assert.Contains(t, result.Issues[0].Message, "exists in data/gh/ but not in dotfiles")
}

func TestRunCheckEmptyBothSides(t *testing.T) {
	dotfilesDir := t.TempDir()
	ghDir := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(dotfilesDir, "home", "base", "empty"), 0o700))
	require.NoError(t, os.MkdirAll(filepath.Join(ghDir, "empty"), 0o700))

	result, err := RunCheck(dotfilesDir, ghDir)
	require.NoError(t, err)
	assert.Equal(t, 1, result.SharedCount)
	// Both sides have the category but no real content
	hasWarn := false
	for _, issue := range result.Issues {
		if issue.Severity == checkutil.SeverityWarn {
			hasWarn = true
		}
	}
	assert.True(t, hasWarn)
}

func TestRunCheckWithCoreDir(t *testing.T) {
	dotfilesDir := t.TempDir()
	ghDir := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(dotfilesDir, "home", "base"), 0o700))
	require.NoError(t, os.MkdirAll(filepath.Join(dotfilesDir, "home", "core", "extra"), 0o700))
	require.NoError(t, os.MkdirAll(filepath.Join(ghDir, "extra"), 0o700))

	result, err := RunCheck(dotfilesDir, ghDir)
	require.NoError(t, err)
	assert.Equal(t, 1, result.SharedCount)
}

func TestRunCheckCoreNotDir(t *testing.T) {
	dotfilesDir := t.TempDir()
	ghDir := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(dotfilesDir, "home", "base"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dotfilesDir, "home", "core"), []byte("file"), 0o600))

	result, err := RunCheck(dotfilesDir, ghDir)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

// --- listSubdirs ---

func TestListSubdirs(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "a"), 0o700))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "b"), 0o700))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".hidden"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte(""), 0o600))

	subs := listSubdirs(dir)
	assert.Contains(t, subs, "a")
	assert.Contains(t, subs, "b")
	assert.NotContains(t, subs, ".hidden")
	assert.NotContains(t, subs, "file.txt")
}

func TestListSubdirsNonExistent(t *testing.T) {
	assert.Nil(t, listSubdirs("/tmp/nonexistent-12345"))
}

// --- hasContentNixFiles ---

func TestHasContentNixFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.nix"), []byte("{}"), 0o600))
	assert.True(t, hasContentNixFiles(dir))
}

func TestHasContentNixFilesOnlyDefault(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "default.nix"), []byte("{}"), 0o600))
	assert.False(t, hasContentNixFiles(dir))
}

func TestHasContentNixFilesEmpty(t *testing.T) {
	assert.False(t, hasContentNixFiles(t.TempDir()))
}

func TestHasContentNixFilesNested(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "test.nix"), []byte("{}"), 0o600))
	assert.True(t, hasContentNixFiles(dir))
}

// --- hasYAMLFiles ---

func TestHasYAMLFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.yml"), []byte("key: val"), 0o600))
	assert.True(t, hasYAMLFiles(dir))
}

func TestHasYAMLFilesEmpty(t *testing.T) {
	assert.False(t, hasYAMLFiles(t.TempDir()))
}

// --- hasAllTypesMarkedNoDotfiles ---

func TestHasAllTypesMarkedNoDotfiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "type.yml"), []byte("- isDotfiles: false\n"), 0o600))
	assert.True(t, hasAllTypesMarkedNoDotfiles(dir))
}

func TestHasAllTypesMarkedNoDotfilesWithTrue(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "type.yml"), []byte("- isDotfiles: true\n"), 0o600))
	assert.False(t, hasAllTypesMarkedNoDotfiles(dir))
}

func TestHasAllTypesMarkedNoDotfilesMissing(t *testing.T) {
	dir := t.TempDir()
	// No YAML files means empty loop → returns true
	assert.True(t, hasAllTypesMarkedNoDotfiles(dir))
}

func TestHasAllTypesMarkedNoDotfilesNonExistentDir(t *testing.T) {
	assert.False(t, hasAllTypesMarkedNoDotfiles("/tmp/nonexistent-dir-12345"))
}

func TestHasAllTypesMarkedNoDotfilesNoField(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "type.yml"), []byte("- name: test\n"), 0o600))
	assert.False(t, hasAllTypesMarkedNoDotfiles(dir))
}

// --- isYAMLFileNoDotfiles ---

func TestIsYAMLFileNoDotfiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.yml")
	require.NoError(t, os.WriteFile(path, []byte("- isDotfiles: false\n"), 0o600))
	assert.True(t, isYAMLFileNoDotfiles(path))
}

func TestIsYAMLFileNoDotfilesTrue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.yml")
	require.NoError(t, os.WriteFile(path, []byte("- isDotfiles: true\n"), 0o600))
	assert.False(t, isYAMLFileNoDotfiles(path))
}

func TestIsYAMLFileNoDotfilesNonExistent(t *testing.T) {
	assert.False(t, isYAMLFileNoDotfiles("/tmp/nonexistent-12345.yml"))
}
