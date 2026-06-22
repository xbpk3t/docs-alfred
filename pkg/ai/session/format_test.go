package session

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatMessages_BasicTurn(t *testing.T) {
	messages := []Message{
		{Role: "user", Content: "什么是 CAP 定理？", Timestamp: "2026-06-21T10:00:00Z"},
		{Role: "assistant", Content: "CAP 定理指出分布式系统不能同时满足一致性（Consistency）、可用性（Availability）和分区容错性（Partition Tolerance）这三个保证。", Timestamp: "2026-06-21T10:00:10Z"},
	}

	result := FormatMessages(messages)

	assert.Contains(t, result, "# Turn 1")
	assert.Contains(t, result, "```markdown")
	assert.Contains(t, result, "什么是 CAP 定理？")
	assert.Contains(t, result, "CAP 定理指出分布式系统不能同时满足")
	// Should NOT have a date annotation on the first turn
	assert.NotContains(t, result, "# Turn 1 [")
}

func TestFormatMessages_DateAnnotation(t *testing.T) {
	messages := []Message{
		{Role: "user", Content: "第一天的问题", Timestamp: "2026-06-20T10:00:00Z"},
		{Role: "assistant", Content: "第一天的回答", Timestamp: "2026-06-20T10:00:10Z"},
		{Role: "user", Content: "第二天的问题", Timestamp: "2026-06-21T09:00:00Z"},
		{Role: "assistant", Content: "第二天的回答", Timestamp: "2026-06-21T09:00:10Z"},
	}

	result := FormatMessages(messages)

	assert.Contains(t, result, "# Turn 1")
	assert.Contains(t, result, "# Turn 2 [2026-06-21]")
}

func TestFormatMessages_ConsecutiveAssistantMerge(t *testing.T) {
	// Simulates the real JSONL pattern where a single turn produces
	// multiple assistant text blocks separated by tool_results.
	messages := []Message{
		{Role: "user", Content: "这个项目里用了哪些设计模式？", Timestamp: "2026-06-20T14:00:00Z"},
		{Role: "assistant", Content: "我看了项目代码，发现使用了以下几种设计模式：", Timestamp: "2026-06-20T14:00:12Z"},
		{Role: "assistant", Content: "1. **策略模式** — `pkg/auth` 里有多个\n2. **工厂模式** — 根据类型创建实例", Timestamp: "2026-06-20T14:00:14Z"},
		{Role: "assistant", Content: "另外 `main.go` 里还用了单例模式。", Timestamp: "2026-06-20T14:00:22Z"},
	}

	result := FormatMessages(messages)

	// Should produce ONE turn with all three assistant text blocks merged
	assert.Contains(t, result, "# Turn 1")
	assert.Contains(t, result, "这个项目里用了哪些设计模式？")
	assert.Contains(t, result, "我看了项目代码")
	assert.Contains(t, result, "策略模式")
	assert.Contains(t, result, "单例模式")

	// Should NOT have a Turn 2 (all merged into one turn)
	assert.NotContains(t, result, "# Turn 2")
}

func TestFormatMessages_OrphanAssistant(t *testing.T) {
	// An assistant message at the start (no preceding user) should still render
	messages := []Message{
		{Role: "assistant", Content: "让我来帮你分析这个问题", Timestamp: "2026-06-21T10:00:00Z"},
	}

	result := FormatMessages(messages)
	assert.Contains(t, result, "# Turn 1")
	assert.Contains(t, result, "让我来帮你分析这个问题")
}

func TestFormatMessages_OrphanUser(t *testing.T) {
	// A user message without an assistant response should render in a code block
	messages := []Message{
		{Role: "user", Content: "这是一个单独的问题", Timestamp: "2026-06-21T10:00:00Z"},
	}

	result := FormatMessages(messages)
	assert.Contains(t, result, "# Turn 1")
	assert.Contains(t, result, "```markdown")
	assert.Contains(t, result, "这是一个单独的问题")
	assert.Contains(t, result, "```")
}

func TestFormatMessages_EmptyInput(t *testing.T) {
	result := FormatMessages(nil)
	assert.Empty(t, result)

	result = FormatMessages([]Message{})
	assert.Empty(t, result)
}

func TestFormatMessages_FromMultiEventJSONL(t *testing.T) {
	// End-to-end: parse → filter (all pass) → format
	messages, err := Parse("testdata/multi-event.jsonl")
	require.NoError(t, err)
	require.Len(t, messages, 4) // 1 user + 3 assistant (pre-merge)

	// Format should merge the 3 consecutive assistant messages
	result := FormatMessages(messages)

	// Should be one turn
	assert.Contains(t, result, "# Turn 1")
	assert.NotContains(t, result, "# Turn 2")

	// Should contain all content
	assert.Contains(t, result, "设计模式")
	assert.Contains(t, result, "策略模式")
	assert.Contains(t, result, "单例模式")
	assert.Contains(t, result, "```markdown")
}

func TestFormatMessages_NoDuplicateDateAnnotation(t *testing.T) {
	// Multiple turns on the same day — only Turn 1 should have no date,
	// subsequent turns should also have no date if on the same day.
	messages := []Message{
		{Role: "user", Content: "问题1", Timestamp: "2026-06-21T10:00:00Z"},
		{Role: "assistant", Content: "回答1", Timestamp: "2026-06-21T10:00:10Z"},
		{Role: "user", Content: "问题2", Timestamp: "2026-06-21T10:05:00Z"},
		{Role: "assistant", Content: "回答2", Timestamp: "2026-06-21T10:05:10Z"},
		{Role: "user", Content: "问题3", Timestamp: "2026-06-21T10:10:00Z"},
		{Role: "assistant", Content: "回答3", Timestamp: "2026-06-21T10:10:10Z"},
	}

	result := FormatMessages(messages)
	assert.Contains(t, result, "# Turn 1")
	assert.Contains(t, result, "# Turn 2")
	assert.Contains(t, result, "# Turn 3")
	// No date annotations since all are on the same day
	assert.NotContains(t, result, "[2026-06-21]")
}
