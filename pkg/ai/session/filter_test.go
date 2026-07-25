package session

import (
	"strings"
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

func TestFilter_TaskNotification(t *testing.T) {
	noise := `<task-notification>
<task-id>a398ea1757e78ba4e</task-id>
<summary>Agent "Critic" finished</summary>
<result>some analysis body</result>
</task-notification>`
	messages := []Message{
		{Role: "user", Content: "请分析这段 log", Timestamp: "2026-01-01T00:00:00Z"},
		{Role: "user", Content: noise, Timestamp: "2026-01-01T00:00:01Z"},
		{Role: "assistant", Content: "分析如下", Timestamp: "2026-01-01T00:00:02Z"},
	}
	filtered := Filter(messages)
	require.Len(t, filtered, 2)
	assert.Equal(t, "请分析这段 log", filtered[0].Content)
	assert.Equal(t, "分析如下", filtered[1].Content)
	for _, m := range filtered {
		assert.NotContains(t, m.Content, "task-notification")
	}
}

func TestFilter_BashInputOutput(t *testing.T) {
	noise := `<bash-input>ccx session export --verbose</bash-input>

<bash-stdout>Exported session to /tmp/out.md</bash-stdout><bash-stderr></bash-stderr>`
	messages := []Message{
		{Role: "user", Content: noise, Timestamp: "2026-01-01T00:00:00Z"},
		{Role: "assistant", Content: "导出成功", Timestamp: "2026-01-01T00:00:01Z"},
	}
	filtered := Filter(messages)
	require.Len(t, filtered, 1)
	assert.Equal(t, "导出成功", filtered[0].Content)
}

func TestFilter_SystemReminder(t *testing.T) {
	messages := []Message{
		{Role: "user", Content: "<system-reminder>\nPlan mode is active.\n</system-reminder>", Timestamp: "2026-01-01T00:00:00Z"},
		{Role: "user", Content: "继续实现", Timestamp: "2026-01-01T00:00:01Z"},
	}
	filtered := Filter(messages)
	require.Len(t, filtered, 1)
	assert.Equal(t, "继续实现", filtered[0].Content)
}

func TestFilter_LocalCommandCaveat(t *testing.T) {
	messages := []Message{
		{Role: "user", Content: "<local-command-caveat>Caveat: local command output</local-command-caveat>", Timestamp: "2026-01-01T00:00:00Z"},
		{Role: "user", Content: "真实问题", Timestamp: "2026-01-01T00:00:01Z"},
	}
	filtered := Filter(messages)
	require.Len(t, filtered, 1)
	assert.Equal(t, "真实问题", filtered[0].Content)
}

func TestFilter_DoesNotDropUserXMLConfig(t *testing.T) {
	// Unknown XML should be kept (not aggressive strip)
	content := `<my-config>
  <enabled>true</enabled>
</my-config>`
	messages := []Message{
		{Role: "user", Content: content, Timestamp: "2026-01-01T00:00:00Z"},
	}
	filtered := Filter(messages)
	require.Len(t, filtered, 1)
	assert.Contains(t, filtered[0].Content, "my-config")
}

func TestFilter_RedactsHomePathInKeptMessage(t *testing.T) {
	messages := []Message{
		{Role: "user", Content: "看 /Users/luck/Desktop/docs/wiki/AI/log.md", Timestamp: "2026-01-01T00:00:00Z"},
	}
	filtered := Filter(messages)
	require.Len(t, filtered, 1)
	assert.Contains(t, filtered[0].Content, "~/Desktop/docs/wiki/AI/log.md")
	assert.NotContains(t, filtered[0].Content, "/Users/luck")
}

func TestFilter_MixedHarnessKeepsRealText(t *testing.T) {
	content := "请继续实现 dedupe\n\n<system-reminder>\nPlan mode is active.\n</system-reminder>"
	messages := []Message{
		{Role: "user", Content: content, Timestamp: "2026-01-01T00:00:00Z"},
	}
	filtered := Filter(messages)
	require.Len(t, filtered, 1)
	assert.Contains(t, filtered[0].Content, "请继续实现 dedupe")
	assert.NotContains(t, filtered[0].Content, "system-reminder")
	assert.NotContains(t, filtered[0].Content, "Plan mode")
}

func TestFilter_MetaCommandOnlyDropped(t *testing.T) {
	messages := []Message{
		{Role: "user", Content: "model", Timestamp: "2026-01-01T00:00:00Z"},
		{Role: "user", Content: "真实问题", Timestamp: "2026-01-01T00:00:01Z"},
	}
	filtered := Filter(messages)
	require.Len(t, filtered, 1)
	assert.Equal(t, "真实问题", filtered[0].Content)
}

func TestFilter_LeadingMetaCommandStripped(t *testing.T) {
	content := "model\n\n/Users/luck/Desktop/docs/wiki/AI/LLM/LLM/log.md\n\n来看一下"
	messages := []Message{
		{Role: "user", Content: content, Timestamp: "2026-01-01T00:00:00Z"},
	}
	filtered := Filter(messages)
	require.Len(t, filtered, 1)
	assert.NotContains(t, filtered[0].Content, "model")
	assert.Contains(t, filtered[0].Content, "来看一下")
	assert.Contains(t, filtered[0].Content, "~/Desktop/docs")
}

func TestFilter_CollapseDuplicatePaste(t *testing.T) {
	block := "来看一下\n\n[2026-07-08]\n\n后面的内容\n\n你来做个分析和判断"
	content := block + "\n\n" + block
	messages := []Message{
		{Role: "user", Content: content, Timestamp: "2026-01-01T00:00:00Z"},
	}
	filtered := Filter(messages)
	require.Len(t, filtered, 1)
	assert.Equal(t, block, filtered[0].Content)
	assert.Equal(t, 1, strings.Count(filtered[0].Content, "你来做个分析和判断"))
}
