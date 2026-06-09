package textutil

import "strings"

// TruncateUTF8 returns a valid UTF-8 string no longer than maxBytes plus the
// optional ellipsis. It never slices through a multi-byte rune.
func TruncateUTF8(s string, maxBytes int) string {
	s = strings.ToValidUTF8(s, "\uFFFD")
	if maxBytes <= 0 {
		return ""
	}
	if len(s) <= maxBytes {
		return s
	}

	var b strings.Builder
	b.Grow(maxBytes + len("..."))
	for _, r := range s {
		if b.Len()+len(string(r)) > maxBytes {
			break
		}
		b.WriteRune(r)
	}

	return b.String() + "..."
}
