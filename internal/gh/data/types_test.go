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
	assert.Len(t, section.Repo, 1)
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
	assert.Equal(t, "tool", section.Type)
	assert.Len(t, section.Topics, 1)
	assert.Equal(t, "overview", section.Topics[0].Topic)
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
	require.NotNil(t, repo.Des)
	assert.Equal(t, "test", *repo.Des)
	require.NotNil(t, repo.Nix)
	assert.Equal(t, "nix-value", *repo.Nix)
	require.NotNil(t, repo.Doc)
	assert.Equal(t, "doc-url", *repo.Doc)
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
	require.Len(t, section.Repo, 2)
	assert.Equal(t, "https://github.com/owner/repo1", section.Repo[0].URL)
	require.NotNil(t, section.Repo[0].Des)
	assert.Equal(t, "first repo", *section.Repo[0].Des)
	assert.Equal(t, "https://github.com/owner/repo2", section.Repo[1].URL)
	require.NotNil(t, section.Repo[1].Des)
	assert.Equal(t, "second repo", *section.Repo[1].Des)
}

func TestSectionFromMap_EmptyMap(t *testing.T) {
	section := sectionFromMap(map[string]any{})
	assert.Empty(t, section.Type)
	assert.Empty(t, section.Repo)
}

func TestSectionFromMap_RepoNonMappingItem(t *testing.T) {
	// mapstructure decodes non-mapping repo items as empty Repo structs; the
	// walker's emitRepoEvents filters them out before yielding events, so real
	// repos are unaffected.
	m := map[string]any{
		"type": "tool",
		"repo": []any{"just a string", 42},
	}
	section := sectionFromMap(m)
	assert.Len(t, section.Repo, 2)
	assert.Empty(t, section.Repo[0].URL)
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
	assert.Empty(t, section.Repo)
}
