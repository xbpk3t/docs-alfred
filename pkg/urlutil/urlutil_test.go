package urlutil

import "testing"

func TestEqualNormalizesURL(t *testing.T) {
	if !Equal("HTTPS://github.com:443/Owner/repo/", "https://github.com/Owner/repo#readme") {
		t.Fatal("expected normalized URLs to match")
	}
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
		if got := RepoName(input); got != want {
			t.Fatalf("RepoName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestGitHubOwnerRepo(t *testing.T) {
	repo, ok := GitHubOwnerRepo("https://github.com/owner/repo.git/tree/main")
	if !ok {
		t.Fatal("expected GitHub repo URL to parse")
	}
	if repo.Owner != "owner" || repo.Name != "repo" {
		t.Fatalf("GitHubOwnerRepo returned %q/%q", repo.Owner, repo.Name)
	}

	if _, ok := GitHubOwnerRepo("https://gitlab.com/owner/repo"); ok {
		t.Fatal("expected non-GitHub URL to be rejected")
	}
}

func TestDomainBlocked(t *testing.T) {
	blocked := map[string]bool{"example.com": true}
	if !DomainBlocked("blog.example.com", blocked) {
		t.Fatal("expected registrable domain match")
	}
	if DomainBlocked("example.org", blocked) {
		t.Fatal("unexpected blocked domain")
	}
}
