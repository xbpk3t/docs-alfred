package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// ParseCodex parses a Codex rollout JSONL file into conversation messages.
func ParseCodex(path string) ([]Message, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open codex rollout: %w", err)
	}
	defer func() { _ = f.Close() }()

	var messages []Message
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		msg, ok, err := parseCodexLine([]byte(line))
		if err != nil {
			slog.Warn("skipping malformed codex JSONL line", "error", err, "line_num", lineNum)

			continue
		}
		if ok {
			messages = append(messages, msg)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan codex rollout: %w", err)
	}

	return messages, nil
}

func parseCodexLine(line []byte) (Message, bool, error) {
	var ev struct {
		Type      string `json:"type"`
		Timestamp string `json:"timestamp"`
		Payload   struct {
			Type    string          `json:"type"`
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(line, &ev); err != nil {
		return Message{}, false, err
	}

	if ev.Type != "response_item" || ev.Payload.Type != "message" {
		return Message{}, false, nil
	}
	if ev.Payload.Role != roleUser && ev.Payload.Role != roleAssistant {
		return Message{}, false, nil
	}

	content := extractCodexText(ev.Payload.Content)
	if content == "" {
		return Message{}, false, nil
	}

	return Message{
		Role:      ev.Payload.Role,
		Content:   content,
		Timestamp: ev.Timestamp,
	}, true, nil
}

func extractCodexText(raw json.RawMessage) string {
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return ""
	}

	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if !isCodexTextBlock(block.Type) {
			continue
		}

		text := strings.TrimSpace(block.Text)
		if text == "" || isInjectedCodexContext(text) {
			continue
		}

		parts = append(parts, text)
	}

	return strings.Join(parts, "\n\n")
}

func isCodexTextBlock(blockType string) bool {
	switch blockType {
	case "input_text", "output_text", "text":
		return true
	default:
		return false
	}
}

func isInjectedCodexContext(text string) bool {
	return strings.HasPrefix(text, "<permissions instructions>") ||
		strings.HasPrefix(text, "<collaboration_mode>") ||
		strings.HasPrefix(text, "<skills_instructions>") ||
		strings.HasPrefix(text, "# AGENTS.md instructions") ||
		strings.HasPrefix(text, "<environment_context>")
}
