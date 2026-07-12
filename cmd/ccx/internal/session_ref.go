package internal

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/samber/mo"
	_ "modernc.org/sqlite"
)

// projectsBase returns $HOME/.claude/projects.
func projectsBase() string {
	return filepath.Join(os.Getenv("HOME"), ".claude", "projects")
}

const (
	AgentCC    Agent = "cc"
	AgentCodex Agent = "codex"

	SourceClaudeCode = "claude-code"
	SourceCodex      = "codex"

	claudeSessionEnv = "CLAUDE_CODE_SESSION_ID"
	codexThreadEnv   = "CODEX_THREAD_ID"
)

// maxScanDisplayMatches is the maximum number of matching paths shown
// in the error message when findSessionByScan finds multiple matches.
const maxScanDisplayMatches = 10

// errSessionNotFound is returned by findSessionByScan when no project
// directory contains the requested session file. It lets callers
// distinguish "not found anywhere" from other scan errors.
var errSessionNotFound = errors.New("session not found")

// Agent identifies the coding agent runtime that owns a session transcript.
type Agent string

// SessionRef is the resolved transcript location for a single export.
type SessionRef struct {
	Agent          Agent
	SessionID      string
	TranscriptPath string
	Source         string
}

// DetectAgent detects the agent runtime from environment variables.
// Returns the detected agent and the resolved session ID from the env var.
// Caller may override the session ID via input.SessionID before calling ResolveSession.
func DetectAgent(sessionOverride string) (Agent, string, error) {
	cc := envVar(claudeSessionEnv)
	cx := envVar(codexThreadEnv)

	id := sessionOverride
	pick := func(v mo.Option[string], agent Agent) (Agent, string, error) {
		if id == "" {
			id = v.MustGet()
		}
		return agent, id, nil
	}

	switch {
	case cc.IsPresent() && cx.IsAbsent():
		return pick(cc, AgentCC)
	case cx.IsPresent() && cc.IsAbsent():
		return pick(cx, AgentCodex)
	case cc.IsPresent() && cx.IsPresent():
		return "", "", fmt.Errorf("ambiguous: both %s and %s are set", claudeSessionEnv, codexThreadEnv)
	default:
		if sessionOverride != "" {
			return "", "", fmt.Errorf("no session ID found; use --agent to specify cc or codex for session %q", sessionOverride)
		}
		return "", "", fmt.Errorf("no session ID found; set %s or %s, or use --agent", claudeSessionEnv, codexThreadEnv)
	}
}

// envVar returns os.Getenv(name) as a mo.Option.
// Returns None when the variable is unset or empty.
func envVar(name string) mo.Option[string] {
	if v := os.Getenv(name); v != "" {
		return mo.Some(v)
	}
	return mo.None[string]()
}

// ResolveSession resolves an agent session/thread ID to a transcript JSONL file.
// projectDir is the resolved project directory (from GetProjectDir), called once
// by the CLI layer and threaded through to avoid multiple os.Getwd calls.
func ResolveSession(agent Agent, sessionIDOverride, projectDir string) (SessionRef, error) {
	switch agent {
	case AgentCC:
		return resolveClaudeSession(sessionIDOverride, projectDir)
	case AgentCodex:
		return resolveCodexSession(sessionIDOverride)
	default:
		return SessionRef{}, fmt.Errorf("unsupported agent %q", agent)
	}
}

func resolveClaudeSession(sessionIDOverride, projectDir string) (SessionRef, error) {
	sessionID, err := resolveSessionID(sessionIDOverride, claudeSessionEnv)
	if err != nil {
		return SessionRef{}, err
	}

	pathKey := strings.ReplaceAll(projectDir, "/", "-")
	transcriptPath := filepath.Join(os.Getenv("HOME"), ".claude", "projects", pathKey, sessionID+".jsonl")
	if err := requireFile(transcriptPath); err != nil {
		// Direct path failed - try scanning all project directories.
		found, scanErr := findSessionByScan(sessionID)
		if scanErr != nil {
			if errors.Is(scanErr, errSessionNotFound) {
				// Session genuinely not found anywhere - original
				// direct-path error is more informative.
				return SessionRef{}, fmt.Errorf("cc transcript %q: %w", transcriptPath, err)
			}
			// Multiple matches or structural scan error - surface that.
			return SessionRef{}, fmt.Errorf("%w", scanErr)
		}
		transcriptPath = found
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

	statePath := discoverCodexStatePath()
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

// findSessionByScan scans all project directories under ~/.claude/projects/
// for a file named <sessionID>.jsonl. Returns the path if exactly one match
// is found. Errors if zero or multiple matches exist.
func findSessionByScan(sessionID string) (string, error) {
	base := projectsBase()

	entries, err := os.ReadDir(base)
	if err != nil {
		return "", fmt.Errorf("scan %s: %w", base, err)
	}

	var matches []string
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		candidate := filepath.Join(base, entry.Name(), sessionID+".jsonl")
		if err := requireFile(candidate); err == nil {
			matches = append(matches, candidate)
		}
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("session %q not found in %s: %w", sessionID, base, errSessionNotFound)
	case 1:
		return matches[0], nil
	default:
		display := matches
		if len(display) > maxScanDisplayMatches {
			display = display[:maxScanDisplayMatches]
		}
		return "", fmt.Errorf("session %q found in multiple project directories:\n%s\n(%d total matches)",
			sessionID, strings.Join(display, "\n"), len(matches))
	}
}

// discoverCodexStatePath finds the most recent Codex state database by scanning
// ~/.codex/ for state_*.sqlite files. Falls back to state_5.sqlite if none found.
func discoverCodexStatePath() string {
	base := filepath.Join(os.Getenv("HOME"), ".codex")
	entries, err := os.ReadDir(base)
	if err != nil {
		return filepath.Join(base, "state_5.sqlite")
	}

	type candidate struct {
		info os.FileInfo
		path string
	}
	var candidates []candidate
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "state_") || !strings.HasSuffix(e.Name(), ".sqlite") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		candidates = append(candidates, candidate{
				info: info,
				path: filepath.Join(base, e.Name()),
			})
	}

	if len(candidates) == 0 {
		return filepath.Join(base, "state_5.sqlite")
	}

	sort.Slice(candidates, func(i, j int) bool {
		ti := candidates[i].info.ModTime()
		tj := candidates[j].info.ModTime()
		if !ti.Equal(tj) {
			return ti.After(tj)
		}
		return candidates[i].path > candidates[j].path // tie-break by name
	})

	return candidates[0].path
}

// GetProjectDir returns the project directory, used for session resolution
// and wiki-root path canonicalization. First checks CLAUDE_PROJECT_DIR env var,
// falls back to the current working directory.
func GetProjectDir() string {
	if projectDir := os.Getenv("CLAUDE_PROJECT_DIR"); projectDir != "" {
		return projectDir
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}

	return cwd
}
