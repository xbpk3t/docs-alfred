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
