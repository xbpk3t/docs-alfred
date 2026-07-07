package internal

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveSession_CCUsesOverride(t *testing.T) {
	projectDir := t.TempDir()
	homeDir := t.TempDir()
	sessionID := "cc-session-1"
	transcriptPath := writeClaudeTranscript(t, homeDir, projectDir, sessionID)
	t.Setenv("HOME", homeDir)
	t.Setenv("CLAUDE_PROJECT_DIR", projectDir)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "other-session")

	ref, err := ResolveSession(AgentCC, sessionID)
	require.NoError(t, err)
	require.Equal(t, AgentCC, ref.Agent)
	require.Equal(t, sessionID, ref.SessionID)
	require.Equal(t, transcriptPath, ref.TranscriptPath)
	require.Equal(t, SourceClaudeCode, ref.Source)
}

func TestResolveSession_CCUsesEnv(t *testing.T) {
	projectDir := t.TempDir()
	homeDir := t.TempDir()
	sessionID := "cc-env-session"
	transcriptPath := writeClaudeTranscript(t, homeDir, projectDir, sessionID)
	t.Setenv("HOME", homeDir)
	t.Setenv("CLAUDE_PROJECT_DIR", projectDir)
	t.Setenv("CLAUDE_CODE_SESSION_ID", sessionID)

	ref, err := ResolveSession(AgentCC, "")
	require.NoError(t, err)
	require.Equal(t, transcriptPath, ref.TranscriptPath)
}

func TestResolveSession_CodexUsesStateDB(t *testing.T) {
	homeDir := t.TempDir()
	sessionID := "codex-thread-1"
	rolloutPath := filepath.Join(homeDir, ".codex", "sessions", "rollout-codex-thread-1.jsonl")
	require.NoError(t, os.MkdirAll(filepath.Dir(rolloutPath), 0o750))
	require.NoError(t, os.WriteFile(rolloutPath, []byte("{}\n"), 0o600))
	writeCodexState(t, homeDir, sessionID, rolloutPath)
	t.Setenv("HOME", homeDir)
	t.Setenv("CODEX_THREAD_ID", "other-thread")

	ref, err := ResolveSession(AgentCodex, sessionID)
	require.NoError(t, err)
	require.Equal(t, AgentCodex, ref.Agent)
	require.Equal(t, sessionID, ref.SessionID)
	require.Equal(t, rolloutPath, ref.TranscriptPath)
	require.Equal(t, SourceCodex, ref.Source)
}

func TestResolveSession_CodexUsesEnv(t *testing.T) {
	homeDir := t.TempDir()
	sessionID := "codex-env-thread"
	rolloutPath := filepath.Join(homeDir, ".codex", "sessions", "rollout-codex-env-thread.jsonl")
	require.NoError(t, os.MkdirAll(filepath.Dir(rolloutPath), 0o750))
	require.NoError(t, os.WriteFile(rolloutPath, []byte("{}\n"), 0o600))
	writeCodexState(t, homeDir, sessionID, rolloutPath)
	t.Setenv("HOME", homeDir)
	t.Setenv("CODEX_THREAD_ID", sessionID)

	ref, err := ResolveSession(AgentCodex, "")
	require.NoError(t, err)
	require.Equal(t, rolloutPath, ref.TranscriptPath)
}

func TestResolveSession_MissingSessionID(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")

	_, err := ResolveSession(AgentCC, "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "CLAUDE_CODE_SESSION_ID")
}

func TestResolveSession_CodexThreadNotFound(t *testing.T) {
	homeDir := t.TempDir()
	writeCodexState(t, homeDir, "existing-thread", filepath.Join(homeDir, "rollout.jsonl"))
	t.Setenv("HOME", homeDir)

	_, err := ResolveSession(AgentCodex, "missing-thread")
	require.Error(t, err)
	require.Contains(t, err.Error(), "codex thread")
}

func writeClaudeTranscript(t *testing.T, homeDir, projectDir, sessionID string) string {
	t.Helper()

	pathKey := strings.ReplaceAll(projectDir, "/", "-")
	sessionDir := filepath.Join(homeDir, ".claude", "projects", pathKey)
	require.NoError(t, os.MkdirAll(sessionDir, 0o750))
	transcriptPath := filepath.Join(sessionDir, sessionID+".jsonl")
	content := `{"type":"user","message":{"content":"hello"},"timestamp":"2024-01-15T10:30:00Z"}` + "\n"
	require.NoError(t, os.WriteFile(transcriptPath, []byte(content), 0o600))

	return transcriptPath
}

func writeCodexState(t *testing.T, homeDir, sessionID, rolloutPath string) {
	t.Helper()

	statePath := filepath.Join(homeDir, ".codex", "state_5.sqlite")
	require.NoError(t, os.MkdirAll(filepath.Dir(statePath), 0o750))
	db, err := sql.Open("sqlite", statePath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`CREATE TABLE threads (id TEXT PRIMARY KEY, rollout_path TEXT NOT NULL)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO threads (id, rollout_path) VALUES (?, ?)`, sessionID, rolloutPath)
	require.NoError(t, err)
}
