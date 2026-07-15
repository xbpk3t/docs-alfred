package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseCodex_UserAndAssistantMessages(t *testing.T) {
	path := writeCodexRollout(t, `
{"type":"response_item","timestamp":"2026-07-07T07:54:10.897Z","payload":{"type":"message","role":"developer","content":[{"type":"input_text","text":"developer instructions"}]}}
{"type":"response_item","timestamp":"2026-07-07T07:54:10.898Z","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"# AGENTS.md instructions for /tmp/project"},{"type":"input_text","text":"<environment_context>secret</environment_context>"}]}}
{"type":"response_item","timestamp":"2026-07-07T07:54:10.899Z","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"Build this feature"}]}}
{"type":"response_item","timestamp":"2026-07-07T07:54:11.000Z","payload":{"type":"function_call","name":"exec_command","arguments":"{}"}}
{"type":"response_item","timestamp":"2026-07-07T07:54:12.000Z","payload":{"type":"function_call_output","output":"tool output"}}
{"type":"response_item","timestamp":"2026-07-07T07:54:13.000Z","payload":{"type":"reasoning","content":[{"type":"text","text":"private reasoning"}]}}
{"type":"response_item","timestamp":"2026-07-07T07:54:14.000Z","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Implemented it"}]}}
`)

	messages, err := ParseCodex(path)
	require.NoError(t, err)
	require.Equal(t, []Message{
		{Role: roleUser, Content: "Build this feature", Timestamp: "2026-07-07T07:54:10.899Z"},
		{Role: roleAssistant, Content: "Implemented it", Timestamp: "2026-07-07T07:54:14.000Z"},
	}, messages)
}

func TestParseCodex_JoinsTextBlocks(t *testing.T) {
	path := writeCodexRollout(t, `
{"type":"response_item","timestamp":"2026-07-07T07:54:10.899Z","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"First"},{"type":"text","text":"Second"},{"type":"image","text":"ignored"}]}}
`)

	messages, err := ParseCodex(path)
	require.NoError(t, err)
	require.Len(t, messages, 1)
	require.Equal(t, "First\n\nSecond", messages[0].Content)
}

func TestParseCodex_SkipsMalformedLines(t *testing.T) {
	path := writeCodexRollout(t, `
not-json
{"type":"response_item","timestamp":"2026-07-07T07:54:10.899Z","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Valid"}]}}
`)

	messages, err := ParseCodex(path)
	require.NoError(t, err)
	require.Len(t, messages, 1)
	require.Equal(t, "Valid", messages[0].Content)
}

func TestParseCodex_TokenTooLongIsFatal(t *testing.T) {
	prev := jsonlScanMaxToken
	jsonlScanMaxToken = 1024
	t.Cleanup(func() { jsonlScanMaxToken = prev })

	// Oversized single JSONL line; content is large enough to exceed the lowered max token.
	path := writeCodexRollout(t, `{"type":"response_item","timestamp":"2026-07-07T07:54:10.899Z","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"`+strings.Repeat("z", 2048)+`"}]}}`+"\n")

	_, err := ParseCodex(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "scan codex rollout")
	require.Contains(t, err.Error(), "token too long")
}

func writeCodexRollout(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	return path
}
