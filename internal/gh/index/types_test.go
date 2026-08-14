package ghindex

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xbpk3t/docs-alfred/internal/gh/model"
)

func TestRepository_IsValid(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{"valid github", "https://github.com/owner/repo", true},
		{"invalid url", "https://example.com/owner/repo", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Repository{Repo: model.Repo{URL: tt.url}}
			assert.Equal(t, tt.want, IsValid(r))
		})
	}
}

func TestRepository_FullName(t *testing.T) {
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
			r := &Repository{Repo: model.Repo{URL: tt.url}}
			assert.Equal(t, tt.want, FullName(r))
		})
	}
}

func TestRepository_GetDes(t *testing.T) {
	r := &Repository{Repo: model.Repo{Des: strptr("test description")}}
	assert.Equal(t, "test description", GetDes(r))

	nilDes := &Repository{}
	assert.Empty(t, GetDes(nilDes))
}

func TestRepository_GetURL(t *testing.T) {
	r := &Repository{Repo: model.Repo{URL: "https://github.com/a/b"}}
	assert.Equal(t, "https://github.com/a/b", GetURL(r))
}

func TestRepository_HasNix(t *testing.T) {
	r1 := &Repository{Repo: model.Repo{Nix: strptr("github:acme/repo#pkg")}}
	assert.True(t, HasNix(r1))

	r2 := &Repository{Repo: model.Repo{Nix: strptr("  ")}}
	assert.False(t, HasNix(r2))

	r3 := &Repository{}
	assert.False(t, HasNix(r3))
}

func TestRepository_HasSubRepos(t *testing.T) {
	r1 := &Repository{Repo: model.Repo{Rel: []model.Repo{{URL: "https://github.com/a/b"}}}}
	assert.True(t, HasSubRepos(r1))

	r2 := &Repository{}
	assert.False(t, HasSubRepos(r2))
}

func TestRepository_IsSubOrDepOrRelRepo(t *testing.T) {
	r1 := &Repository{IsRelatedRepo: true}
	assert.True(t, IsSubOrDepOrRelRepo(r1))

	r2 := &Repository{}
	assert.False(t, IsSubOrDepOrRelRepo(r2))
}
