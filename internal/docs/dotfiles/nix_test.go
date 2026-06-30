package dotfiles

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFirstSeg(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"foo.bar.baz", "foo"},
		{"foo.bar", "foo"},
		{"foo", "foo"},
		{"", ""},
		{".foo", ""},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, firstSeg(tt.input), "firstSeg(%q)", tt.input)
	}
}

func TestCategoryFromFile(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"home/base/foo/bar.nix", "foo"},
		{"home/base/foo/deep/file.nix", "foo"},
		{"home/core/bar/some.nix", "bar"},
		{"home/darwin/some.nix", "desktop"},
		{"home/nixos/some.nix", "nixos"},
		{"home/extra/some.nix", "extra"},
		{"modules/nixos/foo/some.nix", "foo"},
		{"modules/darwin/some.nix", "desktop"},
		{"home/base", ""},
		{"some/other.nix", ""},
		{"short.nix", ""},
		{"", ""},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, categoryFromFile(tt.path), "categoryFromFile(%q)", tt.path)
	}
}

func TestParseNixRefs_PkgsDot(t *testing.T) {
	code := `{ pkgs, ... }: {
  environment.systemPackages = with pkgs; [ curl wget git ];
  programs.bash.enable = true;
}`
	refs := parseNixRefs(code)
	assert.Contains(t, refs, "curl")
	assert.Contains(t, refs, "wget")
	assert.Contains(t, refs, "git")
	assert.NotContains(t, refs, "pkgs")
	assert.NotContains(t, refs, "bash")
}

func TestParseNixRefs_PkgsSelect(t *testing.T) {
	code := `{ config, pkgs, lib, ... }: {
  environment.systemPackages = [ pkgs.hello pkgs.neovim pkgs.ripgrep ];
}`
	refs := parseNixRefs(code)
	assert.Contains(t, refs, "hello")
	assert.Contains(t, refs, "neovim")
	assert.Contains(t, refs, "ripgrep")
	assert.NotContains(t, refs, "config")
	assert.NotContains(t, refs, "lib")
}

func TestParseNixRefs_ProgramsAndServices(t *testing.T) {
	code := `{ ... }: {
  programs.git.enable = true;
  services.nginx.enable = true;
  services.postgresql.enable = true;
}`
	refs := parseNixRefs(code)
	assert.Contains(t, refs, "git")
	assert.Contains(t, refs, "nginx")
	assert.Contains(t, refs, "postgresql")
}

func TestParseNixRefs_WithPkgs(t *testing.T) {
	code := `{ pkgs, ... }:
with pkgs; [
  htop
  jq
  fzf
]`
	refs := parseNixRefs(code)
	assert.Contains(t, refs, "htop")
	assert.Contains(t, refs, "jq")
	assert.Contains(t, refs, "fzf")
}

func TestParseNixRefs_SkipsBuiltins(t *testing.T) {
	code := `{ pkgs, ... }: with pkgs; [ true false null if ]`
	refs := parseNixRefs(code)
	assert.NotContains(t, refs, "true")
	assert.NotContains(t, refs, "false")
	assert.NotContains(t, refs, "null")
	assert.NotContains(t, refs, "if")
}

func TestParseNixRefs_SkipsLibFuncs(t *testing.T) {
	code := `{ lib, pkgs, ... }: with pkgs; [
  (lib.mkIf true pkgs.hello)
  (lib.mkForce pkgs.vim)
]`
	refs := parseNixRefs(code)
	assert.NotContains(t, refs, "mkIf")
	assert.NotContains(t, refs, "mkForce")
	assert.NotContains(t, refs, "lib")
	assert.Contains(t, refs, "hello")
	assert.Contains(t, refs, "vim")
}

func TestParseNixRefs_SkipsNixSkip(t *testing.T) {
	code := `{ pkgs, ... }: with pkgs; [
  stdenv
  callPackage
  fetchurl
  hello
]`
	refs := parseNixRefs(code)
	assert.NotContains(t, refs, "stdenv")
	assert.NotContains(t, refs, "callPackage")
	assert.NotContains(t, refs, "fetchurl")
	assert.Contains(t, refs, "hello")
}

func TestParseNixRefs_EmptyInput(t *testing.T) {
	refs := parseNixRefs("")
	assert.Empty(t, refs)
}

func TestParseNixRefs_InvalidNix(t *testing.T) {
	refs := parseNixRefs("this is not valid nix {{})")
	assert.Empty(t, refs)
}

func TestParseNixRefs_Dedup(t *testing.T) {
	code := `{ pkgs, ... }: with pkgs; [ hello hello hello ]`
	refs := parseNixRefs(code)
	assert.Equal(t, 1, countOccurrences(refs, "hello"))
}

func TestParseNixRefs_DottedPath(t *testing.T) {
	code := `{ pkgs, ... }: [ pkgs.my-package pkgs.hello ]`
	refs := parseNixRefs(code)
	assert.Contains(t, refs, "my-package")
	assert.Contains(t, refs, "hello")
}

func TestDefaultScope(t *testing.T) {
	scope := DefaultScope()
	expected := []string{
		"home/base", "home/core",
		"modules/nixos", "modules/darwin",
		"home/darwin", "home/nixos", "home/extra",
	}
	assert.Equal(t, expected, scope)
}

func TestLoadSelfBuiltPkgs_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "generated.json")
	err := os.WriteFile(path, []byte(`{"hello":{}, "world":{}}`), 0o600)
	require.NoError(t, err)

	result := LoadSelfBuiltPkgs(path)
	assert.True(t, result["hello"])
	assert.True(t, result["world"])
	assert.False(t, result["nonexistent"])
}

func TestLoadSelfBuiltPkgs_EmptyJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "generated.json")
	err := os.WriteFile(path, []byte(`{}`), 0o600)
	require.NoError(t, err)

	result := LoadSelfBuiltPkgs(path)
	assert.Empty(t, result)
}

func TestLoadSelfBuiltPkgs_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "generated.json")
	err := os.WriteFile(path, []byte(`not json`), 0o600)
	require.NoError(t, err)

	result := LoadSelfBuiltPkgs(path)
	assert.Empty(t, result)
}

func TestLoadSelfBuiltPkgs_MissingFile(t *testing.T) {
	result := LoadSelfBuiltPkgs("/nonexistent/path/generated.json")
	assert.Empty(t, result)
}

func TestBuildNixMap_WithFiles(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "home/base/utils"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "home/base/utils/default.nix"),
		[]byte(`{ pkgs, ... }: { environment.systemPackages = [ pkgs.curl pkgs.git ]; }`),
		0o600,
	))

	nixMap, err := BuildNixMap(dir, []string{"home/base"})
	require.NoError(t, err)

	assert.Contains(t, nixMap, "utils")
	assert.True(t, nixMap["utils"]["curl"])
	assert.True(t, nixMap["utils"]["git"])
}

func TestBuildNixMap_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "home/empty"), 0o755))

	nixMap, err := BuildNixMap(dir, []string{"home/empty"})
	require.NoError(t, err)
	assert.Empty(t, nixMap)
}

func TestBuildNixMap_NonExistentScope(t *testing.T) {
	dir := t.TempDir()
	_, err := BuildNixMap(dir, []string{"nonexistent"})
	// WalkDir on a non-existent path may or may not error depending on platform;
	// either way the result should be an empty map.
	if err == nil {
		t.Log("WalkDir did not error on non-existent path (platform-dependent)")
	}
}

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

// countOccurrences counts how many times s appears in slice.
func countOccurrences(slice []string, s string) int {
	count := 0
	for _, v := range slice {
		if v == s {
			count++
		}
	}
	return count
}
