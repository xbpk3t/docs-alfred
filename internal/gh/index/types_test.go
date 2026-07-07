package ghindex

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
			r := &Repository{URL: tt.url}
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
			r := &Repository{URL: tt.url}
			assert.Equal(t, tt.want, FullName(r))
		})
	}
}

func TestRepository_GetDes(t *testing.T) {
	r := &Repository{Des: "test description"}
	assert.Equal(t, "test description", GetDes(r))
}

func TestRepository_GetURL(t *testing.T) {
	r := &Repository{URL: "https://github.com/a/b"}
	assert.Equal(t, "https://github.com/a/b", GetURL(r))
}

func TestRepository_HasNix(t *testing.T) {
	r1 := &Repository{NixURL: "github:acme/repo#pkg"}
	assert.True(t, HasNix(r1))

	r2 := &Repository{NixURL: "  "}
	assert.False(t, HasNix(r2))

	r3 := &Repository{}
	assert.False(t, HasNix(r3))
}

func TestRepository_HasSubRepos(t *testing.T) {
	r1 := &Repository{RelatedRepos: Repos{{URL: "https://github.com/a/b"}}}
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
