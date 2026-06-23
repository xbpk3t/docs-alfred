package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/cmd/ccx/internal"
)

// --- helpers ---

// setupFakeSession creates a minimal JSONL session file in a temporary
// directory tree that mirrors the real ~/.claude/projects layout.
// It sets HOME, CLAUDE_PROJECT_DIR, and clears CLAUDE_CODE_SESSION_ID.
func setupFakeSession(t *testing.T, sessionID string) string {
	t.Helper()

	homeDir := t.TempDir()
	projectDir := t.TempDir()

	t.Setenv("HOME", homeDir)
	t.Setenv("CLAUDE_PROJECT_DIR", projectDir)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")

	pathKey := strings.ReplaceAll(projectDir, "/", "-")
	sessionDir := filepath.Join(homeDir, ".claude", "projects", pathKey)
	require.NoError(t, os.MkdirAll(sessionDir, 0o750))

	// Minimal valid session JSONL: a user message with timestamp.
	// This satisfies extractDisplay (type=user + content) and
	// extractStartedAt (timestamp field).
	content := `{"type":"user","message":{"content":"Test session message for unit test"},"timestamp":"2024-01-15T10:30:00Z"}
`
	transcriptPath := filepath.Join(sessionDir, sessionID+".jsonl")
	require.NoError(t, os.WriteFile(transcriptPath, []byte(content), 0o600))

	return projectDir
}

// withBrokenStdout replaces os.Stdout with a closed pipe so all writes
// fail immediately, exercising error-return paths in Fprintf-based functions.
func withBrokenStdout(t *testing.T, fn func() error) error {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	require.NoError(t, w.Close()) // close write end; subsequent writes fail

	fnErr := fn()

	os.Stdout = old
	_ = r.Close()

	return fnErr
}

// --- writeRaw: Fprintf error path (session_chain.go:62-64) ---

func TestWriteRaw_FprintfError(t *testing.T) {
	chain := []internal.ChainRecord{
		{
			SessionID:      "test-sess",
			StartedAt:      "2024-01-15T10:30:00Z",
			Display:        "test",
			TranscriptPath: "/tmp/test.jsonl",
		},
	}

	err := withBrokenStdout(t, func() error {
		return writeRaw(chain)
	})

	require.Error(t, err)
}

func TestWriteRaw_SingleEntry(t *testing.T) {
	prev := "prev-session"
	ended := "2025-06-01T12:00:00Z"
	chain := []internal.ChainRecord{
		{
			SessionID:      "sess-main",
			PrevSessionID:  &prev,
			StartedAt:      "2025-06-01T10:00:00Z",
			EndedAt:        &ended,
			Display:        "main display",
			TranscriptPath: "/tmp/main.jsonl",
			IsSidechain:    false,
		},
	}

	output, err := captureStdout(t, func() error {
		return writeRaw(chain)
	})
	require.NoError(t, err)

	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	require.Len(t, lines, 1)
	fields := strings.Split(lines[0], "\t")
	require.Len(t, fields, 7)
	assert.Equal(t, "sess-main", fields[0])
	assert.Equal(t, "prev-session", fields[1])
	assert.Equal(t, "2025-06-01T10:00:00Z", fields[2])
	assert.Equal(t, "2025-06-01T12:00:00Z", fields[3])
	assert.Equal(t, "main display", fields[4])
	assert.Equal(t, "/tmp/main.jsonl", fields[5])
	assert.Equal(t, "false", fields[6])
}

// --- writeHumanReadable: Fprintf error paths (session_chain.go:71-73, 82-84, 85-87, 88-90) ---

func TestWriteHumanReadable_FprintfError_EmptyChain(t *testing.T) {
	// With an empty chain, only the header Fprintf is attempted.
	err := withBrokenStdout(t, func() error {
		return writeHumanReadable([]internal.ChainRecord{})
	})

	require.Error(t, err)
}

func TestWriteHumanReadable_FprintfError_NonEmptyChain(t *testing.T) {
	// With a non-empty chain, the header Fprintf fails first.
	chain := []internal.ChainRecord{
		{
			SessionID:      "test-sess",
			StartedAt:      "2024-01-15T10:30:00Z",
			Display:        "test display",
			TranscriptPath: "/tmp/test.jsonl",
		},
	}

	err := withBrokenStdout(t, func() error {
		return writeHumanReadable(chain)
	})

	require.Error(t, err)
}

func TestWriteHumanReadable_SingleEntryWithSidechain(t *testing.T) {
	chain := []internal.ChainRecord{
		{
			SessionID:      "sess-side",
			StartedAt:      "2025-06-01T10:00:00Z",
			Display:        "sidechain display",
			TranscriptPath: "/tmp/side.jsonl",
			IsSidechain:    true,
		},
	}

	output, err := captureStdout(t, func() error {
		return writeHumanReadable(chain)
	})
	require.NoError(t, err)

	assert.Contains(t, output, "Session Chain (1 entries):")
	assert.Contains(t, output, "[sidechain]")
	assert.Contains(t, output, "sidechain display")
}

// --- writeJSON: additional coverage ---

func TestWriteJSON_ThreeEntries(t *testing.T) {
	prev1 := "sess-0"
	prev2 := "sess-1"
	ended := "2025-06-01T12:00:00Z"
	chain := []internal.ChainRecord{
		{SessionID: "sess-0", StartedAt: "2025-06-01T08:00:00Z", Display: "first", TranscriptPath: "/tmp/0.jsonl"},
		{SessionID: "sess-1", PrevSessionID: &prev1, StartedAt: "2025-06-01T09:00:00Z", Display: "second", TranscriptPath: "/tmp/1.jsonl"},
		{SessionID: "sess-2", PrevSessionID: &prev2, StartedAt: "2025-06-01T10:00:00Z", EndedAt: &ended, Display: "third", TranscriptPath: "/tmp/2.jsonl", IsSidechain: true},
	}

	output, err := captureStdout(t, func() error {
		return writeJSON(chain)
	})
	require.NoError(t, err)
	assert.Contains(t, output, "sess-0")
	assert.Contains(t, output, "sess-1")
	assert.Contains(t, output, "sess-2")
}

// --- writeExportResult / writeLines: additional + error paths ---

func TestWriteExportResult_DryRunWithEmptyPaths(t *testing.T) {
	result := &internal.ExportResult{
		OutputPath: "",
		TopicPath:  "",
		Title:      "",
		EngTitle:   "",
		DryRun:     true,
	}

	output, err := captureStdout(t, func() error {
		return writeExportResult(result)
	})
	require.NoError(t, err)
	assert.Contains(t, output, "Dry run")
}

func TestWriteExportResult_NormalWithPaths(t *testing.T) {
	result := &internal.ExportResult{
		OutputPath: "/wiki/topic/file.md",
		TopicPath:  "topic/sub",
		Title:      "My Title",
		EngTitle:   "my-title",
		DryRun:     false,
	}

	output, err := captureStdout(t, func() error {
		return writeExportResult(result)
	})
	require.NoError(t, err)
	assert.Contains(t, output, "Exported session to /wiki/topic/file.md")
	assert.NotContains(t, output, "Dry run")
}

func TestWriteLines_FprintfError(t *testing.T) {
	result := &internal.ExportResult{
		OutputPath: "/tmp/test.md",
		TopicPath:  "dev/go",
		Title:      "Test Title",
		EngTitle:   "test-title",
	}

	err := withBrokenStdout(t, func() error {
		return writeLines("Exported to %s", result)
	})

	require.Error(t, err)
}

func TestWriteLines_MultipleFields(t *testing.T) {
	result := &internal.ExportResult{
		OutputPath: "/a/b/c.md",
		TopicPath:  "dev/go",
		Title:      "Go Testing",
		EngTitle:   "go-testing",
	}

	output, err := captureStdout(t, func() error {
		return writeLines("Written: %s", result)
	})
	require.NoError(t, err)

	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	require.Len(t, lines, 4)
	assert.Equal(t, "Written: /a/b/c.md", lines[0])
	assert.Equal(t, "Topic: dev/go", lines[1])
	assert.Equal(t, "Title: Go Testing", lines[2])
	assert.Equal(t, "EngTitle: go-testing", lines[3])
}

// --- newSessionChainCmd RunE via command execution ---

func TestNewSessionChainCmd_RunE_JsonFlag(t *testing.T) {
	cmd := newSessionChainCmd()
	require.True(t, cmd.Flags().HasFlags())
	require.NotNil(t, cmd.Flags().Lookup("json"))
	require.NotNil(t, cmd.Flags().Lookup("raw"))
	require.NotNil(t, cmd.Flags().Lookup("session"))
}

func TestNewSessionChainCmd_Execute_DefaultOutput(t *testing.T) {
	sessionID := "test-chain-default"
	setupFakeSession(t, sessionID)

	cmd := NewSessionCmd()
	cmd.SetArgs([]string{"chain", "--session", sessionID})

	output, err := captureStdout(t, func() error {
		return cmd.Execute()
	})

	require.NoError(t, err)
	assert.Contains(t, output, "Session Chain")
	assert.Contains(t, output, sessionID)
}

func TestNewSessionChainCmd_Execute_JSONOutput(t *testing.T) {
	sessionID := "test-chain-json"
	setupFakeSession(t, sessionID)

	cmd := NewSessionCmd()
	cmd.SetArgs([]string{"chain", "--session", sessionID, "--json"})

	output, err := captureStdout(t, func() error {
		return cmd.Execute()
	})

	require.NoError(t, err)

	var chain []internal.ChainRecord
	require.NoError(t, json.Unmarshal([]byte(output), &chain))
	require.NotEmpty(t, chain)
	assert.Equal(t, sessionID, chain[0].SessionID)
}

func TestNewSessionChainCmd_Execute_RawOutput(t *testing.T) {
	sessionID := "test-chain-raw"
	setupFakeSession(t, sessionID)

	cmd := NewSessionCmd()
	cmd.SetArgs([]string{"chain", "--session", sessionID, "--raw"})

	output, err := captureStdout(t, func() error {
		return cmd.Execute()
	})

	require.NoError(t, err)

	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	require.NotEmpty(t, lines)
	fields := strings.Split(lines[0], "\t")
	require.Len(t, fields, 7)
	assert.Equal(t, sessionID, fields[0])
}

func TestNewSessionChainCmd_Execute_NoSessionID(t *testing.T) {
	projectDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("CLAUDE_PROJECT_DIR", projectDir)
	t.Setenv("HOME", homeDir)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")

	cmd := NewSessionCmd()
	cmd.SetArgs([]string{"chain"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "walk session chain")
}

func TestNewSessionChainCmd_Execute_NonexistentSessionFile(t *testing.T) {
	// When the session file does not exist, WalkSessionChain still succeeds
	// by using fallback values (display=sessionID, startedAt=now).
	projectDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("CLAUDE_PROJECT_DIR", projectDir)
	t.Setenv("HOME", homeDir)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")

	cmd := NewSessionCmd()
	cmd.SetArgs([]string{"chain", "--session", "nonexistent-id"})

	output, err := captureStdout(t, func() error {
		return cmd.Execute()
	})

	require.NoError(t, err)
	assert.Contains(t, output, "Session Chain")
	assert.Contains(t, output, "nonexistent-id")
}

// --- newSessionExportCmd RunE via command execution ---

func TestNewSessionExportCmd_RunE_Flags(t *testing.T) {
	cmd := newSessionExportCmd()
	require.NotNil(t, cmd.Flags().Lookup("config"))
	require.NotNil(t, cmd.Flags().Lookup("dry-run"))
	require.NotNil(t, cmd.Flags().Lookup("verbose"))
	require.NotNil(t, cmd.Flags().Lookup("wiki-root"))
	require.NotNil(t, cmd.Flags().Lookup("output-dir"))
	require.NotNil(t, cmd.Flags().Lookup("session"))
}

func TestNewSessionExportCmd_Execute_ConfigLoadError(t *testing.T) {
	cmd := NewSessionCmd()
	cmd.SetArgs([]string{"export", "--config", "/nonexistent/path/config.yml"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load config")
}

func TestNewSessionExportCmd_Execute_WikiRootValidationError(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("CCX_WIKI_ROOT", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("CCX_AI_BASE_URL", "")
	t.Setenv("CCX_AI_MODEL", "")

	cmd := NewSessionCmd()
	cmd.SetArgs([]string{"export", "--wiki-root", "/nonexistent/wiki/root"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "export session")
}

func TestNewSessionExportCmd_Execute_SessionChainError(t *testing.T) {
	// Valid wiki root so validateExportInput passes, but nonexistent session
	// causes extractAndMergeSessions to fail because the transcript file
	// referenced by the chain does not exist on disk.
	projectDir := t.TempDir()
	wikiRoot := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("CLAUDE_PROJECT_DIR", projectDir)
	t.Setenv("HOME", homeDir)
	t.Setenv("CCX_WIKI_ROOT", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("CCX_AI_BASE_URL", "")
	t.Setenv("CCX_AI_MODEL", "")

	cmd := NewSessionCmd()
	cmd.SetArgs([]string{"export", "--wiki-root", wikiRoot, "--session", "nonexistent-session"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "export session")
}
