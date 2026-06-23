package ghindex

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xbpk3t/docs-alfred/internal/gh/content"
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
			assert.Equal(t, tt.want, r.IsValid())
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
			assert.Equal(t, tt.want, r.FullName())
		})
	}
}

func TestRepository_GetDes(t *testing.T) {
	r := &Repository{Des: "test description"}
	assert.Equal(t, "test description", r.GetDes())
}

func TestRepository_GetURL(t *testing.T) {
	r := &Repository{URL: "https://github.com/a/b"}
	assert.Equal(t, "https://github.com/a/b", r.GetURL())
}

func TestRepository_HasQs(t *testing.T) {
	r1 := &Repository{Topics: []content.Topic{{Topic: "t1"}}}
	assert.True(t, r1.HasQs())

	r2 := &Repository{}
	assert.False(t, r2.HasQs())
}

func TestRepository_HasNix(t *testing.T) {
	r1 := &Repository{NixURL: "github:acme/repo#pkg"}
	assert.True(t, r1.HasNix())

	r2 := &Repository{NixURL: "  "}
	assert.False(t, r2.HasNix())

	r3 := &Repository{}
	assert.False(t, r3.HasNix())
}

func TestRepository_HasSubRepos(t *testing.T) {
	r1 := &Repository{SubRepos: Repos{{URL: "https://github.com/a/b"}}}
	assert.True(t, r1.HasSubRepos())

	r2 := &Repository{ReplacedRepos: Repos{{URL: "https://github.com/a/b"}}}
	assert.True(t, r2.HasSubRepos())

	r3 := &Repository{RelatedRepos: Repos{{URL: "https://github.com/a/b"}}}
	assert.True(t, r3.HasSubRepos())

	r4 := &Repository{}
	assert.False(t, r4.HasSubRepos())
}

func TestRepository_IsSubOrDepOrRelRepo(t *testing.T) {
	r1 := &Repository{IsSubRepo: true}
	assert.True(t, r1.IsSubOrDepOrRelRepo())

	r2 := &Repository{IsReplacedRepo: true}
	assert.True(t, r2.IsSubOrDepOrRelRepo())

	r3 := &Repository{IsRelatedRepo: true}
	assert.True(t, r3.IsSubOrDepOrRelRepo())

	r4 := &Repository{}
	assert.False(t, r4.IsSubOrDepOrRelRepo())
}
