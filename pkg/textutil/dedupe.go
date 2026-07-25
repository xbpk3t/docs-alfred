package textutil

import (
	"strings"
	"unicode"
)

// CollapseDuplicateBlocks removes an exact duplicated paste of the same block
// within a single message (common when UI double-inserts user input).
//
// Strategy (conservative):
//  1. Normalize blank lines, then if the text is "A + blank + A" with A equal, keep one A.
//  2. If more than half of non-empty lines are exact duplicates of a contiguous prefix
//     block that repeats once, keep the first occurrence only.
//
// Does not attempt fuzzy/semantic dedupe.
func CollapseDuplicateBlocks(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return content
	}

	if once, ok := collapseExactDouble(content); ok {
		return once
	}

	return content
}

// collapseExactDouble detects content of the form block + separators + same block.
func collapseExactDouble(content string) (string, bool) {
	// Normalize: split on blank-line separators into paragraphs
	parts := splitKeepNonEmptyBlocks(content)
	if len(parts) < 2 {
		return content, false
	}

	// Case: exactly two identical blocks
	if len(parts) == 2 && parts[0] == parts[1] {
		return parts[0], true
	}

	// Case: first half of blocks equals second half (even count, same sequence)
	if len(parts)%2 == 0 {
		half := len(parts) / 2
		same := true
		for i := 0; i < half; i++ {
			if parts[i] != parts[half+i] {
				same = false
				break
			}
		}
		if same {
			return strings.Join(parts[:half], "\n\n"), true
		}
	}

	// Case: whole string is S+S with optional whitespace between (no blank-line split)
	if n := len(content); n >= 40 {
		// try mid split on whitespace run near center
		mid := n / 2
		for delta := 0; delta < 20 && mid-delta > 0 && mid+delta < n; delta++ {
			for _, cut := range []int{mid - delta, mid + delta} {
				left := strings.TrimSpace(content[:cut])
				right := strings.TrimSpace(content[cut:])
				if len(left) >= 20 && left == right {
					return left, true
				}
			}
		}
	}

	return content, false
}

func splitKeepNonEmptyBlocks(content string) []string {
	raw := strings.Split(content, "\n")
	var blocks []string
	var cur []string
	flush := func() {
		if len(cur) == 0 {
			return
		}
		b := strings.TrimSpace(strings.Join(cur, "\n"))
		if b != "" {
			blocks = append(blocks, b)
		}
		cur = cur[:0]
	}
	for _, line := range raw {
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		cur = append(cur, line)
	}
	flush()

	return blocks
}

// FirstLineTitle picks a short title candidate from free text (first non-empty line,
// stripped of markdown bullets / headings, truncated by runes).
func FirstLineTitle(content string, maxRunes int) string {
	for line := range strings.SplitSeq(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		line = strings.TrimLeftFunc(line, func(r rune) bool {
			return r == '#' || r == '-' || r == '*' || unicode.IsSpace(r)
		})
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		runes := []rune(line)
		if maxRunes > 0 && len(runes) > maxRunes {
			return string(runes[:maxRunes])
		}

		return line
	}

	return ""
}
