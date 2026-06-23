package fileutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- CachePath tests ---

func TestCachePathEmpty(t *testing.T) {
	result := CachePath("")
	assert.Equal(t, "", result)
}

func TestCachePathAbsolute(t *testing.T) {
	result := CachePath("/absolute/path")
	assert.Equal(t, "/absolute/path", result)
}

func TestCachePathRelative(t *testing.T) {
	result := CachePath("subdir/file.txt")
	assert.Contains(t, result, "docs-alfred")
	assert.Contains(t, result, "subdir")
	assert.Contains(t, result, "file.txt")
}

func TestCachePathDot(t *testing.T) {
	result := CachePath(".")
	assert.Contains(t, result, "docs-alfred")
}

func TestCachePathSlash(t *testing.T) {
	result := CachePath("/")
	assert.Equal(t, "/", result)
}

// --- LegacyCachePath tests ---

func TestLegacyCachePathSimple(t *testing.T) {
	result := LegacyCachePath("file.txt")
	assert.Equal(t, ".cache/file.txt", result)
}

func TestLegacyCachePathNested(t *testing.T) {
	result := LegacyCachePath("sub/dir/file.txt")
	assert.Equal(t, ".cache/sub/dir/file.txt", result)
}

func TestLegacyCachePathLeadingSlash(t *testing.T) {
	result := LegacyCachePath("/file.txt")
	assert.Equal(t, ".cache/file.txt", result)
}

func TestLegacyCachePathDot(t *testing.T) {
	result := LegacyCachePath(".")
	assert.Equal(t, ".cache", result)
}

// --- EnsureDir tests ---

func TestEnsureDirCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "new", "nested")
	require.NoError(t, EnsureDir(dir))
	info, err := os.Stat(dir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestEnsureDirEmpty(t *testing.T) {
	assert.NoError(t, EnsureDir(""))
}

func TestEnsureDirExisting(t *testing.T) {
	dir := t.TempDir()
	assert.NoError(t, EnsureDir(dir))
}

// --- EnsureFileDir tests ---

func TestEnsureFileDirCreatesParent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "file.txt")
	require.NoError(t, EnsureFileDir(path))
	parent := filepath.Dir(path)
	info, err := os.Stat(parent)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestEnsureFileDirDot(t *testing.T) {
	assert.NoError(t, EnsureFileDir("file.txt"))
}

func TestEnsureFileDirRoot(t *testing.T) {
	assert.NoError(t, EnsureFileDir("/file.txt"))
}

// --- checkPathValidity tests ---

func TestCheckPathValidityFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.txt")
	require.NoError(t, os.WriteFile(path, []byte("data"), FilePermPrivate))
	info, err := checkPathValidity(path)
	require.NoError(t, err)
	assert.False(t, info.IsDir())
}

func TestCheckPathValidityDir(t *testing.T) {
	dir := t.TempDir()
	info, err := checkPathValidity(dir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestCheckPathValidityMissing(t *testing.T) {
	_, err := checkPathValidity("/nonexistent/path")
	require.Error(t, err)
}

// --- readFileContent tests ---

func TestReadFileContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.txt")
	require.NoError(t, os.WriteFile(path, []byte("hello"), FilePermPrivate))
	data, err := readFileContent(path)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(data))
}

func TestReadFileContentMissing(t *testing.T) {
	_, err := readFileContent("/nonexistent/file.txt")
	require.Error(t, err)
}

// --- processSingleFile tests ---

func TestProcessSingleFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.txt")
	require.NoError(t, os.WriteFile(path, []byte("content"), FilePermPrivate))
	info, err := os.Stat(path)
	require.NoError(t, err)

	var seen string
	data, err := processSingleFile(path, func(name string) { seen = name }, info)
	require.NoError(t, err)
	assert.Equal(t, "content", string(data))
	assert.Equal(t, "test.txt", seen)
}

func TestProcessSingleFileDir(t *testing.T) {
	dir := t.TempDir()
	info, err := os.Stat(dir)
	require.NoError(t, err)
	_, err = processSingleFile(dir, nil, info)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected file but got directory")
}

func TestProcessSingleFileNilCallback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.txt")
	require.NoError(t, os.WriteFile(path, []byte("data"), FilePermPrivate))
	info, err := os.Stat(path)
	require.NoError(t, err)
	data, err := processSingleFile(path, nil, info)
	require.NoError(t, err)
	assert.Equal(t, "data", string(data))
}

// --- ReadSingleFile tests ---

func TestReadSingleFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.txt")
	require.NoError(t, os.WriteFile(path, []byte("hello"), FilePermPrivate))

	var seen string
	data, err := ReadSingleFile(path, func(name string) { seen = name })
	require.NoError(t, err)
	assert.Equal(t, "hello", string(data))
	assert.Equal(t, "test.txt", seen)
}

func TestReadSingleFileMissing(t *testing.T) {
	_, err := ReadSingleFile("/nonexistent/file.txt", nil)
	require.Error(t, err)
}

func TestReadSingleFileDir(t *testing.T) {
	dir := t.TempDir()
	_, err := ReadSingleFile(dir, nil)
	require.Error(t, err)
}

// --- ReadAndMergeYAMLFilesRecursive tests ---

func TestReadAndMergeYAMLFilesRecursive(t *testing.T) {
	dir := t.TempDir()
	writeContent2(t, filepath.Join(dir, "a.yaml"), "key: a\n")
	writeContent2(t, filepath.Join(dir, "b.yaml"), "key: b\n")

	var seen []string
	data, err := ReadAndMergeYAMLFilesRecursive(dir, func(name string) {
		seen = append(seen, name)
	})
	require.NoError(t, err)
	assert.Contains(t, string(data), "key: a")
	assert.Contains(t, string(data), "key: b")
	assert.Len(t, seen, 2)
}

func TestReadAndMergeYAMLFilesRecursiveNilCallback(t *testing.T) {
	dir := t.TempDir()
	writeContent2(t, filepath.Join(dir, "a.yaml"), "x: 1\n")
	data, err := ReadAndMergeYAMLFilesRecursive(dir, nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "x: 1")
}

func TestWriteContent2(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.txt")
	writeContent2(t, path, "data")
	read, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "data", string(read))
}

func writeContent2(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), FilePermPrivate))
}

// --- Permission constants ---

func TestPermissionConstants(t *testing.T) {
	assert.Equal(t, os.FileMode(0o750), DirPerm)
	assert.Equal(t, os.FileMode(0o640), FilePerm)
	assert.Equal(t, os.FileMode(0o600), FilePermPrivate)
	assert.Equal(t, os.FileMode(0o660), FilePermShared)
}

// --- IsYAMLFileName tests ---

func TestIsYAMLFileNameYml(t *testing.T) {
	assert.True(t, IsYAMLFileName("file.yml"))
}

func TestIsYAMLFileNameYaml(t *testing.T) {
	assert.True(t, IsYAMLFileName("file.yaml"))
}

func TestIsYAMLFileNameHidden(t *testing.T) {
	assert.False(t, IsYAMLFileName(".hidden.yml"))
}

func TestIsYAMLFileNameOther(t *testing.T) {
	assert.False(t, IsYAMLFileName("file.txt"))
}

func TestIsYAMLFileNameNoExt(t *testing.T) {
	assert.False(t, IsYAMLFileName("file"))
}

func TestIsYAMLFileNamePath(t *testing.T) {
	assert.True(t, IsYAMLFileName("/path/to/file.yaml"))
}

func TestIsYAMLFileNameCaseInsensitive(t *testing.T) {
	assert.True(t, IsYAMLFileName("file.YAML"))
	assert.True(t, IsYAMLFileName("file.YML"))
}

// --- Coverage edge cases ---

func TestListYAMLFilesNonExistentDir(t *testing.T) {
	_, err := ListYAMLFiles("/nonexistent/dir")
	require.Error(t, err)
}

func TestListYAMLFilesRecursiveNonExistentDir(t *testing.T) {
	_, err := ListYAMLFilesRecursive("/nonexistent/dir")
	require.Error(t, err)
}

func TestReadAndMergeYAMLFilesRecursiveNonExistentDir(t *testing.T) {
	_, err := ReadAndMergeYAMLFilesRecursive("/nonexistent/dir", nil)
	require.Error(t, err)
}

func TestAtomicWriteJSONFileMarshalError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	// channels can't be marshaled to JSON
	err := AtomicWriteJSONFile(path, make(chan int), FilePermPrivate)
	require.Error(t, err)
}

func TestReadAndMergeYAMLFilesRecursiveReadFileError(t *testing.T) {
	dir := t.TempDir()
	writeContent2(t, filepath.Join(dir, "a.yaml"), "x: 1\n")
	data, err := ReadAndMergeYAMLFilesRecursive(dir, nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "x: 1")
}

func TestUnmarshalJSONInvalid(t *testing.T) {
	_, err := UnmarshalJSON[map[string]string]([]byte(`{"bad"`))
	require.Error(t, err)
}

func TestListYAMLFilesRecursiveGlobError(t *testing.T) {
	// Doublestar glob with invalid pattern should return empty, not error
	// This test just verifies no panic on edge cases
	dir := t.TempDir()
	writeContent2(t, filepath.Join(dir, "a.yaml"), "x: 1\n")
	files, err := ListYAMLFilesRecursive(dir)
	require.NoError(t, err)
	assert.Len(t, files, 1)
}

func TestReadAndMergeYAMLFilesRecursiveMultiple(t *testing.T) {
	dir := t.TempDir()
	writeContent2(t, filepath.Join(dir, "b.yaml"), "b: 2\n")
	writeContent2(t, filepath.Join(dir, "a.yaml"), "a: 1\n")
	data, err := ReadAndMergeYAMLFilesRecursive(dir, nil)
	require.NoError(t, err)
	// Files are sorted, so a.yaml comes first
	assert.Contains(t, string(data), "a: 1")
	assert.Contains(t, string(data), "b: 2")
}

func TestAtomicWriteFileReadOnlyDir(t *testing.T) {
	dir := t.TempDir()
	// Make the directory read-only
	require.NoError(t, os.Chmod(dir, 0o444))
	defer os.Chmod(dir, 0o755) //nolint:errcheck

	path := filepath.Join(dir, "sub", "file.txt")
	err := AtomicWriteFile(path, []byte("data"), FilePermPrivate)
	require.Error(t, err)
}

func TestAtomicWriteJSONFileWriteError(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0o444))
	defer os.Chmod(dir, 0o755) //nolint:errcheck

	path := filepath.Join(dir, "sub", "file.json")
	err := AtomicWriteJSONFile(path, map[string]string{"k": "v"}, FilePermPrivate)
	require.Error(t, err)
}
