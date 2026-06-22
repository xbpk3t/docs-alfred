// Package session parses Claude Code session JSONL transcripts into structured messages,
// filters out noise, and formats them as Turn-structured markdown wiki documents.
package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
)

const (
	roleUser      = "user"
	roleAssistant = "assistant"
)

// Message represents a single message in a conversation exchange.
type Message struct {
	Role      string // "user" | "assistant"
	Content   string // clean text content (thinking/tool_use/tool_result already stripped)
	Timestamp string // ISO 8601 timestamp
}

// parseEvent dispatches a single JSONL line to the appropriate handler
// and returns any messages produced.
func parseEvent(raw json.RawMessage) []Message {
	var typeHolder struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &typeHolder); err != nil {
		return nil
	}

	switch typeHolder.Type {
	case roleUser:
		return parseUserEvent(raw)
	case roleAssistant:
		msg := parseAssistantEvent(raw)
		if msg != nil {
			return []Message{*msg}
		}

		return nil
	default:
		// Skip: queue-operation, last-prompt, mode, permission-mode,
		// attachment, file-history-snapshot, system
		return nil
	}
}

// Parse parses a session JSONL file into a slice of Messages.
//
// Only user text (normal text input, not tool_results or isMeta) and
// assistant text blocks are kept. thinking, tool_use, tool_result,
// system events, and metadata events are filtered out at the parse level.
//
// Consecutive assistant text blocks from the same logical turn (interleaved
// with tool_results) are emitted as separate Messages. Callers should merge
// consecutive same-role messages before formatting (see FormatMessages).
func Parse(path string) ([]Message, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open session file: %w", err)
	}
	defer func() { _ = f.Close() }()

	var messages []Message
	scanner := bufio.NewScanner(f)
	// Allow reading lines up to 1MB (JSONL events can be large)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var raw json.RawMessage
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			slog.Warn("skipping malformed JSONL line", "error", err, "line_num", lineNum)

			continue
		}

		messages = append(messages, parseEvent(raw)...)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan session file: %w", err)
	}

	return messages, nil
}

// ParseAll parses multiple JSONL files and merges all messages sorted by timestamp.
// This is used for session chains (main session + sidechains).
func ParseAll(paths []string) ([]Message, error) {
	var all []Message
	for _, p := range paths {
		msgs, err := Parse(p)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", p, err)
		}
		all = append(all, msgs...)
	}

	sort.Slice(all, func(i, j int) bool {
		if all[i].Timestamp != all[j].Timestamp {
			return all[i].Timestamp < all[j].Timestamp
		}
		// Stable tiebreaker: same-role stays together
		return all[i].Role != roleAssistant && all[j].Role == roleAssistant
	})

	return all, nil
}

// --- event parsing -----------------------------------------------------------

// parseUserEvent processes a user-type JSONL event.
//
// Returns messages for:
//   - Normal user text input (content is a string, isMeta is false)
//   - Command-wrapper content (extracted from <command-args>)
//
// Skips:
//   - isMeta events (system-injected messages)
//   - tool_result events (content is a JSON array, not a string)
func parseUserEvent(raw json.RawMessage) []Message {
	var ev struct {
		Timestamp string `json:"timestamp"`
		Message   struct {
			Content json.RawMessage `json:"content"`
		} `json:"message"`
		IsMeta bool `json:"isMeta"`
	}
	if err := json.Unmarshal(raw, &ev); err != nil {
		return nil
	}

	// Skip system-injected meta messages
	if ev.IsMeta {
		return nil
	}

	// Content is a string for normal user input, array for tool_results.
	// Try string first — if it fails, it's not user text.
	content, ok := tryUnmarshalString(ev.Message.Content)
	if !ok {
		return nil // tool_result or other non-text content
	}

	// Extract meaningful content from command wrappers
	content = extractCommandContent(content)
	if content == "" {
		return nil
	}

	return []Message{{
		Role:      roleUser,
		Content:   content,
		Timestamp: ev.Timestamp,
	}}
}

// parseAssistantEvent processes an assistant-type JSONL event.
//
// The assistant message content is an array of content blocks. Only text-type
// blocks are kept; thinking, tool_use, and other block types are skipped.
func parseAssistantEvent(raw json.RawMessage) *Message {
	var ev struct {
		Timestamp string `json:"timestamp"`
		Message   struct {
			Content []json.RawMessage `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(raw, &ev); err != nil {
		return nil
	}

	var textParts []string

	for _, block := range ev.Message.Content {
		var blockType struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(block, &blockType); err != nil {
			continue
		}

		if blockType.Type != "text" {
			continue // skip thinking, tool_use, etc.
		}

		var textBlock struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(block, &textBlock); err != nil {
			continue
		}

		if trimmed := strings.TrimSpace(textBlock.Text); trimmed != "" {
			textParts = append(textParts, trimmed)
		}
	}

	if len(textParts) == 0 {
		return nil
	}

	return &Message{
		Role:      roleAssistant,
		Content:   strings.Join(textParts, "\n\n"),
		Timestamp: ev.Timestamp,
	}
}

// --- helpers -----------------------------------------------------------------

// tryUnmarshalString attempts to unmarshal raw JSON as a string.
// Returns (value, true) on success, ("", false) if the content is not a string
// (e.g., it's a JSON array as with tool_result events).
func tryUnmarshalString(raw json.RawMessage) (string, bool) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, true
	}

	return "", false
}

// extractCommandContent extracts the meaningful user input from a command-wrapped
// message (e.g., "/command-name<command-args>actual args</command-args>").
//
// Returns:
//   - The extracted command-args content if <command-name> is present
//   - The original content if no command wrapper is detected
//   - Empty string if wrapper is detected but no usable content is found
func extractCommandContent(content string) string {
	if !strings.Contains(content, "<command-name>") {
		return content
	}

	// Try <command-args> first (the user's actual arguments)
	if start := strings.Index(content, "<command-args>"); start >= 0 {
		start += len("<command-args>")
		if end := strings.Index(content, "</command-args>"); end > start {
			if args := strings.TrimSpace(content[start:end]); args != "" {
				return args
			}
		}
	}

	// Fallback: extract from <command-message> (full message content)
	if start := strings.Index(content, "<command-message>"); start >= 0 {
		start += len("<command-message>")
		if end := strings.Index(content, "</command-message>"); end > start {
			return strings.TrimSpace(content[start:end])
		}
	}

	return ""
}
