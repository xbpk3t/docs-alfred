package session

import (
	"encoding/json"
	"log/slog"
	"strings"
)

// syntheticModel is emitted by Claude Code for system/synthetic assistant rows.
// It is never a real model id and must not become primary model metadata.
const syntheticModel = "<synthetic>"

// ExtractPrimaryModelCC scans a Claude Code session JSONL and returns the last
// non-empty, non-synthetic assistant message.model. Returns "" when none found.
// IO/scan failures are returned as errors; missing model is not an error.
func ExtractPrimaryModelCC(path string) (string, error) {
	var last string
	err := scanJSONLLines(path, "open session file: %w", "scan session file: %w", func(lineNum int, line string) error {
		var ev struct {
			Type    string `json:"type"`
			Message struct {
				Model string `json:"model"`
			} `json:"message"`
		}
		// Malformed lines are skipped (same as Parse); only IO/scan errors fail the call.
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			slog.Warn("skipping malformed JSONL line", "error", err, "line_num", lineNum)

			return nil
		}
		if ev.Type != roleAssistant {
			return nil
		}
		if model := normalizeModel(ev.Message.Model); model != "" {
			last = model
		}

		return nil
	})
	if err != nil {
		return "", err
	}

	return last, nil
}

// ExtractPrimaryModelCodex scans a Codex rollout JSONL and returns the last
// non-empty model from turn_context.payload.model. Returns "" when none found.
// IO/scan failures are returned as errors; missing model is not an error.
func ExtractPrimaryModelCodex(path string) (string, error) {
	var last string
	err := scanJSONLLines(path, "open codex rollout: %w", "scan codex rollout: %w", func(lineNum int, line string) error {
		var ev struct {
			Type    string `json:"type"`
			Payload struct {
				Model string `json:"model"`
			} `json:"payload"`
		}
		// Malformed lines are skipped (same as ParseCodex); only IO/scan errors fail.
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			slog.Warn("skipping malformed codex JSONL line", "error", err, "line_num", lineNum)

			return nil
		}
		if ev.Type != "turn_context" {
			return nil
		}
		if model := normalizeModel(ev.Payload.Model); model != "" {
			last = model
		}

		return nil
	})
	if err != nil {
		return "", err
	}

	return last, nil
}

func normalizeModel(model string) string {
	model = strings.TrimSpace(model)
	if model == "" || model == syntheticModel {
		return ""
	}

	return model
}
