package ai

import (
	"strings"

	"github.com/bytedance/sonic"
)

// ExtractJSONBraces finds the outermost balanced {...} block in the string.
// This handles cases where AI wraps JSON in markdown code fences or other text.
func ExtractJSONBraces(s string) string {
	for start := strings.IndexByte(s, '{'); start >= 0; {
		if !looksLikeJSONObjectStart(s, start) {
			next := strings.IndexByte(s[start+1:], '{')
			if next < 0 {
				return ""
			}
			start += next + 1

			continue
		}

		if extracted, ok := extractJSONObjectAt(s, start); ok {
			return extracted
		}

		next := strings.IndexByte(s[start+1:], '{')
		if next < 0 {
			return ""
		}
		start += next + 1
	}

	return ""
}

// UnmarshalStrictJSON tries to unmarshal raw JSON; if that fails, it strips
// markdown code fences and extracts the outermost JSON block before retrying.
// Returns the original sonic.Unmarshal error if extraction also fails.
func UnmarshalStrictJSON(raw string, v any) error {
	originalErr := sonic.Unmarshal([]byte(raw), v)
	if originalErr == nil {
		return nil
	}

	// Strip markdown code fences and re-extract.
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	if unmarshalExtractedJSON(cleaned, v) {
		return nil
	}

	// Return the original error if no extracted object can be parsed.
	return originalErr
}

func unmarshalExtractedJSON(raw string, v any) bool {
	for start := strings.IndexByte(raw, '{'); start >= 0; {
		if !looksLikeJSONObjectStart(raw, start) {
			next := strings.IndexByte(raw[start+1:], '{')
			if next < 0 {
				break
			}
			start += next + 1

			continue
		}

		extracted, ok := extractJSONObjectAt(raw, start)
		if ok {
			if err := sonic.Unmarshal([]byte(extracted), v); err == nil {
				return true
			}
		}

		next := strings.IndexByte(raw[start+1:], '{')
		if next < 0 {
			break
		}
		start += next + 1
	}

	return false
}

func extractJSONObjectAt(s string, start int) (string, bool) {
	if start < 0 || start >= len(s) || s[start] != '{' {
		return "", false
	}

	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(s); i++ {
		ch := s[i]
		if inString {
			consumeJSONStringByte(ch, &inString, &escaped)

			continue
		}

		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1], true
			}
		}
	}

	return "", false
}

func looksLikeJSONObjectStart(s string, start int) bool {
	for i := start + 1; i < len(s); i++ {
		switch s[i] {
		case ' ', '\t', '\n', '\r':
			continue
		case '"', '}':
			return true
		default:
			return false
		}
	}

	return false
}

func consumeJSONStringByte(ch byte, inString, escaped *bool) {
	if *escaped {
		*escaped = false

		return
	}

	switch ch {
	case '\\':
		*escaped = true
	case '"':
		*inString = false
	}
}
