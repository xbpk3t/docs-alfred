package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunWikiURLs_NonExistentConfig(t *testing.T) {
	err := runWikiURLs("/tmp/nonexistent-config-file-12345.yml", []string{"https://example.com"}, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config file not found")
}

func TestRunWikiURLs_ValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "rss2nl.yml")
	require.NoError(t, os.WriteFile(cfgFile, []byte("rss2nl: test\n"), 0644))

	// No URLs provided should not fail for runWikiURLs
	err := runWikiURLs(cfgFile, nil, "")
	assert.NoError(t, err)
}

func TestRunWikiInbox_NonExistentConfig(t *testing.T) {
	err := runWikiInbox("/tmp/nonexistent-config-file-12345.yml", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config file not found")
}

func TestRunWikiInbox_ValidConfigNoInbox(t *testing.T) {
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "rss2nl.yml")
	require.NoError(t, os.WriteFile(cfgFile, []byte("rss2nl: test\n"), 0644))

	// No inbox file should fail because wiki/inbox.md doesn't exist
	// but wikiRoot defaults to "wiki" which doesn't exist in tmpDir
	err := runWikiInbox(cfgFile, tmpDir)
	require.Error(t, err)
	// Should fail with "inbox file not found" not "config file not found"
	assert.Contains(t, err.Error(), "inbox file not found")
}

func TestRunWikiURLs_EmptyArgs(t *testing.T) {
	// No URLs - should not error, just print "doing nothing"
	// We test this by running runWikiURLs with an empty list
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "rss2nl.yml")
	require.NoError(t, os.WriteFile(cfgFile, []byte("rss2nl: test\n"), 0644))

	err := runWikiURLs(cfgFile, []string{}, "")
	assert.NoError(t, err)
}

func TestDetectContentType(t *testing.T) {
	tests := []struct {
		url      string
		expected contentType
	}{
		{"https://www.youtube.com/watch?v=abc123", contentVideo},
		{"https://bilibili.com/video/BV123", contentVideo},
		{"https://youtu.be/abc123", contentVideo},
		{"https://xiaoyuzhou.fm/episode/123", contentAudio},
		{"https://example.com/article", contentText},
	}
	for _, tt := range tests {
		result := detectContentType(tt.url)
		assert.Equal(t, tt.expected, result, "detectContentType(%q)", tt.url)
	}
}
