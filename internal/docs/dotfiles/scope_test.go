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

func TestDefaultScope(t *testing.T) {
	scope := DefaultScope()
	expected := []string{
		"home/base", "home/core",
		"modules/nixos", "modules/darwin",
		"home/darwin", "home/nixos", "home/extra",
	}
	assert.Equal(t, expected, scope)
}
