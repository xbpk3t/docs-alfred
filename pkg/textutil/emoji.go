package textutil

import (
	"strings"
	"unicode"
)

// RemoveEmoji removes emoji characters from a string.
// It strips dingbats, miscellaneous symbols, So category characters,
// variation selectors, and supplemental symbols (0x1F000+).
func RemoveEmoji(s string) string {
	return strings.Map(func(r rune) rune {
		// Keep normal ASCII and common punctuation (including backtick U+0060)
		if r < 0x2600 {
			return r
		}
		// Remove variation selectors (U+FE00-U+FE0F)
		if r >= 0xFE00 && r <= 0xFE0F {
			return -1
		}
		// Remove emoji and symbol ranges (but not Sk which includes backtick)
		if unicode.Is(unicode.So, r) {
			return -1
		}
		// Remove supplemental symbols (0x1F000+)
		if r >= 0x1F000 {
			return -1
		}
		// Remove dingbats (0x2700-0x27BF)
		if r >= 0x2700 && r <= 0x27BF {
			return -1
		}
		// Remove miscellaneous symbols (0x2600-0x26FF)
		if r >= 0x2600 && r <= 0x26FF {
			return -1
		}
		// Keep everything else (CJK, Latin extended, etc.)
		return r
	}, s)
}

// SanitizeContent removes emoji and embedded section headers from message content,
// and collapses multiple consecutive spaces.
func SanitizeContent(content string) string {
	content = RemoveEmoji(content)

	// Strip embedded section headers (## ...) that come from assistant's markdown.
	// These would conflict with ## User / ## Claude headers in the old format.
	// In the new Turn format they're still noise — keep them as plain text by
	// removing the ## prefix rather than dropping the content entirely.
	lines := strings.Split(content, "\n")
	var filtered []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") &&
			!strings.HasPrefix(trimmed, "## User") &&
			!strings.HasPrefix(trimmed, "## Claude") {
			// Replace with the text content only (remove the ## prefix)
			headerText := strings.TrimPrefix(trimmed, "## ")
			filtered = append(filtered, headerText)

			continue
		}
		filtered = append(filtered, line)
	}

	result := strings.Join(filtered, "\n")

	// Collapse multiple spaces on each line (from emoji removal)
	collapsed := make([]string, 0, len(filtered))
	for line := range strings.SplitSeq(result, "\n") {
		for strings.Contains(line, "  ") {
			line = strings.ReplaceAll(line, "  ", " ")
		}
		collapsed = append(collapsed, line)
	}

	return strings.Join(collapsed, "\n")
}
