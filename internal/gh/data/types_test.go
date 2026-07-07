package ghdata

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSectionFromMap(t *testing.T) {
	m := map[string]any{
		"type": "language",
		"repo": []any{
			map[string]any{
				"url": "https://github.com/owner/repo",
				"des": "test repo",
			},
		},
	}
	section := sectionFromMap(m)
	assert.Equal(t, "language", section.Type)
	assert.Len(t, section.Repos, 1)
}

func TestSectionFromMap_Topics(t *testing.T) {
	m := map[string]any{
		"type": "tool",
		"topics": []any{
			map[string]any{
				"topic": "overview",
			},
		},
	}
	section := sectionFromMap(m)
	// Section doesn't parse topics directly (they're handled by ConfigRepo)
	assert.Equal(t, "tool", section.Type)
}

func TestSectionFromMap_NilRecord(t *testing.T) {
	m := map[string]any{
		"type":   "tool",
		"record": nil,
	}
	section := sectionFromMap(m)
	assert.Equal(t, "tool", section.Type)
}

func TestRepoFromMap(t *testing.T) {
	m := map[string]any{
		"url": "https://github.com/owner/repo",
		"des": "test",
		"nix": "nix-value",
		"doc": "doc-url",
	}
	repo := repoFromMap(m)
	assert.Equal(t, "https://github.com/owner/repo", repo.URL)
	assert.Equal(t, "test", repo.Des)
	assert.Equal(t, "nix-value", repo.NixURL)
	assert.Equal(t, "doc-url", repo.Doc)
}

func TestTopicFromMap(t *testing.T) {
	m := map[string]any{
		"topic": "main",
	}
	topic := topicFromMap(m)
	assert.Equal(t, "main", topic.Topic)
}

func TestTopic_DirName(t *testing.T) {
	tests := []struct {
		name  string
		want  string
		topic Topic
	}{
		{"simple", "topic-name", Topic{Topic: "topic-name"}},
		{"empty", "", Topic{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.topic.DirName())
		})
	}
}

func TestSectionFromMap_WithRepos(t *testing.T) {
	m := map[string]any{
		"type": "language",
		"repo": []any{
			map[string]any{
				"url": "https://github.com/owner/repo1",
				"des": "first repo",
			},
			map[string]any{
				"url": "https://github.com/owner/repo2",
				"des": "second repo",
			},
		},
	}
	section := sectionFromMap(m)
	require.Len(t, section.Repos, 2)
	assert.Equal(t, "https://github.com/owner/repo1", section.Repos[0].URL)
	assert.Equal(t, "first repo", section.Repos[0].Des)
	assert.Equal(t, "https://github.com/owner/repo2", section.Repos[1].URL)
	assert.Equal(t, "second repo", section.Repos[1].Des)
}

func TestSectionFromMap_EmptyMap(t *testing.T) {
	section := sectionFromMap(map[string]any{})
	assert.Empty(t, section.Type)
	assert.Empty(t, section.Repos)
}

func TestTopicFromMap_WithRepos(t *testing.T) {
	m := map[string]any{
		"topic": "parent",
		"repo": []any{
			map[string]any{"url": "https://github.com/acme/sub"},
		},
	}
	topic := topicFromMap(m)
	assert.Equal(t, "parent", topic.Topic)
	require.Len(t, topic.Repos, 1)
	assert.Equal(t, "https://github.com/acme/sub", topic.Repos[0].URL)
}

func TestDecodeYAMLMap_Basic(t *testing.T) {
	input := map[string]any{
		"url": "https://github.com/test/repo",
		"des": "a description",
	}
	var out repoFields
	decodeYAMLMap(input, &out)
	assert.Equal(t, "https://github.com/test/repo", out.URL)
	assert.Equal(t, "a description", out.Des)
}

func TestSectionFromMap_RepoNonMappingItem(t *testing.T) {
	m := map[string]any{
		"type": "tool",
		"repo": []any{"just a string", 42},
	}
	section := sectionFromMap(m)
	assert.Empty(t, section.Repos)
}

func TestSectionFromMap_TopicsNonSlice(t *testing.T) {
	m := map[string]any{
		"type":   "tool",
		"topics": "not a slice",
	}
	section := sectionFromMap(m)
	assert.Equal(t, "tool", section.Type)
}

func TestSectionFromMap_RepoEmptySlice(t *testing.T) {
	m := map[string]any{
		"type": "tool",
		"repo": []any{},
	}
	section := sectionFromMap(m)
	assert.Empty(t, section.Repos)
}
