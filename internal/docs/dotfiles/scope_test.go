package dotfiles

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCategoryFromFile(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"home/base/foo/bar.nix", "foo"},
		{"home/base/foo/deep/file.nix", "foo"},
		{"home/core/bar/some.nix", "bar"},
		{"home/darwin/some.nix", "desktop"},
		{"home/nixos/some.nix", "desktop"},
		// home/extra was dropped from scopeMap in 92d14e2 (merge home scopes).
		{"home/extra/some.nix", ""},
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

func TestDefaultScope(t *testing.T) {
	scope := DefaultScope()
	expected := []string{
		"home/base", "home/core",
		"modules/nixos", "modules/darwin",
		"home/darwin", "home/nixos",
	}
	assert.Equal(t, expected, scope)
}
