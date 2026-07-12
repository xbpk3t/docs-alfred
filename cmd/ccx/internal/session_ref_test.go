package internal

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

	ref, err := ResolveSession(AgentCC, sessionID, projectDir)
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

	ref, err := ResolveSession(AgentCC, "", projectDir)
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

	ref, err := ResolveSession(AgentCodex, sessionID, "")
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

	ref, err := ResolveSession(AgentCodex, "", "")
	require.NoError(t, err)
	require.Equal(t, rolloutPath, ref.TranscriptPath)
}

func TestResolveSession_MissingSessionID(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")

	_, err := ResolveSession(AgentCC, "", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "CLAUDE_CODE_SESSION_ID")
}

func TestResolveSession_CodexThreadNotFound(t *testing.T) {
	homeDir := t.TempDir()
	writeCodexState(t, homeDir, "existing-thread", filepath.Join(homeDir, "rollout.jsonl"))
	t.Setenv("HOME", homeDir)

	_, err := ResolveSession(AgentCodex, "missing-thread", "")
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

// --- Scan fallback tests ---

func TestResolveSession_CCScanFallback_Success(t *testing.T) {
	homeDir := t.TempDir()
	sessionID := "scan-fallback-session"
	projectDir := t.TempDir()

	// Write the session file into a project dir that does NOT match projectDir.
	actualProjectDir := t.TempDir()
	actualPathKey := strings.ReplaceAll(actualProjectDir, "/", "-")
	sessionDir := filepath.Join(homeDir, ".claude", "projects", actualPathKey)
	require.NoError(t, os.MkdirAll(sessionDir, 0o750))
	transcriptPath := filepath.Join(sessionDir, sessionID+".jsonl")
	require.NoError(t, os.WriteFile(transcriptPath, []byte("{}\n"), 0o600))

	t.Setenv("HOME", homeDir)
	t.Setenv("CLAUDE_CODE_SESSION_ID", sessionID)
	// Set a wrong project dir so the direct path fails.
	t.Setenv("CLAUDE_PROJECT_DIR", projectDir)

	ref, err := ResolveSession(AgentCC, "", projectDir)
	require.NoError(t, err)
	require.Equal(t, AgentCC, ref.Agent)
	require.Equal(t, sessionID, ref.SessionID)
	require.Equal(t, transcriptPath, ref.TranscriptPath)
}

func TestResolveSession_CCScanFallback_NotFound(t *testing.T) {
	homeDir := t.TempDir()
	sessionID := "no-such-session"
	projectDir := t.TempDir()

	// Create an unrelated project dir with no matching session file.
	otherPathKey := strings.ReplaceAll(t.TempDir(), "/", "-")
	sessionDir := filepath.Join(homeDir, ".claude", "projects", otherPathKey)
	require.NoError(t, os.MkdirAll(sessionDir, 0o750))

	t.Setenv("HOME", homeDir)
	t.Setenv("CLAUDE_CODE_SESSION_ID", sessionID)
	t.Setenv("CLAUDE_PROJECT_DIR", projectDir)

	_, err := ResolveSession(AgentCC, "", "")
	require.Error(t, err)
	// Should mention the direct path (original error), not the scan error.
	// The scan fallback failed, so the original error is returned.
	require.Contains(t, err.Error(), "cc transcript")
}

func TestResolveSession_CCScanFallback_MultipleMatches(t *testing.T) {
	homeDir := t.TempDir()
	sessionID := "dup-session"
	projectDir := t.TempDir()

	projectsDir := filepath.Join(homeDir, ".claude", "projects")
	require.NoError(t, os.MkdirAll(projectsDir, 0o750))

	// Create TWO project subdirectories, each with the same session file.
	for _, name := range []string{"proj-alpha", "proj-beta"} {
		sd := filepath.Join(projectsDir, name)
		require.NoError(t, os.MkdirAll(sd, 0o750))
		require.NoError(t, os.WriteFile(filepath.Join(sd, sessionID+".jsonl"), []byte("{}\n"), 0o600))
	}

	t.Setenv("HOME", homeDir)
	t.Setenv("CLAUDE_CODE_SESSION_ID", sessionID)
	t.Setenv("CLAUDE_PROJECT_DIR", projectDir)

	_, err := ResolveSession(AgentCC, "", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "found in multiple")
}

func TestResolveSession_CCScanFallback_NoProjectsDir(t *testing.T) {
	homeDir := t.TempDir()
	sessionID := "nobody-home"
	projectDir := t.TempDir()

	t.Setenv("HOME", homeDir)
	t.Setenv("CLAUDE_CODE_SESSION_ID", sessionID)
	t.Setenv("CLAUDE_PROJECT_DIR", projectDir)

	_, err := ResolveSession(AgentCC, "", "")
	require.Error(t, err)
	// Direct path fails, scan fails because projects dir doesn't exist.
	// The scan error is surfaced direct (no extra "resolve session:" prefix).
	require.Contains(t, err.Error(), "scan")
}

func TestDiscoverCodexStatePath_MultiplePicksLatest(t *testing.T) {
	homeDir := t.TempDir()
	codexDir := filepath.Join(homeDir, ".codex")
	require.NoError(t, os.MkdirAll(codexDir, 0o750))

	// Create an older state file.
	oldPath := filepath.Join(codexDir, "state_4.sqlite")
	require.NoError(t, os.WriteFile(oldPath, []byte("old"), 0o600))
	oldTime := time.Now().Add(-2 * time.Hour)
	require.NoError(t, os.Chtimes(oldPath, oldTime, oldTime))

	// Create a newer state file.
	newPath := filepath.Join(codexDir, "state_6.sqlite")
	require.NoError(t, os.WriteFile(newPath, []byte("new"), 0o600))

	t.Setenv("HOME", homeDir)
	got := discoverCodexStatePath()
	require.Equal(t, newPath, got)
}

func TestDiscoverCodexStatePath_FallbackToState5(t *testing.T) {
	homeDir := t.TempDir()
	codexDir := filepath.Join(homeDir, ".codex")
	require.NoError(t, os.MkdirAll(codexDir, 0o750))

	t.Setenv("HOME", homeDir)
	got := discoverCodexStatePath()
	require.Equal(t, filepath.Join(codexDir, "state_5.sqlite"), got)
}

func TestDiscoverCodexStatePath_SkipsNonStateFiles(t *testing.T) {
	homeDir := t.TempDir()
	codexDir := filepath.Join(homeDir, ".codex")
	require.NoError(t, os.MkdirAll(codexDir, 0o750))

	// Create non-matching files.
	require.NoError(t, os.WriteFile(filepath.Join(codexDir, "other.db"), []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(codexDir, "state.json"), []byte("x"), 0o600))

	// Create a valid state file.
	validPath := filepath.Join(codexDir, "state_7.sqlite")
	require.NoError(t, os.WriteFile(validPath, []byte("valid"), 0o600))

	t.Setenv("HOME", homeDir)
	got := discoverCodexStatePath()
	require.Equal(t, validPath, got)
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
