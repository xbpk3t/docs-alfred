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
		"record": []any{
			map[string]any{"date": "2024-01-01", "des": "record1"},
		},
	}
	section := sectionFromMap(m)
	assert.Equal(t, "language", section.Type)
	assert.Len(t, section.Repos, 1)
	assert.True(t, section.HasRecord)
	assert.True(t, section.RecordValid)
	assert.Len(t, section.Record, 1)
}

func TestSectionFromMap_Using(t *testing.T) {
	m := map[string]any{
		"type": "tool",
		"using": map[string]any{
			"url": "https://github.com/owner/using-repo",
			"des": "using desc",
		},
	}
	section := sectionFromMap(m)
	require.NotNil(t, section.Using)
	assert.Equal(t, "https://github.com/owner/using-repo", section.Using.URL)
}

func TestSectionFromMap_Topics(t *testing.T) {
	m := map[string]any{
		"type": "tool",
		"topics": []any{
			map[string]any{
				"topic": "overview",
				"meta": map[string]any{
					"slug":   "intro",
					"hasPic": true,
				},
			},
		},
	}
	section := sectionFromMap(m)
	require.Len(t, section.Topics, 1)
	assert.Equal(t, "overview", section.Topics[0].Topic)
	assert.Equal(t, "intro", section.Topics[0].Meta.Slug)
	assert.True(t, section.Topics[0].Meta.HasPic)
}

func TestSectionFromMap_InvalidRecord(t *testing.T) {
	m := map[string]any{
		"type":   "tool",
		"record": "not-an-array",
	}
	section := sectionFromMap(m)
	assert.True(t, section.HasRecord)
	assert.False(t, section.RecordValid)
}

func TestSectionFromMap_NilRecord(t *testing.T) {
	m := map[string]any{
		"type":   "tool",
		"record": nil,
	}
	section := sectionFromMap(m)
	assert.True(t, section.HasRecord)
	assert.False(t, section.RecordValid)
}

func TestRepoFromMap(t *testing.T) {
	m := map[string]any{
		"url": "https://github.com/owner/repo",
		"des": "test",
		"zk":  "zk link",
		"topics": []any{
			map[string]any{"topic": "t1"},
		},
		"record": []any{
			map[string]any{"date": "2024-01-01", "des": "r1"},
		},
	}
	repo := repoFromMap(m)
	assert.Equal(t, "https://github.com/owner/repo", repo.URL)
	assert.Equal(t, "test", repo.Des)
	assert.Equal(t, "zk link", repo.Zk)
	assert.Len(t, repo.Topics, 1)
	assert.True(t, repo.HasRecord)
	assert.True(t, repo.RecordValid)
}

func TestRepoFromMap_InvalidRecord(t *testing.T) {
	m := map[string]any{
		"url":    "https://github.com/owner/repo",
		"record": 123,
	}
	repo := repoFromMap(m)
	assert.True(t, repo.HasRecord)
	assert.False(t, repo.RecordValid)
}

func TestTopicsFromAny(t *testing.T) {
	tests := []struct {
		input any
		name  string
		want  int
	}{
		{nil, "nil", 0},
		{"string", "not slice", 0},
		{[]any{}, "empty slice", 0},
		{[]any{map[string]any{"topic": "t1"}}, "valid", 1},
		{[]any{map[string]any{"topic": "t1"}, "invalid"}, "mixed", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			topics := topicsFromAny(tt.input)
			assert.Len(t, topics, tt.want)
		})
	}
}

func TestTopicFromMap(t *testing.T) {
	m := map[string]any{
		"topic":  "main",
		"hasPic": true,
		"meta": map[string]any{
			"slug":   "main-slug",
			"hasPic": true,
		},
		"sub": []any{
			map[string]any{"topic": "sub1"},
		},
		"record": []any{
			map[string]any{"date": "2024-01-01", "des": "r1"},
		},
	}
	topic := topicFromMap(m)
	assert.Equal(t, "main", topic.Topic)
	assert.True(t, topic.HasPic)
	assert.Equal(t, "main-slug", topic.Meta.Slug)
	assert.Len(t, topic.Sub, 1)
	assert.True(t, topic.HasRecord)
}

func TestTopic_DirName(t *testing.T) {
	tests := []struct {
		name  string
		want  string
		topic Topic
	}{
		{"with slug", "my-slug", Topic{Topic: "t", Meta: &TopicMeta{Slug: "my-slug"}}},
		{"without slug", "topic-name", Topic{Topic: "topic-name"}},
		{"empty", "", Topic{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.topic.DirName())
		})
	}
}

func TestTopic_HasPicture(t *testing.T) {
	tests := []struct {
		name  string
		topic Topic
		want  bool
	}{
		{"meta hasPic", Topic{Meta: &TopicMeta{HasPic: true}, HasPic: true}, true},
		{"topic hasPic", Topic{HasPic: true}, true},
		{"neither", Topic{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.topic.HasPicture())
		})
	}
}

func TestRecordsFromAny(t *testing.T) {
	tests := []struct {
		input     any
		name      string
		wantLen   int
		wantValid bool
	}{
		{nil, "nil", 0, false},
		{"string", "not slice", 0, false},
		{[]any{}, "empty", 0, true},
		{[]any{map[string]any{"date": "2024-01-01", "des": "test"}}, "valid", 1, true},
		{[]any{map[string]any{"date": "2024-01-01", "des": "test"}, "bad"}, "mixed", 1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			records, valid := recordsFromAny(tt.input)
			assert.Len(t, records, tt.wantLen)
			assert.Equal(t, tt.wantValid, valid)
		})
	}
}

// --- Additional focused tests ---

func TestSectionFromMap_WithUsingEntry(t *testing.T) {
	m := map[string]any{
		"type": "tool",
		"using": map[string]any{
			"url": "https://github.com/acme/framework",
			"des": "a framework",
			"zk":  "zk-ref",
		},
	}
	section := sectionFromMap(m)
	require.NotNil(t, section.Using)
	assert.Equal(t, "https://github.com/acme/framework", section.Using.URL)
	assert.Equal(t, "a framework", section.Using.Des)
	assert.Equal(t, "zk-ref", section.Using.Zk)
	assert.True(t, section.Using.RecordValid)
	assert.Empty(t, section.Repos)
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
	assert.False(t, section.HasRecord)
}

func TestSectionFromMap_WithRecord(t *testing.T) {
	m := map[string]any{
		"type": "language",
		"record": []any{
			map[string]any{"date": "2024-03-15", "des": "initial setup"},
			map[string]any{"date": "2024-06-01", "des": "update"},
		},
	}
	section := sectionFromMap(m)
	assert.True(t, section.HasRecord)
	assert.True(t, section.RecordValid)
	require.Len(t, section.Record, 2)
	assert.Equal(t, "2024-03-15", section.Record[0].Date)
	assert.Equal(t, "initial setup", section.Record[0].Des)
	assert.Equal(t, "2024-06-01", section.Record[1].Date)
	assert.Equal(t, "update", section.Record[1].Des)
}

func TestSectionFromMap_EmptyMap(t *testing.T) {
	section := sectionFromMap(map[string]any{})
	assert.Empty(t, section.Type)
	assert.Nil(t, section.Using)
	assert.Empty(t, section.Repos)
	assert.Empty(t, section.Topics)
	assert.False(t, section.HasRecord)
	assert.True(t, section.RecordValid) // default is true per constructor
}

func TestRepoFromMap_WithTopicsAndRecord(t *testing.T) {
	m := map[string]any{
		"url": "https://github.com/owner/repo",
		"des": "test repo",
		"zk":  "zk link",
		"topics": []any{
			map[string]any{"topic": "installation"},
			map[string]any{"topic": "usage"},
		},
		"record": []any{
			map[string]any{"date": "2024-05-10", "des": "first release"},
		},
	}
	repo := repoFromMap(m)
	assert.Equal(t, "https://github.com/owner/repo", repo.URL)
	assert.Equal(t, "test repo", repo.Des)
	assert.Equal(t, "zk link", repo.Zk)
	require.Len(t, repo.Topics, 2)
	assert.Equal(t, "installation", repo.Topics[0].Topic)
	assert.Equal(t, "usage", repo.Topics[1].Topic)
	assert.True(t, repo.HasRecord)
	assert.True(t, repo.RecordValid)
	require.Len(t, repo.Record, 1)
	assert.Equal(t, "2024-05-10", repo.Record[0].Date)
	assert.Equal(t, "first release", repo.Record[0].Des)
}

func TestTopicsFromAny_NonSliceInput(t *testing.T) {
	assert.Nil(t, topicsFromAny("a string"))
	assert.Nil(t, topicsFromAny(42))
	assert.Nil(t, topicsFromAny(map[string]any{"key": "value"}))
	assert.Nil(t, topicsFromAny(true))
}

func TestTopicFromMap_WithSubTopics(t *testing.T) {
	m := map[string]any{
		"topic": "parent",
		"sub": []any{
			map[string]any{"topic": "child1"},
			map[string]any{
				"topic": "child2",
				"sub": []any{
					map[string]any{"topic": "grandchild"},
				},
			},
		},
	}
	topic := topicFromMap(m)
	assert.Equal(t, "parent", topic.Topic)
	require.Len(t, topic.Sub, 2)
	assert.Equal(t, "child1", topic.Sub[0].Topic)
	assert.Empty(t, topic.Sub[0].Sub)
	assert.Equal(t, "child2", topic.Sub[1].Topic)
	require.Len(t, topic.Sub[1].Sub, 1)
	assert.Equal(t, "grandchild", topic.Sub[1].Sub[0].Topic)
	assert.False(t, topic.HasRecord)
}

func TestRecordsFromAny_NilInput(t *testing.T) {
	records, valid := recordsFromAny(nil)
	assert.Nil(t, records)
	assert.False(t, valid)
}

func TestRecordsFromAny_NonSliceInput(t *testing.T) {
	records, valid := recordsFromAny("not a slice")
	assert.Nil(t, records)
	assert.False(t, valid)

	records, valid = recordsFromAny(123)
	assert.Nil(t, records)
	assert.False(t, valid)

	records, valid = recordsFromAny(map[string]any{"key": "value"})
	assert.Nil(t, records)
	assert.False(t, valid)
}

func TestDecodeYAMLMap_Basic(t *testing.T) {
	input := map[string]any{
		"url": "https://github.com/test/repo",
		"des": "a description",
		"zk":  "zk value",
	}
	var out repoFields
	decodeYAMLMap(input, &out)
	assert.Equal(t, "https://github.com/test/repo", out.URL)
	assert.Equal(t, "a description", out.Des)
	assert.Equal(t, "zk value", out.Zk)
}

func TestTopic_DirName_WithMetaSlug(t *testing.T) {
	topic := Topic{
		Topic: "my-topic",
		Meta:  &TopicMeta{Slug: "custom-slug"},
	}
	assert.Equal(t, "custom-slug", topic.DirName())
}

func TestTopic_DirName_WithoutMetaSlug(t *testing.T) {
	topic := Topic{
		Topic: "my-topic",
		Meta:  &TopicMeta{Slug: ""},
	}
	assert.Equal(t, "my-topic", topic.DirName())
}

func TestTopic_HasPicture_WithMetaHasPic(t *testing.T) {
	topic := Topic{
		Meta:   &TopicMeta{HasPic: true},
		HasPic: false,
	}
	assert.True(t, topic.HasPicture())
}

func TestTopic_HasPicture_WithTopicHasPic(t *testing.T) {
	topic := Topic{
		Meta:   &TopicMeta{HasPic: false},
		HasPic: true,
	}
	assert.True(t, topic.HasPicture())
}

func TestSectionFromMap_RepoNonMappingItem(t *testing.T) {
	m := map[string]any{
		"type": "tool",
		"repo": []any{"just a string", 42},
	}
	section := sectionFromMap(m)
	assert.Empty(t, section.Repos)
}

func TestTopicFromMap_EmptyRecord(t *testing.T) {
	m := map[string]any{
		"topic":  "main",
		"record": []any{},
	}
	topic := topicFromMap(m)
	assert.True(t, topic.HasRecord)
	assert.True(t, topic.RecordValid)
	assert.Empty(t, topic.Record)
}

func TestSectionFromMap_TopicsNonSlice(t *testing.T) {
	m := map[string]any{
		"type":   "tool",
		"topics": "not a slice",
	}
	section := sectionFromMap(m)
	assert.Nil(t, section.Topics)
}

func TestSectionFromMap_RepoEmptySlice(t *testing.T) {
	m := map[string]any{
		"type": "tool",
		"repo": []any{},
	}
	section := sectionFromMap(m)
	assert.Empty(t, section.Repos)
}
