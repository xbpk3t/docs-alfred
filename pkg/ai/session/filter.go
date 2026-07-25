package session

import (
	"strings"

	"github.com/xbpk3t/docs-alfred/pkg/textutil"
)

// knownHarnessMarkers are structural tags that mark harness noise.
// Presence triggers envelope stripping; pure-harness messages are dropped.
//
// Keep this list explicit and conservative: unknown XML (user configs, etc.) is retained.
var knownHarnessMarkers = []string{
	"task-notification",
	"bash-input",
	"bash-stdout",
	"bash-stderr",
	"local-command-stdout",
	"local-command-caveat",
	"system-reminder",
}

// metaCommandOnly is a user message that is only a Claude Code meta command name
// left after slash-command extraction (e.g. "/model" → "model").
var metaCommandOnly = map[string]struct{}{
	"model": {}, "clear": {}, "compact": {}, "help": {}, "cost": {},
	"config": {}, "doctor": {}, "login": {}, "logout": {}, "memory": {},
	"status": {}, "export": {}, "add-dir": {}, "vim": {}, "terminal-setup": {},
	"ide": {}, "install-github-app": {}, "release-notes": {}, "upgrade": {},
	"privacy-settings": {}, "hooks": {}, "mcp": {}, "files": {}, "context": {},
	"rewind": {}, "plan": {}, "review": {}, "security-review": {}, "init": {},
	"insights": {}, "stats": {}, "tasks": {}, "theme": {}, "voice": {},
	"fast": {}, "effort": {}, "keybindings": {}, "skills": {}, "agents": {},
	"desktop": {}, "usage": {}, "extra-usage": {}, "rate-limit-options": {},
	"teleport": {}, "copy": {}, "exit": {}, "quit": {}, "goal": {},
}

// Filter removes known noise messages from the parsed list.
//
// Only hard/structural filters are applied:
//   - Empty messages
//   - API error messages (structural: always starts with "API Error:")
//   - Harness envelopes (strip blocks; drop if nothing real remains)
//   - Meta command-only leftovers (e.g. bare "model" from /model)
//   - Stop hook noise (structural: "session-scoped Stop hook" marker)
//   - Exact duplicated paste halves inside a single message
//
// Kept messages are sanitized (emoji, section headers, path redact, HTML entities).
func Filter(messages []Message) []Message {
	filtered := make([]Message, 0, len(messages))
	for _, msg := range messages {
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		if !shouldKeepRaw(content) {
			continue
		}

		content = stripHarnessBlocks(content)
		content = stripLeadingMetaCommandLine(content)
		content = textutil.CollapseDuplicateBlocks(content)
		content = textutil.SanitizeContent(content)
		content = strings.TrimSpace(content)
		if content == "" || isMetaCommandOnly(content) {
			continue
		}

		msg.Content = content
		filtered = append(filtered, msg)
	}

	return filtered
}

// shouldKeepRaw returns false for messages that are clearly not conversation
// content before harness stripping / sanitize.
func shouldKeepRaw(content string) bool {
	if strings.HasPrefix(content, "API Error:") {
		return false
	}
	if strings.Contains(content, "session-scoped Stop hook") {
		return false
	}

	return true
}

// stripHarnessBlocks removes known harness XML envelopes.
// Closed <tag>…</tag> blocks are removed; an unclosed open tag eats the rest
// of the string (truncated harness injections). Empty result means "drop message".
//
// Implemented with strings (not RE2 backrefs) so nested same-tag content is
// handled by repeatedly stripping the innermost/first closed pair.
func stripHarnessBlocks(content string) string {
	if !hasHarnessMarker(content) {
		return content
	}

	for changed := true; changed; {
		changed = false
		for _, name := range knownHarnessMarkers {
			open := "<" + name + ">"
			// also allow attributes: <name ...>
			openAttr := "<" + name + " "
			closeTag := "</" + name + ">"

			for {
				start, openLen := indexHarnessOpen(content, open, openAttr)
				if start < 0 {
					break
				}
				end := strings.Index(content[start+openLen:], closeTag)
				if end < 0 {
					// Unclosed: drop from open tag to EOF
					content = strings.TrimSpace(content[:start])
					changed = true
					break
				}
				endAbs := start + openLen + end + len(closeTag)
				content = content[:start] + content[endAbs:]
				changed = true
			}
		}
	}

	return strings.TrimSpace(content)
}

// indexHarnessOpen finds the earliest open tag for a harness marker.
// Returns (index, openLen) or (-1, 0).
func indexHarnessOpen(content, openExact, openAttrPrefix string) (int, int) {
	iExact := strings.Index(content, openExact)
	iAttr := strings.Index(content, openAttrPrefix)

	switch {
	case iExact < 0 && iAttr < 0:
		return -1, 0
	case iExact >= 0 && (iAttr < 0 || iExact <= iAttr):
		return iExact, len(openExact)
	default:
		// Find closing '>' of the open tag with attributes
		gt := strings.IndexByte(content[iAttr:], '>')
		if gt < 0 {
			return iAttr, len(content) - iAttr
		}

		return iAttr, gt + 1
	}
}

func hasHarnessMarker(content string) bool {
	for _, name := range knownHarnessMarkers {
		if strings.Contains(content, "<"+name) {
			return true
		}
	}

	return false
}

func isMetaCommandOnly(content string) bool {
	line := strings.TrimSpace(content)
	if line == "" || strings.ContainsAny(line, " \t\n") {
		return false
	}
	line = strings.TrimPrefix(line, "/")
	_, ok := metaCommandOnly[strings.ToLower(line)]

	return ok
}

// stripLeadingMetaCommandLine drops a leading lone meta-command line
// (e.g. "model\n\nreal user text" → "real user text").
func stripLeadingMetaCommandLine(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return content
	}
	first := strings.TrimSpace(lines[0])
	first = strings.TrimPrefix(first, "/")
	if _, ok := metaCommandOnly[strings.ToLower(first)]; !ok {
		return content
	}
	i := 1
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i >= len(lines) {
		return ""
	}

	return strings.TrimSpace(strings.Join(lines[i:], "\n"))
}
