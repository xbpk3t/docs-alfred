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
