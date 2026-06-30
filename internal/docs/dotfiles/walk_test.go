package dotfiles

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWalkNixFiles_Basic(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "home/base/utils"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "home/base/utils/default.nix"),
		[]byte(`{ pkgs, ... }: { environment.systemPackages = [ pkgs.curl pkgs.git ]; }`),
		0o600,
	))

	var got []struct{ cat, pkg string }
	err := WalkNixFiles(dir, []string{"home/base"}, func(cat, pkg string) error {
		got = append(got, struct{ cat, pkg string }{cat, pkg})
		return nil
	})
	require.NoError(t, err)
	assert.Len(t, got, 2)

	cats := map[string][]string{}
	for _, g := range got {
		cats[g.cat] = append(cats[g.cat], g.pkg)
	}
	sort.Strings(cats["utils"])
	assert.Equal(t, []string{"curl", "git"}, cats["utils"])
}

func TestWalkNixFiles_MultipleScopes(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "home/base/a"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "home/core/b"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "home/base/a/default.nix"),
		[]byte(`{ pkgs, ... }: [ pkgs.curl ]`),
		0o600,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "home/core/b/default.nix"),
		[]byte(`{ pkgs, ... }: [ pkgs.wget ]`),
		0o600,
	))

	var pkgs []string
	err := WalkNixFiles(dir, []string{"home/base", "home/core"}, func(cat, pkg string) error {
		pkgs = append(pkgs, pkg)
		return nil
	})
	require.NoError(t, err)
	sort.Strings(pkgs)
	assert.Equal(t, []string{"curl", "wget"}, pkgs)
}

func TestWalkNixFiles_NonExistentScope(t *testing.T) {
	dir := t.TempDir()
	err := WalkNixFiles(dir, []string{"nonexistent"}, func(cat, pkg string) error {
		t.Fatal("should not be called")
		return nil
	})
	assert.NoError(t, err)
}

func TestWalkNixFiles_SkipsNonNixFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "home/base/test"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "home/base/test/readme.md"), []byte("hello"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "home/base/test/config.nix"), []byte(`{ pkgs, ... }: [ pkgs.hello ]`), 0o600))

	var pkgs []string
	err := WalkNixFiles(dir, []string{"home/base"}, func(cat, pkg string) error {
		pkgs = append(pkgs, pkg)
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"hello"}, pkgs)
}

func TestWalkNixFiles_FnError(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "home/base/test"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "home/base/test/default.nix"),
		[]byte(`{ pkgs, ... }: [ pkgs.curl pkgs.wget ]`),
		0o600,
	))

	testErr := errors.New("stop")
	err := WalkNixFiles(dir, []string{"home/base"}, func(cat, pkg string) error {
		return testErr
	})
	assert.ErrorIs(t, err, testErr)
}

func TestWalkNixFiles_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "home/base/empty"), 0o755))

	var called bool
	err := WalkNixFiles(dir, []string{"home/base"}, func(cat, pkg string) error {
		called = true
		return nil
	})
	require.NoError(t, err)
	assert.False(t, called)
}

func TestWalkNixFiles_ProgramsAndServices(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "home/base/sys"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "home/base/sys/default.nix"),
		[]byte(`{ ... }: { programs.git.enable = true; services.nginx.enable = true; }`),
		0o600,
	))

	var pkgs []string
	err := WalkNixFiles(dir, []string{"home/base"}, func(cat, pkg string) error {
		pkgs = append(pkgs, pkg)
		return nil
	})
	require.NoError(t, err)
	sort.Strings(pkgs)
	assert.Equal(t, []string{"git", "nginx"}, pkgs)
}
