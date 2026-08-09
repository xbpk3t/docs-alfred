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
	// coreutils / home-manager / ssh were added to the skip list in 92d14e2
	// as config/scope keys — they are intentionally "skip" now.
	for _, name := range []string{"gcc", "bash", "mpv", "pipewire", "gpg", "firefox", "vim", "git"} {
		assert.False(t, isSkip(name), "expected isSkip(%q) = false", name)
	}
	for _, name := range []string{"coreutils", "home-manager", "ssh"} {
		assert.True(t, isSkip(name), "expected isSkip(%q) = true (config/scope key)", name)
	}
}

func TestIsSkip_Empty(t *testing.T) {
	assert.False(t, isSkip(""))
}

func TestIsSkip_Unknown(t *testing.T) {
	assert.False(t, isSkip("some-random-package"))
}
