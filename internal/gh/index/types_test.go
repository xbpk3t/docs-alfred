package ghindex

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xbpk3t/docs-alfred/internal/gh/model/gh"
)

func TestRepo_FullName(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"valid", "https://github.com/owner/repo", "owner/repo"},
		{"invalid", "https://example.com/a/b", ""},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Repo{Repo: gh.Repo{URL: tt.url}}
			assert.Equal(t, tt.want, FullName(r))
		})
	}
}

func TestRepo_GetDes(t *testing.T) {
	r := &Repo{Repo: gh.Repo{Des: strptr("test description")}}
	assert.Equal(t, "test description", GetDes(r))

	nilDes := &Repo{}
	assert.Empty(t, GetDes(nilDes))
}

func TestRepo_GetURL(t *testing.T) {
	r := &Repo{Repo: gh.Repo{URL: "https://github.com/a/b"}}
	assert.Equal(t, "https://github.com/a/b", GetURL(r))
}

func TestRepo_HasNix(t *testing.T) {
	r1 := &Repo{Repo: gh.Repo{Nix: strptr("github:acme/repo#pkg")}}
	assert.True(t, HasNix(r1))

	r2 := &Repo{Repo: gh.Repo{Nix: strptr("  ")}}
	assert.False(t, HasNix(r2))

	r3 := &Repo{}
	assert.False(t, HasNix(r3))
}
