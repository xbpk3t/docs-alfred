package session

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilter_Noise(t *testing.T) {
	messages, err := Parse("testdata/noise.jsonl")
	require.NoError(t, err)
	require.Len(t, messages, 6)

	filtered := Filter(messages)

	// Only hard filters applied:
	// - user: "我想优化的代码" → kept
	// - assistant: "API Error: 500..." → filtered (structural: "API Error:" prefix)
	// - assistant: "Good question" → kept (no structural filter for this)
	// - assistant: "Now let me look at the code" → kept
	// - assistant: "All right" → kept
	// - assistant: "Sure, here's the fix:\n\n```go\n..." → kept
	require.Len(t, filtered, 5)

	// First message should be the user
	assert.Equal(t, "user", filtered[0].Role)
	assert.Equal(t, "我想优化的代码", filtered[0].Content)

	// The last message should be the one with code
	last := filtered[len(filtered)-1]
	assert.Equal(t, "assistant", last.Role)
	assert.Contains(t, last.Content, "Sure, here's the fix")
	assert.Contains(t, last.Content, "```go")
}

func TestFilter_Empty(t *testing.T) {
	messages := []Message{
		{Role: "user", Content: "  ", Timestamp: "2026-01-01T00:00:00Z"},
		{Role: "assistant", Content: "real content", Timestamp: "2026-01-01T00:00:01Z"},
	}

	filtered := Filter(messages)
	require.Len(t, filtered, 1)
	assert.Equal(t, "real content", filtered[0].Content)
}

func TestFilter_APIError(t *testing.T) {
	messages := []Message{
		{Role: "assistant", Content: "API Error: 429 Too many requests", Timestamp: "2026-01-01T00:00:00Z"},
		{Role: "assistant", Content: "API Error: 500 Internal Server Error", Timestamp: "2026-01-01T00:00:01Z"},
	}

	filtered := Filter(messages)
	assert.Empty(t, filtered)
}

func TestFilter_LocalCommandStdout(t *testing.T) {
	messages := []Message{
		{Role: "user", Content: "real question", Timestamp: "2026-01-01T00:00:00Z"},
		{Role: "user", Content: "<local-command-stdout>Goal set: something", Timestamp: "2026-01-01T00:00:01Z"},
		{Role: "assistant", Content: "real answer", Timestamp: "2026-01-01T00:00:02Z"},
	}
	filtered := Filter(messages)
	require.Len(t, filtered, 2)
	assert.Equal(t, "real question", filtered[0].Content)
	assert.Equal(t, "real answer", filtered[1].Content)
}

func TestFilter_StopHook(t *testing.T) {
	messages := []Message{
		{Role: "assistant", Content: "session-scoped Stop hook: waiting for condition", Timestamp: "2026-01-01T00:00:00Z"},
		{Role: "assistant", Content: "real content", Timestamp: "2026-01-01T00:00:01Z"},
	}
	filtered := Filter(messages)
	require.Len(t, filtered, 1)
	assert.Equal(t, "real content", filtered[0].Content)
}
