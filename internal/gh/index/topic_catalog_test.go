package ghindex

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xbpk3t/docs-alfred/internal/gh/content"
)

func TestTopicCatalogIncludesConfigRepoTopics(t *testing.T) {
	repos := ConfigRepos{
		{
			Tag:    "kernel",
			Type:   "tool",
			Topics: content.Topics{{Topic: "Config Topic"}},
			Repos: Repos{
				{
					URL:            "https://github.com/acme/main-repo",
					RelatedRepos: Repos{{URL: "https://github.com/acme/related-repo"}},
				},
			},
		},
	}

	catalog := repos.TopicCatalog()

	assertCatalogHas(t, catalog, "kernel/tool/Config Topic", "gh:config")
}

func assertCatalogHas(t *testing.T, catalog []TopicCandidate, path, source string) {
	t.Helper()
	for _, item := range catalog {
		if item.Path == path && item.Source == source {
			return
		}
	}

	assert.Failf(t, "missing catalog path", "path=%s source=%s catalog=%v", path, source, catalog)
}

func TestIsCatalogPathSafe(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"tag/type/topic", true},
		{"", false},
		{"/absolute", false},
		{"has/./dot", false},
		{"has/../parent", false},
		{"has//empty", false},
		{"has/\x00null", false},
		{"has/\nnewline", false},
		{"simple", true},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.want, isCatalogPathSafe(tt.path))
		})
	}
}

func TestCleanCatalogPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"  tag/type  ", "tag/type"},
		{"tag\\type", "tag/type"},
		{"", ""},
		{"  ", ""},
		{"tag/type/", "tag/type"},
		{"/tag/type", "tag/type"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, cleanCatalogPath(tt.input))
		})
	}
}

func TestJoinPath(t *testing.T) {
	tests := []struct {
		want  string
		parts []string
	}{
		{"a/b/c", []string{"a", "b", "c"}},
		{"a/c", []string{"a", "", "c"}},
		{"", []string{"", "", ""}},
		{"", []string{}},
		{"a", []string{"a"}},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, joinPath(tt.parts...))
		})
	}
}

func TestTopicBase(t *testing.T) {
	assert.Equal(t, "tag/type", topicBase("tag", "type"))
	assert.Empty(t, topicBase("", "type"))
	assert.Empty(t, topicBase("tag", ""))
	assert.Empty(t, topicBase("", ""))
}

func TestTopicDirName(t *testing.T) {
	assert.Equal(t, "topic-name", topicDirName(&content.Topic{Topic: "topic-name"}))
	assert.Empty(t, topicDirName(nil))
}

func TestTopicCatalog_NilConfig(t *testing.T) {
	cr := ConfigRepos{nil}
	catalog := cr.TopicCatalog()
	assert.Empty(t, catalog)
}

func TestTopicCatalog_EmptyRepos(t *testing.T) {
	repos := ConfigRepos{}
	catalog := repos.TopicCatalog()
	assert.Empty(t, catalog)
}

func TestAppendRepoTopicCandidates_NilRepo(t *testing.T) {
	var candidates []TopicCandidate
	seen := make(map[string]bool)
	repos := Repos{nil}
	// Should not panic
	appendRepoTopicCandidates(&candidates, seen, repos, "tag", "type")
	assert.Empty(t, candidates)
}
