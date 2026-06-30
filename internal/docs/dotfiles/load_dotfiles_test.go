package dotfiles

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadSelfBuiltPkgs_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "generated.json")
	err := os.WriteFile(path, []byte(`{"hello":{}, "world":{}}`), 0o600)
	require.NoError(t, err)

	result, err := LoadSelfBuiltPkgs(path)
	require.NoError(t, err)
	assert.True(t, result["hello"])
	assert.True(t, result["world"])
	assert.False(t, result["nonexistent"])
}

func TestLoadSelfBuiltPkgs_EmptyJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "generated.json")
	err := os.WriteFile(path, []byte(`{}`), 0o600)
	require.NoError(t, err)

	result, err := LoadSelfBuiltPkgs(path)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestLoadSelfBuiltPkgs_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "generated.json")
	err := os.WriteFile(path, []byte(`not json`), 0o600)
	require.NoError(t, err)

	_, err = LoadSelfBuiltPkgs(path)
	assert.Error(t, err)
}

func TestLoadSelfBuiltPkgs_MissingFile(t *testing.T) {
	result, err := LoadSelfBuiltPkgs("/nonexistent/path/generated.json")
	require.NoError(t, err)
	assert.Empty(t, result)
}

// --- LoadDotfilesNixData ---

func TestLoadDotfilesNixData_Basic(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "home/base/utils"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "home/base/utils/default.nix"),
		[]byte(`{ pkgs, ... }: [ pkgs.curl pkgs.git ]`),
		0o600,
	))

	result, err := LoadDotfilesNixData(dir, []string{"home/base"})
	require.NoError(t, err)
	sort.Strings(result["curl"])
	assert.Equal(t, []string{"utils"}, result["curl"])
	assert.Equal(t, []string{"utils"}, result["git"])
}

func TestLoadDotfilesNixData_Empty(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "home/base/empty"), 0o755))

	result, err := LoadDotfilesNixData(dir, []string{"home/base"})
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestLoadDotfilesNixData_NonExistentScope(t *testing.T) {
	dir := t.TempDir()
	result, err := LoadDotfilesNixData(dir, []string{"nonexistent"})
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestLoadDotfilesNixData_MultipleCategories(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "home/base/a"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "home/base/b"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "home/base/a/default.nix"),
		[]byte(`{ pkgs, ... }: [ pkgs.curl ]`),
		0o600,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "home/base/b/default.nix"),
		[]byte(`{ pkgs, ... }: [ pkgs.curl ]`),
		0o600,
	))

	result, err := LoadDotfilesNixData(dir, []string{"home/base"})
	require.NoError(t, err)
	cats := result["curl"]
	sort.Strings(cats)
	assert.Equal(t, []string{"a", "b"}, cats)
}

// --- LoadDotfilesCategories ---

func TestLoadDotfilesCategories_Base(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "home/base/a"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "home/base/b"), 0o755))

	cats, err := LoadDotfilesCategories(dir)
	require.NoError(t, err)
	sort.Strings(cats)
	assert.Equal(t, []string{"a", "b"}, cats)
}

func TestLoadDotfilesCategories_BaseAndCore(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "home/base/a"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "home/core/b"), 0o755))

	cats, err := LoadDotfilesCategories(dir)
	require.NoError(t, err)
	sort.Strings(cats)
	assert.Equal(t, []string{"a", "b"}, cats)
}

func TestLoadDotfilesCategories_Deduplicates(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "home/base/same"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "home/core/same"), 0o755))

	cats, err := LoadDotfilesCategories(dir)
	require.NoError(t, err)
	assert.Equal(t, []string{"same"}, cats)
}

func TestLoadDotfilesCategories_MissingBase(t *testing.T) {
	dir := t.TempDir()
	cats, err := LoadDotfilesCategories(dir)
	require.NoError(t, err)
	assert.Empty(t, cats)
}

func TestLoadDotfilesCategories_SkipsHidden(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "home/base/visible"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "home/base/.hidden"), 0o755))

	cats, err := LoadDotfilesCategories(dir)
	require.NoError(t, err)
	assert.Equal(t, []string{"visible"}, cats)
}
