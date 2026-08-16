package session

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// ErrNoSessionName is returned when a transcript/state contains no session name.
var ErrNoSessionName = errors.New("no session name")

// SessionNameFromCC extracts the session title from a Claude Code transcript.
//
// CC writes title events as the session evolves; multiple events may exist and
// the title may be updated as the conversation focus shifts. The latest (last)
// event is authoritative.
//
// Two event shapes carry the session name, because Claude Code changed the
// event it writes for a user rename:
//
//   - `custom-title` events (field `customTitle`): the current shape, written
//     when the user runs `/rename`. Newer CC builds no longer emit `ai-title`.
//   - `ai-title` events (field `aiTitle`): the legacy shape written by older CC
//     builds as the session evolves.
//
// Both are matched so transcripts from either generation resolve a title; only
// a session with no rename in either shape yields ErrNoSessionName.
func SessionNameFromCC(path string) (string, error) {
	latest := ""

	err := scanJSONLLines(path, "open session file: %w", "scan session file: %w", func(lineNum int, line string) error {
		var ev struct {
			Type        string `json:"type"`
			CustomTitle string `json:"customTitle"`
			AITitle     string `json:"aiTitle"`
		}
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			slog.Warn("skipping malformed JSONL line", "error", err, "line_num", lineNum)
			return nil
		}

		// Match whichever title event shape this CC build writes; the switch
		// keeps the two field names explicit so a future event shape is an
		// obvious addition here rather than a silent extra case.
		var title string
		switch ev.Type {
		case "custom-title":
			title = ev.CustomTitle
		case "ai-title":
			title = ev.AITitle
		}
		if trimmed := strings.TrimSpace(title); trimmed != "" {
			latest = trimmed
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if latest == "" {
		return "", fmt.Errorf("session name: %w", ErrNoSessionName)
	}

	return latest, nil
}

// SessionNameFromCodex extracts the session name from the Codex state database.
//
// Codex derives the title from the first user message at thread creation and
// stores it in the threads table. Users can rename threads in the Codex UI, so
// title (not first_user_message) is authoritative; first_user_message is only a
// fallback if the title is blank.
func SessionNameFromCodex(statePath, threadID string) (string, error) {
	db, err := openCodexState(statePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = db.Close() }()

	var title, firstUserMessage string
	err = db.QueryRow(
		"SELECT title, first_user_message FROM threads WHERE id = ?", threadID,
	).Scan(&title, &firstUserMessage)
	if err != nil {
		return "", fmt.Errorf("query codex thread name: %w", err)
	}

	if trimmed := strings.TrimSpace(title); trimmed != "" {
		return trimmed, nil
	}
	if trimmed := strings.TrimSpace(firstUserMessage); trimmed != "" {
		return trimmed, nil
	}

	return "", fmt.Errorf("codex thread name: %w", ErrNoSessionName)
}

func openCodexState(statePath string) (*sql.DB, error) {
	if err := RequireFile(statePath); err != nil {
		return nil, fmt.Errorf("codex state %q: %w", statePath, err)
	}

	db, err := sql.Open("sqlite", statePath)
	if err != nil {
		return nil, fmt.Errorf("open codex state: %w", err)
	}

	return db, nil
}

// RequireFile checks a path is an existing regular file (not a directory).
func RequireFile(path string) error {
	info, err := os.Stat(filepath.Clean(path))
	if err != nil {
		return err
	}
	if info.IsDir() {
		return errors.New("path is a directory")
	}

	return nil
}
