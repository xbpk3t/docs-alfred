package session

import (
	"strings"

	"github.com/xbpk3t/docs-alfred/pkg/textutil"
)

// Filter removes known noise messages from the parsed list.
//
// Only hard/structural filters are applied:
//   - Empty messages
//   - API error messages (structural: always starts with "API Error:")
//   - System stdout injection (structural: <local-command-stdout> tag)
//   - Stop hook noise (structural: "session-scoped Stop hook" marker)
//
// Content-based heuristics (transition phrases, word patterns) are deliberately
// avoided — they're unreliable across languages and conversation styles.
func Filter(messages []Message) []Message {
	filtered := make([]Message, 0, len(messages))
	for _, msg := range messages {
		if !shouldKeep(msg) {
			continue
		}
		msg.Content = textutil.SanitizeContent(msg.Content)
		if strings.TrimSpace(msg.Content) == "" {
			continue
		}
		filtered = append(filtered, msg)
	}

	return filtered
}

// shouldKeep returns false for messages that are clearly not conversation content.
func shouldKeep(msg Message) bool {
	content := strings.TrimSpace(msg.Content)
	if content == "" {
		return false
	}

	// API error messages have no conversational value
	if strings.HasPrefix(content, "API Error:") {
		return false
	}

	// System command stdout injection — these are tool execution echoes,
	// not user intent (e.g. <local-command-stdout>Goal set: ...)
	if strings.Contains(content, "<local-command-stdout>") {
		return false
	}

	// Stop hook messages are system-generated noise
	if strings.Contains(content, "session-scoped Stop hook") {
		return false
	}

	return true
}
