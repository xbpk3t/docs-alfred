package internal

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

const (
	AgentCC    Agent = "cc"
	AgentCodex Agent = "codex"

	SourceClaudeCode = "claude-code"
	SourceCodex      = "codex"

	claudeSessionEnv = "CLAUDE_CODE_SESSION_ID"
	codexThreadEnv   = "CODEX_THREAD_ID"
)

// Agent identifies the coding agent runtime that owns a session transcript.
type Agent string

// SessionRef is the resolved transcript location for a single export.
type SessionRef struct {
	Agent          Agent
	SessionID      string
	TranscriptPath string
	Source         string
}

// ResolveSession resolves an agent session/thread ID to a transcript JSONL file.
func ResolveSession(agent Agent, sessionIDOverride string) (SessionRef, error) {
	switch agent {
	case AgentCC:
		return resolveClaudeSession(sessionIDOverride)
	case AgentCodex:
		return resolveCodexSession(sessionIDOverride)
	default:
		return SessionRef{}, fmt.Errorf("unsupported agent %q", agent)
	}
}

func resolveClaudeSession(sessionIDOverride string) (SessionRef, error) {
	projectDir := getProjectDir()
	sessionID, err := resolveSessionID(sessionIDOverride, claudeSessionEnv)
	if err != nil {
		return SessionRef{}, err
	}

	pathKey := strings.ReplaceAll(projectDir, "/", "-")
	transcriptPath := filepath.Join(os.Getenv("HOME"), ".claude", "projects", pathKey, sessionID+".jsonl")
	if err := requireFile(transcriptPath); err != nil {
		return SessionRef{}, fmt.Errorf("cc transcript %q: %w", transcriptPath, err)
	}

	return SessionRef{
		Agent:          AgentCC,
		SessionID:      sessionID,
		TranscriptPath: transcriptPath,
		Source:         SourceClaudeCode,
	}, nil
}

func resolveCodexSession(sessionIDOverride string) (SessionRef, error) {
	sessionID, err := resolveSessionID(sessionIDOverride, codexThreadEnv)
	if err != nil {
		return SessionRef{}, err
	}

	statePath := codexStatePath()
	rolloutPath, err := lookupCodexRolloutPath(statePath, sessionID)
	if err != nil {
		return SessionRef{}, err
	}
	if err := requireFile(rolloutPath); err != nil {
		return SessionRef{}, fmt.Errorf("codex rollout %q: %w", rolloutPath, err)
	}

	return SessionRef{
		Agent:          AgentCodex,
		SessionID:      sessionID,
		TranscriptPath: rolloutPath,
		Source:         SourceCodex,
	}, nil
}

func resolveSessionID(override, envName string) (string, error) {
	if override != "" {
		return override, nil
	}

	if sessionID := os.Getenv(envName); sessionID != "" {
		return sessionID, nil
	}

	return "", fmt.Errorf("no session ID specified; use --session or set %s", envName)
}

func lookupCodexRolloutPath(statePath, sessionID string) (string, error) {
	if err := requireFile(statePath); err != nil {
		return "", fmt.Errorf("codex state %q: %w", statePath, err)
	}

	db, err := sql.Open("sqlite", statePath)
	if err != nil {
		return "", fmt.Errorf("open codex state: %w", err)
	}
	defer func() { _ = db.Close() }()

	var rolloutPath string
	err = db.QueryRow("SELECT rollout_path FROM threads WHERE id = ?", sessionID).Scan(&rolloutPath)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("codex thread %q not found", sessionID)
	}
	if err != nil {
		return "", fmt.Errorf("query codex thread: %w", err)
	}
	if strings.TrimSpace(rolloutPath) == "" {
		return "", fmt.Errorf("codex thread %q has empty rollout_path", sessionID)
	}

	return rolloutPath, nil
}

func requireFile(path string) error {
	info, err := os.Stat(filepath.Clean(path))
	if err != nil {
		return err
	}
	if info.IsDir() {
		return errors.New("path is a directory")
	}

	return nil
}

func codexStatePath() string {
	return filepath.Join(os.Getenv("HOME"), ".codex", "state_5.sqlite")
}

// getProjectDir returns the project directory.
func getProjectDir() string {
	if projectDir := os.Getenv("CLAUDE_PROJECT_DIR"); projectDir != "" {
		return projectDir
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}

	return cwd
}
