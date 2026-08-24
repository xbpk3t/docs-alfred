package ghdata

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTopicFromMap(t *testing.T) {
	m := map[string]any{
		"topic": "overview",
		"kind":  "tools",
		"repo": []any{
			map[string]any{
				"url":   "https://github.com/owner/repo",
				"score": 5,
			},
		},
	}
	topic := topicFromMap(m)
	assert.Equal(t, "overview", topic.Topic)
	assert.Equal(t, "tools", string(topic.Kind))
	require.Len(t, topic.Repo, 1)
	assert.Equal(t, "https://github.com/owner/repo", topic.Repo[0].URL)
	require.NotNil(t, topic.Repo[0].Score)
	assert.Equal(t, 5, *topic.Repo[0].Score)
}

func TestTopicFromMap_NilRecord(t *testing.T) {
	m := map[string]any{
		"topic":  "overview",
		"record": nil,
	}
	topic := topicFromMap(m)
	assert.Equal(t, "overview", topic.Topic)
	assert.Nil(t, topic.Record)
}

func TestTopicFromMap_RepoNonMappingItem(t *testing.T) {
	// mapstructure decodes non-mapping repo items as empty Repo structs; the
	// walker's processYAMLDoc filters non-mapping topic items out before
	// yielding events, so real repos are unaffected at the event level.
	m := map[string]any{
		"topic": "overview",
		"repo":  []any{"just a string", 42},
	}
	topic := topicFromMap(m)
	require.Len(t, topic.Repo, 2)
	assert.Empty(t, topic.Repo[0].URL)
}

func TestTopicFromMap_EmptyMap(t *testing.T) {
	topic := topicFromMap(map[string]any{})
	assert.Empty(t, topic.Topic)
	assert.Empty(t, topic.Repo)
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
