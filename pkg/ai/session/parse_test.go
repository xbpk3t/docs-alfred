package session

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	// The thinking, tool_use, tool_result, and non-dialog events should be skipped.
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

func TestParse_FileNotFound(t *testing.T) {
	_, err := Parse("testdata/nonexistent.jsonl")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open session file")
}

func TestParse_MalformedJSONL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.jsonl")
	require.NoError(t, os.WriteFile(path, []byte("not json\n"), 0o600))

	messages, err := Parse(path)
	require.NoError(t, err)
	assert.Empty(t, messages)
}

func TestParseAll_FileNotFound(t *testing.T) {
	_, err := ParseAll([]string{"testdata/nonexistent.jsonl"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse")
}

func TestTryUnmarshalString_String(t *testing.T) {
	raw := json.RawMessage(`"hello"`)
	s, ok := tryUnmarshalString(raw)
	assert.True(t, ok)
	assert.Equal(t, "hello", s)
}

func TestTryUnmarshalString_Array(t *testing.T) {
	raw := json.RawMessage(`[{"type":"text","text":"hello"}]`)
	s, ok := tryUnmarshalString(raw)
	assert.False(t, ok)
	assert.Empty(t, s)
}

func TestTryUnmarshalString_Number(t *testing.T) {
	raw := json.RawMessage(`42`)
	s, ok := tryUnmarshalString(raw)
	assert.False(t, ok)
	assert.Empty(t, s)
}

func TestExtractCommandContent_NoCommandWrapper(t *testing.T) {
	assert.Equal(t, "hello world", extractCommandContent("hello world"))
}

func TestExtractCommandContent_WithCommandArgs(t *testing.T) {
	content := "/test<command-name>test</command-name><command-args>actual args</command-args>"
	assert.Equal(t, "actual args", extractCommandContent(content))
}

func TestExtractCommandContent_WithCommandMessage(t *testing.T) {
	content := "/test<command-name>test</command-name><command-message>the message</command-message>"
	assert.Equal(t, "the message", extractCommandContent(content))
}

func TestExtractCommandContent_EmptyArgsFallsToMessage(t *testing.T) {
	content := "/test<command-name>test</command-name><command-args>  </command-args><command-message>fallback</command-message>"
	assert.Equal(t, "fallback", extractCommandContent(content))
}

func TestExtractCommandContent_CommandNameButNoArgsNoMessage(t *testing.T) {
	content := "/test<command-name>test</command-name>"
	assert.Empty(t, extractCommandContent(content))
}

func TestExtractCommandContent_UnclosedArgs(t *testing.T) {
	content := "/test<command-name>test</command-name><command-args>incomplete"
	assert.Empty(t, extractCommandContent(content))
}

func TestParseUserEvent_ToolResultSkipped(t *testing.T) {
	// tool_result events have array content, which should be skipped
	raw := json.RawMessage(`{"type":"user","timestamp":"2026-06-21T10:00:00Z","message":{"content":[{"type":"tool_result","content":"output"}]}}`)
	msgs := parseUserEvent(raw)
	assert.Empty(t, msgs)
}

func TestParseUserEvent_MetaSkipped(t *testing.T) {
	raw := json.RawMessage(`{"type":"user","timestamp":"2026-06-21T10:00:00Z","message":{"content":"meta msg"},"isMeta":true}`)
	msgs := parseUserEvent(raw)
	assert.Empty(t, msgs)
}

func TestParseUserEvent_ValidText(t *testing.T) {
	raw := json.RawMessage(`{"type":"user","timestamp":"2026-06-21T10:00:00Z","message":{"content":"real message"}}`)
	msgs := parseUserEvent(raw)
	require.Len(t, msgs, 1)
	assert.Equal(t, "user", msgs[0].Role)
	assert.Equal(t, "real message", msgs[0].Content)
}

func TestParseAssistantEvent_ThinkingOnlySkipped(t *testing.T) {
	raw := json.RawMessage(`{"type":"assistant","timestamp":"2026-06-21T10:00:00Z","message":{"content":[{"type":"thinking","text":"thinking..."}]}}`)
	msg := parseAssistantEvent(raw)
	assert.Nil(t, msg)
}

func TestParseAssistantEvent_TextAndThinking(t *testing.T) {
	raw := json.RawMessage(`{"type":"assistant","timestamp":"2026-06-21T10:00:00Z","message":{"content":[{"type":"thinking","text":"hmm"},{"type":"text","text":"answer"}]}}`)
	msg := parseAssistantEvent(raw)
	require.NotNil(t, msg)
	assert.Equal(t, "assistant", msg.Role)
	assert.Equal(t, "answer", msg.Content)
}

func TestParseAssistantEvent_EmptyContentBlocks(t *testing.T) {
	raw := json.RawMessage(`{"type":"assistant","timestamp":"2026-06-21T10:00:00Z","message":{"content":[]}}`)
	msg := parseAssistantEvent(raw)
	assert.Nil(t, msg)
}

func TestParseEvent_UnknownType(t *testing.T) {
	raw := json.RawMessage(`{"type":"system","data":"something"}`)
	msgs := parseEvent(raw)
	assert.Empty(t, msgs)
}

func TestParseEvent_InvalidJSON(t *testing.T) {
	msgs := parseEvent(json.RawMessage(`not json`))
	assert.Empty(t, msgs)
}

func TestFilter_SanitizedToEmpty(t *testing.T) {
	// A message that has content but sanitizes to empty (e.g., only emoji)
	msgs := []Message{
		{Role: "user", Content: "🎉🎊", Timestamp: "2026-01-01T00:00:00Z"},
	}
	filtered := Filter(msgs)
	assert.Empty(t, filtered)
}
