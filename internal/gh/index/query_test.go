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

	got := FilterRepos(repos, "/git")
	require.Len(t, got, 1)
	assert.Equal(t, "git/git", FullName(got[0]))
}

func TestReposFilterRanksRepoNameMatchesBeforeMetadataMatches(t *testing.T) {
	repos := Repos{
		{URL: "https://github.com/microsoft/LightGBM", Des: "uses git for source control"},
		{URL: "https://github.com/git/git", Des: "Git SCM"},
	}

	got := FilterRepos(repos, "git")
	require.Len(t, got, 2)
	assert.Equal(t, "git/git", FullName(got[0]))
	assert.Equal(t, "microsoft/LightGBM", FullName(got[1]))
}

func TestReposFilterNormalizesGitHubURLQueries(t *testing.T) {
	repos := Repos{
		{URL: "https://github.com/git/git", Des: "Git SCM"},
	}

	got := FilterRepos(repos, "https://github.com/git/git.git/tree/master")
	require.Len(t, got, 1)
	assert.Equal(t, "git/git", FullName(got[0]))
}

func TestReposFilter_EmptyRepos(t *testing.T) {
	var repos Repos
	got := FilterRepos(repos, "test")
	assert.Nil(t, got)
}

func TestReposFilter_EmptyQuery(t *testing.T) {
	repos := Repos{
		{URL: "https://github.com/a/b"},
	}
	got := FilterRepos(repos, "")
	assert.Len(t, got, 1)
}

func TestReposFilter_TagMatch(t *testing.T) {
	repos := Repos{
		{URL: "https://github.com/a/b", Tag: "kernel"},
		{URL: "https://github.com/c/d", Tag: "network"},
	}
	got := FilterRepos(repos, "kernel")
	require.Len(t, got, 1)
	assert.Equal(t, "a/b", FullName(got[0]))
}

func TestReposFilter_TypeMatch(t *testing.T) {
	repos := Repos{
		{URL: "https://github.com/a/b", Type: "tool"},
	}
	got := FilterRepos(repos, "tool")
	require.Len(t, got, 1)
}

func TestReposFilter_DesMatch(t *testing.T) {
	repos := Repos{
		{URL: "https://github.com/a/b", Des: "awesome tool"},
	}
	got := FilterRepos(repos, "awesome")
	require.Len(t, got, 1)
}

func TestReposFilter_SuffixMatch(t *testing.T) {
	repos := Repos{
		{URL: "https://github.com/a/b", Des: "test"},
	}
	got := FilterRepos(repos, "a/b")
	require.Len(t, got, 1)
}

func TestReposFilter_SlashQueryNoMetadata(t *testing.T) {
	repos := Repos{
		{URL: "https://github.com/a/b", Des: "git tool"},
	}
	// Slash query should not match metadata
	got := FilterRepos(repos, "/git")
	assert.Empty(t, got)
}

func TestNormalizeSearchQuery(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"  test  ", "test"},
		{"https://github.com/owner/repo.git", "owner/repo"},
		{"https://github.com/owner/repo.git/tree/main", "owner/repo"},
		{"github.com/owner/repo", "owner/repo"},
		{"simple", "simple"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, normalizeSearchQuery(tt.input))
		})
	}
}

func TestRepoNameFromFullName(t *testing.T) {
	assert.Equal(t, "repo", repoNameFromFullName("owner/repo"))
	assert.Equal(t, "full", repoNameFromFullName("full"))
}

func TestExtractTags(t *testing.T) {
	repos := Repos{
		{Tag: "kernel"},
		{Tag: "network"},
		{Tag: "kernel"},
		{Tag: ""},
	}
	tags := ExtractTags(repos)
	assert.Equal(t, []string{"kernel", "network"}, tags)
}

func TestExtractTypesByTag(t *testing.T) {
	repos := Repos{
		{Tag: "kernel", Type: "tool"},
		{Tag: "kernel", Type: "lib"},
		{Tag: "network", Type: "tool"},
	}
	types := ExtractTypesByTag(repos, "kernel")
	assert.Equal(t, []string{"tool", "lib"}, types)
}

func TestQueryReposByTag(t *testing.T) {
	repos := Repos{
		{URL: "https://github.com/a/b", Type: "tool"},
		{URL: "https://github.com/c/d", Type: "lib"},
	}
	filtered := QueryReposByTag(repos, "tool")
	assert.Len(t, filtered, 1)
}

func TestQueryReposByTagAndType(t *testing.T) {
	repos := Repos{
		{URL: "https://github.com/a/b", Tag: "kernel", Type: "tool"},
		{URL: "https://github.com/c/d", Tag: "kernel", Type: "lib"},
		{URL: "https://github.com/e/f", Tag: "network", Type: "tool"},
	}
	filtered := QueryReposByTagAndType(repos, "kernel", "tool")
	assert.Len(t, filtered, 1)
}

func TestMatchRepo_NilRepo(t *testing.T) {
	_, ok := matchRepo(nil, "test")
	assert.False(t, ok)
}

func TestReposFilter_FullNameExactMatch(t *testing.T) {
	repos := Repos{
		{URL: "https://github.com/a/b"},
		{URL: "https://github.com/c/d"},
	}
	got := FilterRepos(repos, "a/b")
	require.Len(t, got, 1)
	assert.Equal(t, "a/b", FullName(got[0]))
}

func TestReposFilter_ContainsMatch(t *testing.T) {
	repos := Repos{
		{URL: "https://github.com/owner/my-repo"},
	}
	got := FilterRepos(repos, "my-re")
	require.Len(t, got, 1)
}

func TestNormalizeSearchQuery_TrimGitSuffix(t *testing.T) {
	assert.Equal(t, "owner/repo", normalizeSearchQuery("owner/repo.git"))
}

func TestExtractTags_EmptyRepos(t *testing.T) {
	var repos Repos
	tags := ExtractTags(repos)
	assert.Empty(t, tags)
}

func TestExtractTypesByTag_EmptyRepos(t *testing.T) {
	var repos Repos
	types := ExtractTypesByTag(repos, "kernel")
	assert.Empty(t, types)
}

func TestQueryReposByTag_EmptyResult(t *testing.T) {
	repos := Repos{
		{URL: "https://github.com/a/b", Type: "tool"},
	}
	filtered := QueryReposByTag(repos, "nonexistent")
	assert.Empty(t, filtered)
}

func TestQueryReposByTagAndType_EmptyResult(t *testing.T) {
	repos := Repos{
		{URL: "https://github.com/a/b", Tag: "kernel", Type: "tool"},
	}
	filtered := QueryReposByTagAndType(repos, "kernel", "nonexistent")
	assert.Empty(t, filtered)
}

func TestReposFilter_SortByScoreAndIndex(t *testing.T) {
	// Multiple repos matching with different scores to exercise all sort branches
	// "awesome" matches: a/z-repo (des=score 60), b/a-repo (des=score 60), c/m-repo (tag=score 4)
	// After sort: tag matches (score 4) come first (ascending), then des matches (score 60)
	repos := Repos{
		{URL: "https://github.com/a/z-repo", Des: "awesome"},
		{URL: "https://github.com/b/a-repo", Des: "awesome"},
		{URL: "https://github.com/c/m-repo", Tag: "awesome"},
	}
	got := FilterRepos(repos, "awesome")
	require.Len(t, got, 3)
	// Tag match (score 4) should come first since sort is ascending by score
	assert.Equal(t, "c/m-repo", FullName(got[0]))
}

func TestReposFilter_RepoNameExactMatch(t *testing.T) {
	// Test repo name exact match (score 1 vs des match score 5)
	repos := Repos{
		{URL: "https://github.com/a/xyz", Des: "has tool in it"},
		{URL: "https://github.com/b/tool"},
	}
	got := FilterRepos(repos, "tool")
	require.Len(t, got, 2)
	// name exact match (score 1) should come before des contains (score 5)
	assert.Equal(t, "b/tool", FullName(got[0]))
}

func TestReposFilter_EqualScoresDifferentIndices(t *testing.T) {
	// Two repos with same score should maintain original order (stable sort)
	repos := Repos{
		{URL: "https://github.com/a/first", Des: "test"},
		{URL: "https://github.com/b/second", Des: "test"},
	}
	got := FilterRepos(repos, "test")
	require.Len(t, got, 2)
	assert.Equal(t, "a/first", FullName(got[0]))
	assert.Equal(t, "b/second", FullName(got[1]))
}
