package ai

import (
	"encoding/json"
	"strings"
)

// ExtractJSONBraces finds the outermost balanced {...} block in the string.
// This handles cases where AI wraps JSON in markdown code fences or other text.
func ExtractJSONBraces(s string) string {
	start := strings.Index(s, "{")
	if start < 0 {
		return ""
	}
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}

	return ""
}

// UnmarshalStrictJSON tries to unmarshal raw JSON; if that fails, it strips
// markdown code fences and extracts the outermost JSON block before retrying.
// Returns the original json.Unmarshal error if extraction also fails.
func UnmarshalStrictJSON(raw string, v any) error {
	if err := json.Unmarshal([]byte(raw), v); err == nil {
		return nil
	}

	// Strip markdown code fences and re-extract.
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	extracted := ExtractJSONBraces(cleaned)
	if extracted == "" {
		// Return the original error if nothing extracted.
		return json.Unmarshal([]byte(raw), v)
	}

	return json.Unmarshal([]byte(extracted), v)
}
