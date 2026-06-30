package dotfiles

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDedupRef_NoDups(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "home/base/cat1"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "home/base/cat1/default.nix"),
		[]byte(`{ pkgs, ... }: [ pkgs.uniq1 pkgs.uniq2 ]`),
		0o600,
	))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "home/base/cat2"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "home/base/cat2/default.nix"),
		[]byte(`{ pkgs, ... }: [ pkgs.uniq3 ]`),
		0o600,
	))

	dups, err := DedupRef(dir, []string{"home/base"})
	require.NoError(t, err)
	assert.Empty(t, dups)
}

func TestDedupRef_WithDups(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "home/base/cat1"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "home/base/cat1/default.nix"),
		[]byte(`{ pkgs, ... }: [ pkgs.shared ]`),
		0o600,
	))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "home/base/cat2"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "home/base/cat2/default.nix"),
		[]byte(`{ pkgs, ... }: [ pkgs.shared ]`),
		0o600,
	))

	dups, err := DedupRef(dir, []string{"home/base"})
	require.NoError(t, err)
	assert.Contains(t, dups, "shared")
}
