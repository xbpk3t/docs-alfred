package domrules

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractTopicEntries_WithTopicsAndSubs(t *testing.T) {
	entry := map[string]any{
		"topics": []any{
			map[string]any{
				"topic": "overview",
				"sub": []any{
					map[string]any{"topic": "install"},
					map[string]any{"topic": "config"},
				},
			},
			map[string]any{
				"topic": "usage",
			},
		},
	}
	topics := extractTopicEntries(entry)
	require.Len(t, topics, 2)
	assert.Equal(t, "overview", topics[0].Topic)
	assert.Equal(t, []string{"install", "config"}, topics[0].Topics)
	assert.Equal(t, "usage", topics[1].Topic)
	assert.Empty(t, topics[1].Topics)
}

func TestExtractTopicEntries_NoTopics(t *testing.T) {
	entry := map[string]any{
		"type": "language",
	}
	topics := extractTopicEntries(entry)
	assert.Empty(t, topics)
}

func TestExtractTopicEntries_EmptyTopics(t *testing.T) {
	entry := map[string]any{
		"topics": []any{},
	}
	topics := extractTopicEntries(entry)
	assert.Empty(t, topics)
}

func TestExtractTopicEntries_NonMappingTopics(t *testing.T) {
	entry := map[string]any{
		"topics": []any{"string", 42},
	}
	topics := extractTopicEntries(entry)
	assert.Empty(t, topics)
}

func TestExtractTopicEntries_TopicWithNilSub(t *testing.T) {
	entry := map[string]any{
		"topics": []any{
			map[string]any{
				"topic": "main",
				"sub":   nil,
			},
		},
	}
	topics := extractTopicEntries(entry)
	require.Len(t, topics, 1)
	assert.Equal(t, "main", topics[0].Topic)
	assert.Nil(t, topics[0].Topics)
}

func TestExtractTopicEntries_SubWithNonMappingEntries(t *testing.T) {
	entry := map[string]any{
		"topics": []any{
			map[string]any{
				"topic": "main",
				"sub":   []any{"string", 42},
			},
		},
	}
	topics := extractTopicEntries(entry)
	require.Len(t, topics, 1)
	assert.Equal(t, "main", topics[0].Topic)
	assert.Empty(t, topics[0].Topics)
}

func TestExtractTopicEntries_EmptyEntry(t *testing.T) {
	entry := map[string]any{}
	topics := extractTopicEntries(entry)
	assert.Empty(t, topics)
}
