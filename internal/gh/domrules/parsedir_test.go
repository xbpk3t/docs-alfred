package domrules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseYAMLDir_ValidFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.yml"), []byte("- name: test\n  score: 4\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.yml"), []byte("- name: test2\n  score: 3\n"), 0644))

	count, errs := ParseYAMLDir(dir)
	assert.Equal(t, 2, count)
	assert.Empty(t, errs)
}

func TestParseYAMLDir_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.yml"), []byte("invalid: [yaml: broken\n"), 0644))

	count, errs := ParseYAMLDir(dir)
	assert.Equal(t, 0, count)
	assert.NotEmpty(t, errs)
}

func TestParseYAMLDir_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	count, errs := ParseYAMLDir(dir)
	assert.Equal(t, 0, count)
	assert.Empty(t, errs)
}

func TestParseYAMLDir_MixedFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "good.yml"), []byte("- name: test\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.yml"), []byte("invalid: [yaml:\n"), 0644))

	count, errs := ParseYAMLDir(dir)
	assert.Equal(t, 1, count)
	assert.Len(t, errs, 1)
}

func TestParseYAMLDir_NonExistentDir(t *testing.T) {
	count, errs := ParseYAMLDir("/tmp/nonexistent-dir-parse-99999")
	assert.Equal(t, 0, count)
	assert.Len(t, errs, 1)
}

func TestParseYAMLDir_MultiDoc(t *testing.T) {
	dir := t.TempDir()
	content := "---\n- name: doc1\n---\n- name: doc2\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "multi.yml"), []byte(content), 0644))

	count, errs := ParseYAMLDir(dir)
	assert.Equal(t, 1, count)
	assert.Empty(t, errs)
}

func TestParseYAMLFile_ReadError(t *testing.T) {
	err := parseYAMLFile("/tmp/nonexistent-parse-file-99999.yml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read error")
}

func TestParseYAMLFile_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "empty.yml")
	require.NoError(t, os.WriteFile(file, []byte(""), 0644))

	err := parseYAMLFile(file)
	require.NoError(t, err)
}
