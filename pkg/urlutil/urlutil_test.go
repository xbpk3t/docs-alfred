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

func TestDomain(t *testing.T) {
	require.Equal(t, "example.com", Domain("https://example.com/path"))
	require.Equal(t, "example.com", Domain("https://Example.COM:8080/path"))
	require.Empty(t, Domain("%zz"))
}

func TestNormalize_TrailingSlash(t *testing.T) {
	result := Normalize("https://example.com/path/")
	require.Equal(t, "https://example.com/path", result)
}

func TestNormalize_BasicURL(t *testing.T) {
	result := Normalize("HTTPS://Example.COM:443/path#frag")
	require.NotEmpty(t, result)
	require.Contains(t, result, "example.com")
}

func TestRegistrableDomain(t *testing.T) {
	require.Equal(t, "example.com", RegistrableDomain("sub.example.com"))
	require.Equal(t, "example.co.uk", RegistrableDomain("sub.example.co.uk"))
	require.Empty(t, RegistrableDomain("localhost"))
	require.Empty(t, RegistrableDomain(""))
}

func TestDomainBlocked_EmptyDomain(t *testing.T) {
	require.False(t, DomainBlocked("", map[string]bool{"example.com": true}))
	require.False(t, DomainBlocked("  ", map[string]bool{"example.com": true}))
}

func TestDomainBlocked_ExactMatch(t *testing.T) {
	require.True(t, DomainBlocked("example.com", map[string]bool{"example.com": true}))
}

func TestDomainBlocked_SuffixMatch(t *testing.T) {
	// For an IP address, RegistrableDomain returns "", so the suffix loop runs.
	// The suffix loop checks full suffixes starting from the last part.
	blocked := map[string]bool{"1": true}
	require.True(t, DomainBlocked("127.0.0.1", blocked), "should match suffix '1'")
	// "10.0.0.0" has no matching suffix in the blocked set
	require.False(t, DomainBlocked("10.0.0.0", blocked))
}

func TestDomainBlocked_ReturnsFalse(t *testing.T) {
	// IP address not matching any blocked suffix
	require.False(t, DomainBlocked("192.168.1.1", map[string]bool{"10.0.0.0": true}))
}

func TestDomainBlocked_NotBlocked(t *testing.T) {
	require.False(t, DomainBlocked("safe.org", map[string]bool{"example.com": true}))
}

func TestRepoName_ParsePath(t *testing.T) {
	// When url.Parse succeeds and path is non-empty, use repoNameFromPath
	require.Equal(t, "repo", RepoName("https://github.com/owner/repo"))
}

func TestRepoName_Fallback(t *testing.T) {
	// When url.Parse succeeds but path is empty, fall back to stripping scheme prefix
	require.Equal(t, "example.com", RepoName("https://example.com"))
}

func TestRepoNameFromPath_Empty(t *testing.T) {
	require.Empty(t, repoNameFromPath(""))
	require.Equal(t, "repo", repoNameFromPath("/owner/repo"))
	require.Equal(t, "repo", repoNameFromPath("/owner/repo.git"))
}

func TestSourceRepo_ParseError(t *testing.T) {
	_, ok := SourceRepo("%zz")
	require.False(t, ok)
}

func TestSourceRepo_InsufficientParts(t *testing.T) {
	_, ok := SourceRepo("https://github.com/only-owner")
	require.False(t, ok)
}

func TestIsSourceRepo_False(t *testing.T) {
	require.False(t, IsSourceRepo("https://example.com/owner/repo"))
}

func TestCountURLs(t *testing.T) {
	count := CountURLs("Visit https://a.com and https://b.com")
	require.Equal(t, 2, count)
}

func TestCountURLs_Empty(t *testing.T) {
	require.Equal(t, 0, CountURLs(""))
	require.Equal(t, 0, CountURLs("no urls here"))
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		url     string
		wantErr bool
	}{
		{"https://example.com/path", false},
		{"http://example.com", false},
		{"file:///etc/passwd", true},
		{"ftp://example.com/file", true},
		{"https://evil.com@legit.com", true},
		{"https://user:pass@example.com", true},
		{"https://example.com/path\x00null", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			err := ValidateURL(tt.url)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
