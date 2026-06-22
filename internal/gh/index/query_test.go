package ghindex

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReposFilterSlashQueryMatchesRepoPathNotGitHubURLPrefix(t *testing.T) {
	repos := Repos{
		{URL: "https://github.com/microsoft/LightGBM", Des: "gradient boosting framework"},
		{URL: "https://github.com/git/git", Des: "Git SCM"},
	}

	got := repos.Filter("/git")
	require.Len(t, got, 1)
	assert.Equal(t, "git/git", got[0].FullName())
}

func TestReposFilterRanksRepoNameMatchesBeforeMetadataMatches(t *testing.T) {
	repos := Repos{
		{URL: "https://github.com/microsoft/LightGBM", Des: "uses git for source control"},
		{URL: "https://github.com/git/git", Des: "Git SCM"},
	}

	got := repos.Filter("git")
	require.Len(t, got, 2)
	assert.Equal(t, "git/git", got[0].FullName())
	assert.Equal(t, "microsoft/LightGBM", got[1].FullName())
}

func TestReposFilterNormalizesGitHubURLQueries(t *testing.T) {
	repos := Repos{
		{URL: "https://github.com/git/git", Des: "Git SCM"},
	}

	got := repos.Filter("https://github.com/git/git.git/tree/master")
	require.Len(t, got, 1)
	assert.Equal(t, "git/git", got[0].FullName())
}
