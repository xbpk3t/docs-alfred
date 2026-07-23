package session

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractPrimaryModelCC_LastNonSynthetic(t *testing.T) {
	path := writeTempJSONL(t, ""+
		`{"type":"user","message":{"role":"user","content":"hi"},"timestamp":"2026-01-01T00:00:00Z"}`+"\n"+
		`{"type":"assistant","message":{"role":"assistant","model":"deepseek-v4-flash","content":[{"type":"text","text":"a"}]},"timestamp":"2026-01-01T00:00:01Z"}`+"\n"+
		`{"type":"assistant","message":{"role":"assistant","model":"<synthetic>","content":[{"type":"text","text":"x"}]},"timestamp":"2026-01-01T00:00:02Z"}`+"\n"+
		`{"type":"assistant","message":{"role":"assistant","model":"grok-4.5","content":[{"type":"text","text":"b"}]},"timestamp":"2026-01-01T00:00:03Z"}`+"\n")

	got, err := ExtractPrimaryModelCC(path)
	require.NoError(t, err)
	require.Equal(t, "grok-4.5", got)
}

func TestExtractPrimaryModelCC_None(t *testing.T) {
	path := writeTempJSONL(t, ""+
		`{"type":"user","message":{"role":"user","content":"hi"},"timestamp":"2026-01-01T00:00:00Z"}`+"\n"+
		`{"type":"assistant","message":{"role":"assistant","model":"<synthetic>","content":[{"type":"text","text":"x"}]},"timestamp":"2026-01-01T00:00:01Z"}`+"\n")

	got, err := ExtractPrimaryModelCC(path)
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestExtractPrimaryModelCC_FileNotFound(t *testing.T) {
	_, err := ExtractPrimaryModelCC(filepath.Join(t.TempDir(), "missing.jsonl"))
	require.Error(t, err)
}

func TestExtractPrimaryModelCodex_LastModel(t *testing.T) {
	path := writeTempJSONL(t, ""+
		`{"type":"session_meta","timestamp":"2026-07-03T07:04:28.855Z","payload":{"id":"abc"}}`+"\n"+
		`{"type":"turn_context","timestamp":"2026-07-03T07:04:28.868Z","payload":{"model":"gpt-5.4","cwd":"/tmp"}}`+"\n"+
		`{"type":"response_item","timestamp":"2026-07-03T07:04:28.900Z","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hi"}]}}`+"\n"+
		`{"type":"turn_context","timestamp":"2026-07-03T07:10:00.000Z","payload":{"model":"gpt-5.5","cwd":"/tmp"}}`+"\n")

	got, err := ExtractPrimaryModelCodex(path)
	require.NoError(t, err)
	require.Equal(t, "gpt-5.5", got)
}

func TestExtractPrimaryModelCodex_None(t *testing.T) {
	path := writeTempJSONL(t, ""+
		`{"type":"response_item","timestamp":"2026-07-03T07:04:28.900Z","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}}`+"\n")

	got, err := ExtractPrimaryModelCodex(path)
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestNormalizeModel(t *testing.T) {
	require.Equal(t, "grok-4.5", normalizeModel("  grok-4.5  "))
	require.Empty(t, normalizeModel(""))
	require.Empty(t, normalizeModel("<synthetic>"))
	require.Empty(t, normalizeModel("  <synthetic>  "))
}

func writeTempJSONL(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "session.jsonl")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	return path
}
