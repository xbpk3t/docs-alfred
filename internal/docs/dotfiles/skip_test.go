package dotfiles

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsSkip_NixBuiltins(t *testing.T) {
	for _, name := range []string{"true", "false", "null", "if", "then", "else", "let", "in", "rec", "with", "inherit", "import", "pkgs", "lib", "config", "options", "types"} {
		assert.True(t, isSkip(name), "expected isSkip(%q) = true", name)
	}
}

func TestIsSkip_NixLibFuncs(t *testing.T) {
	for _, name := range []string{"mkIf", "mkForce", "optionals", "optional", "mkDefault", "inputs", "outputs", "self", "super"} {
		assert.True(t, isSkip(name), "expected isSkip(%q) = true", name)
	}
}

func TestIsSkip_NixSkip(t *testing.T) {
	for _, name := range []string{"stdenv", "callPackage", "fetchurl", "buildGoModule", "override", "logind", "meta", "name", "version"} {
		assert.True(t, isSkip(name), "expected isSkip(%q) = true", name)
	}
}

func TestIsSkip_RealPackages(t *testing.T) {
	for _, name := range []string{"gcc", "coreutils", "bash", "mpv", "pipewire", "home-manager", "gpg", "ssh", "firefox", "vim", "git"} {
		assert.False(t, isSkip(name), "expected isSkip(%q) = false", name)
	}
}

func TestIsSkip_Empty(t *testing.T) {
	assert.False(t, isSkip(""))
}

func TestIsSkip_Unknown(t *testing.T) {
	assert.False(t, isSkip("some-random-package"))
}
