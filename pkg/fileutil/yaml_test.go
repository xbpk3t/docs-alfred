package fileutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListYAMLFiles(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "a.yml"))
	writeTestFile(t, filepath.Join(dir, "b.yaml"))
	writeTestFile(t, filepath.Join(dir, "c.txt"))
	writeTestFile(t, filepath.Join(dir, ".hidden.yml"))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "subdir"), DirPerm))
	writeTestFile(t, filepath.Join(dir, "subdir", "nested.yml"))

	files, err := ListYAMLFiles(dir)
	require.NoError(t, err)

	want := []string{filepath.Join(dir, "a.yml"), filepath.Join(dir, "b.yaml")}
	require.Equal(t, want, files)
}

func TestListYAMLFilesRecursive(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "a.yml"))
	writeTestFile(t, filepath.Join(dir, ".hidden.yml"))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "subdir"), DirPerm))
	writeTestFile(t, filepath.Join(dir, "subdir", "nested.yaml"))

	files, err := ListYAMLFilesRecursive(dir)
	require.NoError(t, err)

	want := []string{filepath.Join(dir, "a.yml"), filepath.Join(dir, "subdir", "nested.yaml")}
	require.Equal(t, want, files)
}

func TestReadAndMergeYAMLFilesRecursiveUsesVisibleSortedYAMLFiles(t *testing.T) {
	dir := t.TempDir()
	writeContent(t, filepath.Join(dir, "b.yaml"), "b: true\n")
	writeContent(t, filepath.Join(dir, ".hidden.yml"), "hidden: true\n")
	writeContent(t, filepath.Join(dir, "note.txt"), "note: true\n")
	require.NoError(t, os.Mkdir(filepath.Join(dir, "subdir"), DirPerm))
	writeContent(t, filepath.Join(dir, "subdir", "a.yml"), "a: true\n")

	var seen []string
	data, err := ReadAndMergeYAMLFilesRecursive(dir, func(filename string) {
		seen = append(seen, filename)
	})
	require.NoError(t, err)
	require.Equal(t, "b: true\na: true\n", string(data))
	require.Equal(t, []string{"b.yaml", "a.yml"}, seen)
	require.False(t, strings.Contains(string(data), "hidden"), "unexpected hidden file content")
	require.False(t, strings.Contains(string(data), "note"), "unexpected note file content")
}

func writeTestFile(t *testing.T, path string) {
	t.Helper()
	writeContent(t, path, "key: value\n")
}

func writeContent(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), FilePermPrivate))
}
