package session

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_Simple(t *testing.T) {
	messages, err := Parse("testdata/simple.jsonl")
	require.NoError(t, err)
	require.Len(t, messages, 2)

	assert.Equal(t, "user", messages[0].Role)
	assert.Contains(t, messages[0].Content, "帮我分析一下这个代码的性能瓶颈")
	assert.Equal(t, "2026-06-21T10:00:00Z", messages[0].Timestamp)

	assert.Equal(t, "assistant", messages[1].Role)
	assert.Contains(t, messages[1].Content, "我给你分析一下这段代码的性能瓶颈")
	assert.Equal(t, "2026-06-21T10:00:05Z", messages[1].Timestamp)
}

func TestParse_MultiEvent(t *testing.T) {
	// This JSONL simulates the real pattern:
	// user text → assistant thinking + tool_use → user tool_result → assistant text ×3 → ...
	// The thinking, tool_use, tool_result, and non-dialogue events should be skipped.
	// Assistant text blocks from the SAME event are merged; blocks across events
	// remain as separate Messages (format.go merges them later).
	messages, err := Parse("testdata/multi-event.jsonl")
	require.NoError(t, err)

	// Expected output:
	// 1. user: "这个项目里用了哪些设计模式？"
	// 2. assistant: "我看了项目代码，发现使用了以下几种设计模式："
	// 3. assistant: "1. **策略模式**..."
	// 4. assistant: "另外 main.go 里还用了单例模式初始化数据库连接。"
	require.Len(t, messages, 4)

	assert.Equal(t, "user", messages[0].Role)
	assert.Contains(t, messages[0].Content, "设计模式")
	assert.Equal(t, "2026-06-20T14:00:00Z", messages[0].Timestamp)

	assert.Equal(t, "assistant", messages[1].Role)
	assert.Contains(t, messages[1].Content, "我看了项目代码")
	assert.Equal(t, "2026-06-20T14:00:12Z", messages[1].Timestamp)

	assert.Equal(t, "assistant", messages[2].Role)
	assert.Contains(t, messages[2].Content, "策略模式")
	assert.Equal(t, "2026-06-20T14:00:14Z", messages[2].Timestamp)

	assert.Equal(t, "assistant", messages[3].Role)
	assert.Contains(t, messages[3].Content, "单例模式")
	assert.Equal(t, "2026-06-20T14:00:22Z", messages[3].Timestamp)
}

func TestParse_MetaSkip(t *testing.T) {
	messages, err := Parse("testdata/meta-skip.jsonl")
	require.NoError(t, err)
	require.Len(t, messages, 2)

	assert.Equal(t, "user", messages[0].Role)
	assert.Equal(t, "用户发送的真实消息", messages[0].Content)
	assert.Equal(t, "assistant", messages[1].Role)
	assert.Equal(t, "这是真实回复", messages[1].Content)
}

func TestParse_NoEvents(t *testing.T) {
	// An empty JSONL should return nil, no error
	messages, err := Parse("testdata/empty.jsonl")
	require.NoError(t, err)
	assert.Empty(t, messages)
}

func TestParseAll(t *testing.T) {
	// Parse simple + meta-skip, verify messages are merged and sorted by timestamp
	messages, err := ParseAll([]string{
		"testdata/meta-skip.jsonl",
		"testdata/simple.jsonl",
	})
	require.NoError(t, err)

	// meta-skip: 2 messages (10:00:01, 10:00:05)
	// simple: 2 messages (10:00:00, 10:00:05)
	// merged: 4 messages sorted by timestamp
	require.Len(t, messages, 4)

	// The earliest message should be from simple.jsonl (10:00:00)
	assert.Equal(t, "user", messages[0].Role)
	assert.Contains(t, messages[0].Content, "帮我分析一下")
	assert.Equal(t, "2026-06-21T10:00:00Z", messages[0].Timestamp)

	// Then meta-skip user (10:00:01)
	assert.Equal(t, "用户发送的真实消息", messages[1].Content)

	// The last two are both at 10:00:05
	assert.Equal(t, "2026-06-21T10:00:05Z", messages[2].Timestamp)
	assert.Equal(t, "2026-06-21T10:00:05Z", messages[3].Timestamp)
}
