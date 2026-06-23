package transcript

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
)

func TestTruncateTranscriptKeepsUTF8Valid(t *testing.T) {
	result := truncateTranscript(strings.Repeat("你好", 20), 5)

	assert.True(t, utf8.ValidString(result))
	assert.Contains(t, result, "...")
}

func TestTruncateTranscriptShortContent(t *testing.T) {
	result := truncateTranscript("short", 100)
	assert.Equal(t, "short", result)
}

func TestTruncateTranscriptExactLength(t *testing.T) {
	result := truncateTranscript("exact", 5)
	assert.Equal(t, "exact", result)
}

func TestTruncateTranscriptLongContent(t *testing.T) {
	content := strings.Repeat("word ", 100)
	result := truncateTranscript(content, 50)
	assert.LessOrEqual(t, len(result), 55) // truncated + "..."
	assert.True(t, utf8.ValidString(result))
}

func TestTruncateTranscriptMaxZero(t *testing.T) {
	result := truncateTranscript("content", 0)
	// With maxChars=0, TruncateUTF8 returns empty or "..."
	_ = result // just exercise the code path
}
