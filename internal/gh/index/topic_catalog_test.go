package ghindex

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xbpk3t/docs-alfred/internal/gh/model/gh"
)

func TestTopicCatalogIncludesConfigTopicTopics(t *testing.T) {
	repos := ConfigRepos{
		{
			Tag:    "kernel",
			Type:   "tool",
			Topics: Topics{{Topic: "Config Topic", Kind: "type"}},
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

func TestTopicDirName(t *testing.T) {
	assert.Equal(t, "topic-name", (&gh.Topic{Topic: "topic-name"}).DirName())
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

func TestTopicCatalogExcludesTemp(t *testing.T) {
	repos := ConfigRepos{
		{
			Tag:  "kernel",
			Type: "mem",
			Topics: Topics{
				{Topic: "futex", Kind: "type"},
				{Topic: "draft", Kind: "temp"},
				{Topic: "bpf", Kind: "tools"},
			},
		},
	}

	catalog := repos.TopicCatalog()
	assert.Len(t, catalog, 2)
	assertCatalogHas(t, catalog, "kernel/mem/futex", "gh:config")
	assertCatalogHas(t, catalog, "kernel/mem/bpf", "gh:config")

	all := repos.TopicCatalogWithKinds(append(append([]string{}, DefaultTopicKinds...), "temp"))
	assert.Len(t, all, 3)
	assertCatalogHas(t, all, "kernel/mem/draft", "gh:config")
}
