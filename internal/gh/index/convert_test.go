package ghindex

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToRepos_BasicConversion(t *testing.T) {
	cr := ConfigRepos{
		{
			Type: "tool",
			Tag:  "kernel",
			Repos: Repos{
				{URL: "https://github.com/acme/main-repo"},
			},
		},
	}

	repos := cr.ToRepos()
	require.NotEmpty(t, repos)
	assert.GreaterOrEqual(t, len(repos), 1)
}

func TestToRepos_SubRepos(t *testing.T) {
	cr := ConfigRepos{
		{
			Type: "tool",
			Tag:  "kernel",
			Repos: Repos{
				{
					URL: "https://github.com/acme/main",
					RelatedRepos: Repos{
						{URL: "https://github.com/acme/related"},
					},
				},
			},
		},
	}

	repos := cr.ToRepos()
	require.NotEmpty(t, repos)

	// Check sub-repo flags
	var relatedRepo bool
	for _, r := range repos {
		if r.IsRelatedRepo {
			relatedRepo = true
		}
	}
	assert.True(t, relatedRepo)
}

func TestToRepos_InvalidURL(t *testing.T) {
	cr := ConfigRepos{
		{
			Type: "tool",
			Tag:  "kernel",
			Repos: Repos{
				{URL: "not-a-github-url"},
			},
		},
	}

	repos := cr.ToRepos()
	// Invalid GitHub URLs should be filtered out
	assert.Empty(t, repos)
}

func TestProcessRepo_NilSubRepos(t *testing.T) {
	repo := &Repository{
		URL: "https://github.com/acme/main",
	}
	repos := processRepo(repo, "tool")
	require.Len(t, repos, 1)
	assert.Equal(t, "tool", repos[0].Type)
}

func TestToRepos_GitLabWithNix(t *testing.T) {
	cr := ConfigRepos{
		{
			Type: "devops",
			Tag:  "devops",
			Repos: Repos{
				{
					URL:    "https://gitlab.com/gitlab-org/cli",
					NixURL: "https://mynixos.com/nixpkgs/package/glab",
				},
			},
		},
	}

	repos := cr.ToRepos()
	require.NotEmpty(t, repos)
	assert.Equal(t, "https://gitlab.com/gitlab-org/cli", repos[0].URL)
	assert.Equal(t, "https://mynixos.com/nixpkgs/package/glab", repos[0].NixURL)
}

func TestIsValidSourceRepoURL(t *testing.T) {
	assert.True(t, isValidSourceRepoURL("https://github.com/owner/repo"))
	assert.True(t, isValidSourceRepoURL("https://gitlab.com/owner/repo"))
	assert.False(t, isValidSourceRepoURL("https://example.com/owner/repo"))
	assert.False(t, isValidSourceRepoURL("not-a-url"))
}
