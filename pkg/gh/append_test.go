package gh

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

func TestAppendRecord_InvalidDate(t *testing.T) {
	result, err := AppendRecord(&AppendRecordOptions{
		URL:  "https://github.com/owner/repo",
		Date: "not-a-date",
		Des:  "test record",
	})
	require.Error(t, err, "expected error for invalid date")
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid date format")
}

func TestAppendRecord_NoFileOrURL(t *testing.T) {
	// When no URL or File is provided, AppendRecord will call findFileByURL
	// with an empty URL. Since we didn't provide a file, this should fail.
	result, err := AppendRecord(&AppendRecordOptions{
		Date: "2024-01-01",
		Des:  "test record",
	})
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestAppendRecord_NoFileFound(t *testing.T) {
	// With a URL that doesn't match any file but no explicit file path
	result, err := AppendRecord(&AppendRecordOptions{
		URL:  "https://github.com/nonexistent/repo-that-does-not-exist-12345",
		Date: "2024-01-01",
		Des:  "test record",
	})
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "no file contains URL")
}

func TestFindFileByURL_NonExistent(t *testing.T) {
	found, err := findFileByURL("/tmp/nonexistent-gh-dir-12345", "https://github.com/owner/repo")
	require.Error(t, err)
	assert.Empty(t, found)
}

func TestFindFileByURL_EmptyURL(t *testing.T) {
	found, err := findFileByURL("/tmp/nonexistent-gh-dir-12345", "")
	require.Error(t, err)
	assert.Empty(t, found)
}

func TestValidateDateStrict(t *testing.T) {
	assert.True(t, checkutil.DateFullPattern.MatchString("2024-01-01"))
	assert.True(t, checkutil.DateFullPattern.MatchString("2023-12-31"))
	assert.False(t, checkutil.DateFullPattern.MatchString("2024-1-1"))
	assert.False(t, checkutil.DateFullPattern.MatchString("2024/01/01"))
	assert.False(t, checkutil.DateFullPattern.MatchString("not-a-date"))
	assert.False(t, checkutil.DateFullPattern.MatchString(""))
	assert.False(t, checkutil.DateFullPattern.MatchString("240101"))
}

func TestInferTopicFromURL(t *testing.T) {
	tests := []struct {
		url      string
		expected string
	}{
		{"https://github.com/owner/repo-name", "repo-name"},
		{"https://github.com/owner/repo-name/", "repo-name"},
		{"https://github.com/a/b/c/d", "d"},
		{"", ""},
	}
	for _, tt := range tests {
		result := inferTopicFromURL(tt.url)
		assert.Equal(t, tt.expected, result, "inferTopicFromURL(%q)", tt.url)
	}
}
