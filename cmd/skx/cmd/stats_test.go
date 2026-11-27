package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/cmd/skx/internal/skx"
)

func TestPruneGhosts(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.md"), []byte("x"), 0o644))

	entries := []skx.StatsEntry{
		{Name: "a", Target: "a"},
		{Name: "ghost", Target: "ghost"},
		{Name: "empty"},
	}
	kept, removed, err := pruneGhosts(entries, dir)
	require.NoError(t, err)
	assert.Equal(t, []skx.StatsEntry{{Name: "a", Target: "a"}}, kept)
	assert.ElementsMatch(t, []string{"ghost", "empty"}, removed)
}

func TestPruneGhostsUppercaseExtension(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "analysis"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "analysis", "BRK.MD"), []byte("x"), 0o644))

	kept, removed, err := pruneGhosts([]skx.StatsEntry{{Name: "BRK", Target: "analysis/BRK"}}, dir)
	require.NoError(t, err)
	assert.Len(t, kept, 1)
	assert.Empty(t, removed)
}

func TestPruneGhostsStatErrorAborts(t *testing.T) {
	dir := t.TempDir()
	blocked := filepath.Join(dir, "blocked")
	require.NoError(t, os.MkdirAll(blocked, 0o755))
	require.NoError(t, os.Chmod(blocked, 0o000))
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o755) })
	if _, err := os.Stat(filepath.Join(blocked, "x.md")); err == nil {
		t.Skip("expected a stat error but got none (running as root?)")
	}

	kept, removed, err := pruneGhosts([]skx.StatsEntry{{Name: "x", Target: "blocked/x"}}, dir)
	require.Error(t, err)
	assert.Nil(t, kept)
	assert.Nil(t, removed)
}
