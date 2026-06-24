package ghindex

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xbpk3t/docs-alfred/internal/gh/content"
)

func TestTopicCatalogIncludesConfigUsingRepoAndNestedTopics(t *testing.T) {
	repos := ConfigRepos{
		{
			Tag:    "kernel",
			Type:   "tool",
			Topics: content.Topics{{Topic: "Config Topic", Meta: &content.TopicMeta{Slug: "config-topic"}}},
			Using:  Repository{Topics: content.Topics{{Topic: "Using Topic"}}},
			Repos: Repos{
				{
					URL: "https://github.com/acme/main-repo",
					Topics: content.Topics{{
						Topic: "Parent Topic",
						Meta:  &content.TopicMeta{Slug: "parent-topic"},
						Sub:   content.Topics{{Topic: "Child Topic", Meta: &content.TopicMeta{Slug: "child-topic"}}},
					}},
					SubRepos: Repos{{
						URL:    "https://github.com/acme/sub-repo",
						Topics: content.Topics{{Topic: "Sub Topic"}},
					}},
				},
			},
		},
	}

	catalog := repos.TopicCatalog()

	assertCatalogHas(t, catalog, "kernel/tool/config-topic", "gh:config")
	assertCatalogHas(t, catalog, "kernel/tool/Using Topic", "gh:using")
	assertCatalogHas(t, catalog, "kernel/tool/main-repo/parent-topic", "gh:repo")
	assertCatalogHas(t, catalog, "kernel/tool/main-repo/parent-topic/child-topic", "gh:repo")
	assertCatalogHas(t, catalog, "kernel/tool/sub-repo/Sub Topic", "gh:repo")
}

func TestTopicCatalogUsesPicDirAsCanonicalPath(t *testing.T) {
	repos := ConfigRepos{{
		Tag:    "kernel",
		Type:   "tool",
		Topics: content.Topics{{Topic: "Display", PicDir: "custom/topic/path"}},
	}}

	catalog := repos.TopicCatalog()

	assertCatalogHas(t, catalog, "custom/topic/path", "gh:config")
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
	assert.Equal(t, "slug", topicDirName(&content.Topic{Topic: "t", Meta: &content.TopicMeta{Slug: "slug"}}))
	assert.Empty(t, topicDirName(nil))
	assert.Equal(t, "topic", topicDirName(&content.Topic{Topic: "topic", Meta: &content.TopicMeta{}}))
}

func TestTopicCatalog_NilConfig(t *testing.T) {
	cr := ConfigRepos{nil}
	catalog := cr.TopicCatalog()
	assert.Empty(t, catalog)
}

func TestCanonicalTopicPath_NilTopic(t *testing.T) {
	path := canonicalTopicPath(nil, "base/path")
	assert.Equal(t, "base/path", path)
}

func TestCanonicalTopicPath_WithPicDir(t *testing.T) {
	topic := &content.Topic{Topic: "t", PicDir: "custom/path"}
	path := canonicalTopicPath(topic, "base")
	assert.Equal(t, "custom/path", path)
}

func TestTopicCatalog_WithReplacedAndRelatedRepos(t *testing.T) {
	repos := ConfigRepos{{
		Tag:  "kernel",
		Type: "tool",
		Repos: Repos{{
			URL:    "https://github.com/acme/main",
			Topics: content.Topics{{Topic: "Main Topic", Meta: &content.TopicMeta{Slug: "main-topic"}}},
			ReplacedRepos: Repos{{
				URL:    "https://github.com/acme/old",
				Topics: content.Topics{{Topic: "Old Topic"}},
			}},
			RelatedRepos: Repos{{
				URL:    "https://github.com/acme/related",
				Topics: content.Topics{{Topic: "Related Topic"}},
			}},
		}},
	}}

	catalog := repos.TopicCatalog()
	assertCatalogHas(t, catalog, "kernel/tool/main/main-topic", "gh:repo")
	assertCatalogHas(t, catalog, "kernel/tool/old/Old Topic", "gh:repo")
	assertCatalogHas(t, catalog, "kernel/tool/related/Related Topic", "gh:repo")
}

func TestTopicCatalog_DuplicatePaths(t *testing.T) {
	// Same topic path should only appear once
	repos := ConfigRepos{{
		Tag:    "kernel",
		Type:   "tool",
		Topics: content.Topics{{Topic: "same", Meta: &content.TopicMeta{Slug: "same"}}},
	}}

	catalog := repos.TopicCatalog()
	count := 0
	for _, c := range catalog {
		if c.Path == "kernel/tool/same" {
			count++
		}
	}
	assert.Equal(t, 1, count)
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
