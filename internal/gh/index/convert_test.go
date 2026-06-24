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
			Using: Repository{
				URL: "https://github.com/acme/using-repo",
			},
			Repos: Repos{
				{URL: "https://github.com/acme/main-repo"},
			},
		},
	}

	repos := cr.ToRepos()
	require.NotEmpty(t, repos)
	// Both using and main repo should be present
	assert.GreaterOrEqual(t, len(repos), 2)
}

func TestToRepos_SubRepos(t *testing.T) {
	cr := ConfigRepos{
		{
			Type: "tool",
			Tag:  "kernel",
			Repos: Repos{
				{
					URL: "https://github.com/acme/main",
					SubRepos: Repos{
						{URL: "https://github.com/acme/sub"},
					},
					ReplacedRepos: Repos{
						{URL: "https://github.com/acme/replaced"},
					},
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
	var subRepo, replacedRepo, relatedRepo bool
	for _, r := range repos {
		if r.IsSubRepo {
			subRepo = true
		}
		if r.IsReplacedRepo {
			replacedRepo = true
		}
		if r.IsRelatedRepo {
			relatedRepo = true
		}
	}
	assert.True(t, subRepo)
	assert.True(t, replacedRepo)
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

func TestIsValidGithubURL(t *testing.T) {
	assert.True(t, isValidGithubURL("https://github.com/owner/repo"))
	assert.False(t, isValidGithubURL("https://example.com/owner/repo"))
	assert.False(t, isValidGithubURL("not-a-url"))
}
