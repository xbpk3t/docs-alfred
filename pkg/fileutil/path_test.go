package fileutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPathWithin(t *testing.T) {
	root := t.TempDir()
	sibling := t.TempDir()
	missing := filepath.Join(filepath.Dir(sibling), "does-not-exist", "file.md")

	tests := []struct {
		name string
		path string
		want bool
	}{
		{"within root", filepath.Join(root, "topic", "file.md"), true}, // leaf may not exist yet
		{"same as root", root, true},
		{"nested path", filepath.Join(root, "a", "b", "c", "d", "file.md"), true},
		{"outside root", filepath.Join(filepath.Dir(root), "elsewhere", "file.md"), false},
		{"similar prefix", filepath.Join(sibling, "file.md"), false},
		{"nonexistent root side", missing, false},
		{"empty path", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, PathWithin(root, tt.path))
		})
	}
}

func TestPathWithinSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "topic"), 0o700))
	outside := t.TempDir()
	link := filepath.Join(root, "topic", "escape")
	require.NoError(t, os.Symlink(outside, link))

	// A symlink inside root must not pass as "within" for its resolved target,
	// even when the final component does not exist yet.
	assert.False(t, PathWithin(root, filepath.Join(link, "file.md")))
	assert.True(t, PathWithin(root, filepath.Join(root, "topic", "file.md")))
}
