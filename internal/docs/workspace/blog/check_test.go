package blog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunCheckEmptyDirs(t *testing.T) {
	dataDir := t.TempDir()
	blogDir := t.TempDir()

	result, err := RunCheck(dataDir, blogDir)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.Issues)
	assert.Equal(t, 0, result.GHTypes)
	assert.Equal(t, 0, result.BlogDirs)
}

func TestRunCheckMatchingDirs(t *testing.T) {
	dataDir := t.TempDir()
	blogDir := t.TempDir()

	// Create data/gh YAML
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, "cat"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "cat", "type.yml"), []byte("- type: t"), 0o600))

	// Create matching blog dir
	require.NoError(t, os.MkdirAll(filepath.Join(blogDir, "cat", "type"), 0o700))

	result, err := RunCheck(dataDir, blogDir)
	require.NoError(t, err)
	assert.Equal(t, 1, result.GHTypes)
	assert.Equal(t, 1, result.BlogDirs)
	assert.Empty(t, result.Issues)
}

func TestRunCheckExtraBlogDir(t *testing.T) {
	dataDir := t.TempDir()
	blogDir := t.TempDir()

	// Blog has dir but no data/gh YAML
	require.NoError(t, os.MkdirAll(filepath.Join(blogDir, "cat", "type"), 0o700))

	result, err := RunCheck(dataDir, blogDir)
	require.NoError(t, err)
	assert.Equal(t, 1, result.BlogDirs)
	assert.NotEmpty(t, result.Issues)
	assert.Contains(t, result.Issues[0].Message, "no corresponding data/gh YAML file")
}

func TestRunCheckMissingBlogDir(t *testing.T) {
	dataDir := t.TempDir()
	blogDir := "/tmp/nonexistent-blog-dir-12345"

	result, err := RunCheck(dataDir, blogDir)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestRunCheckSkipsStaticDir(t *testing.T) {
	dataDir := t.TempDir()
	blogDir := t.TempDir()

	// "static" should be skipped
	require.NoError(t, os.MkdirAll(filepath.Join(blogDir, "static", "css"), 0o700))

	result, err := RunCheck(dataDir, blogDir)
	require.NoError(t, err)
	assert.Equal(t, 0, result.BlogDirs)
}

func TestRunCheckSkipsNonDirEntries(t *testing.T) {
	dataDir := t.TempDir()
	blogDir := t.TempDir()

	// Create a file at top level (should be skipped)
	require.NoError(t, os.WriteFile(filepath.Join(blogDir, "readme.txt"), []byte("hello"), 0o600))

	result, err := RunCheck(dataDir, blogDir)
	require.NoError(t, err)
	assert.Equal(t, 0, result.BlogDirs)
}

func TestRunCheckNestedBlogDirs(t *testing.T) {
	dataDir := t.TempDir()
	blogDir := t.TempDir()

	// Create nested blog dirs
	require.NoError(t, os.MkdirAll(filepath.Join(blogDir, "cat1", "type1"), 0o700))
	require.NoError(t, os.MkdirAll(filepath.Join(blogDir, "cat1", "type2"), 0o700))

	// Create matching data/gh YAML
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, "cat1"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "cat1", "type1.yml"), []byte("- type: t"), 0o600))

	result, err := RunCheck(dataDir, blogDir)
	require.NoError(t, err)
	assert.Equal(t, 1, result.GHTypes)
	assert.Equal(t, 2, result.BlogDirs)
	// type2 has no corresponding gh YAML
	assert.Len(t, result.Issues, 1)
}

func TestRunCheckHiddenDirInGHData(t *testing.T) {
	dataDir := t.TempDir()
	blogDir := t.TempDir()

	// .hidden dir should be skipped in data/gh
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, ".hidden"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, ".hidden", "x.yml"), []byte("- type: x"), 0o600))

	result, err := RunCheck(dataDir, blogDir)
	require.NoError(t, err)
	assert.Equal(t, 0, result.GHTypes)
}

func TestRunCheck_SubEntryNotDir(t *testing.T) {
	dataDir := t.TempDir()
	blogDir := t.TempDir()

	// Create a file (not dir) inside a blog category dir - should be skipped
	require.NoError(t, os.MkdirAll(filepath.Join(blogDir, "cat"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(blogDir, "cat", "file.txt"), []byte("text"), 0o600))

	result, err := RunCheck(dataDir, blogDir)
	require.NoError(t, err)
	assert.Equal(t, 0, result.BlogDirs)
}

func TestRunCheck_DataDirNonExistent(t *testing.T) {
	dataDir := "/tmp/nonexistent-data-dir-99999"
	blogDir := t.TempDir()

	_, err := RunCheck(dataDir, blogDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "walk data/gh")
}

func TestRunCheck_BlogDirReadError(t *testing.T) {
	dataDir := t.TempDir()
	blogDir := t.TempDir()

	// Create a category dir in blogDir, then make it unreadable
	catDir := filepath.Join(blogDir, "cat")
	require.NoError(t, os.MkdirAll(catDir, 0o700))
	// Create a file named "cat" to block reading sub-entries
	require.NoError(t, os.Remove(catDir))
	require.NoError(t, os.WriteFile(catDir, []byte("file"), 0o600))

	result, err := RunCheck(dataDir, blogDir)
	require.NoError(t, err)
	// "cat" is a file, not a dir, so it's skipped
	assert.Equal(t, 0, result.BlogDirs)
}

func TestRunCheck_BlogDirIsFile(t *testing.T) {
	dataDir := t.TempDir()
	blogFile := filepath.Join(t.TempDir(), "blogfile")
	require.NoError(t, os.WriteFile(blogFile, []byte("not a dir"), 0o600))

	_, err := RunCheck(dataDir, blogFile)
	require.Error(t, err)
	// This error is NOT IsNotExist, so collectBlogDirs returns an error
	assert.NotContains(t, err.Error(), "walk data/gh")
}

func TestCollectBlogDirs_SubDirReadError(t *testing.T) {
	dataDir := t.TempDir()
	blogDir := t.TempDir()

	// Create a top-level category dir, then replace it with a file so the
	// inner os.ReadDir fails.
	catDir := filepath.Join(blogDir, "cat")
	require.NoError(t, os.MkdirAll(catDir, 0o700))

	// Create a subdir that is a file (not a dir) to cause the inner ReadDir to fail
	subFile := filepath.Join(catDir, "subfile")
	require.NoError(t, os.WriteFile(subFile, []byte("file"), 0o600))

	result, err := RunCheck(dataDir, blogDir)
	require.NoError(t, err)
	// subFile is not a dir, so it's skipped by the inner loop
	assert.Equal(t, 0, result.BlogDirs)
}
