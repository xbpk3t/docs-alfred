package dotfiles

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
	// bash is now tracked as a real package (removed from nixSkip)
	assert.Contains(t, refs, "bash")
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

func TestParseNixRefs_NestedAttrset(t *testing.T) {
	code := `{ ... }: {
  programs = {
    bash = { enable = true; };
    zsh = { enable = true; };
  };
  services = {
    nginx = { enable = true; };
  };
}`
	refs := parseNixRefs(code)
	assert.Contains(t, refs, "bash")
	assert.Contains(t, refs, "zsh")
	// services nested attrset is not extracted (too many false positives from NixOS modules)
	assert.NotContains(t, refs, "nginx")
}

func TestParseNixRefs_DottedPath(t *testing.T) {
	code := `{ pkgs, ... }: [ pkgs.my-package pkgs.hello ]`
	refs := parseNixRefs(code)
	assert.Contains(t, refs, "my-package")
	assert.Contains(t, refs, "hello")
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
