package urlutil

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEqualNormalizesURL(t *testing.T) {
	require.True(t, Equal(
		"HTTPS://github.com:443/Owner/repo/",
		"https://github.com/Owner/repo#readme",
	), "expected normalized URLs to match")
}

func TestRepoName(t *testing.T) {
	tests := map[string]string{
		"https://github.com/owner/repo":      "repo",
		"https://github.com/owner/repo.git/": "repo",
		"https://github.com/a/b/c/d":         "d",
		"owner/repo.git":                     "repo",
		"":                                   "",
	}

	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			require.Equal(t, want, RepoName(input))
		})
	}
}

func TestGitHubOwnerRepo(t *testing.T) {
	repo, ok := GitHubOwnerRepo("https://github.com/owner/repo.git/tree/main")
	require.True(t, ok, "expected GitHub repo URL to parse")
	require.Equal(t, "owner", repo.Owner)
	require.Equal(t, "repo", repo.Name)

	_, ok = GitHubOwnerRepo("https://gitlab.com/owner/repo")
	require.False(t, ok, "expected non-GitHub URL to be rejected")
}

func TestSourceRepo(t *testing.T) {
	tests := []struct {
		input string
		host  string
		owner string
		name  string
		ok    bool
	}{
		{input: "https://github.com/owner/repo.git/tree/main", host: "github.com", owner: "owner", name: "repo", ok: true},
		{input: "https://gitlab.com/group/project/-/issues", host: "gitlab.com", owner: "group", name: "project", ok: true},
		{input: "https://example.com/owner/repo", ok: false},
		{input: "https://github.com/search", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			repo, ok := SourceRepo(tt.input)
			require.Equal(t, tt.ok, ok)
			if !ok {
				return
			}
			require.Equal(t, tt.host, repo.Host)
			require.Equal(t, tt.owner, repo.Owner)
			require.Equal(t, tt.name, repo.Name)
			require.True(t, IsSourceRepo(tt.input))
		})
	}
}

func TestDomainBlocked(t *testing.T) {
	blocked := map[string]bool{"example.com": true}
	require.True(t, DomainBlocked("blog.example.com", blocked), "expected registrable domain match")
	require.False(t, DomainBlocked("example.org", blocked), "unexpected blocked domain")
}
