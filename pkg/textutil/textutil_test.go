package textutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- RemoveEmoji tests ---

func TestRemoveEmojiASCII(t *testing.T) {
	assert.Equal(t, "hello world", RemoveEmoji("hello world"))
}

func TestRemoveEmojiWithEmoji(t *testing.T) {
	result := RemoveEmoji("hello 🌍 world")
	assert.Equal(t, "hello  world", result)
}

func TestRemoveEmojiDingbats(t *testing.T) {
	// Dingbats range 0x2700-0x27BF
	result := RemoveEmoji("test ✂ end")
	assert.Equal(t, "test  end", result)
}

func TestRemoveEmojiMiscSymbols(t *testing.T) {
	// Miscellaneous symbols 0x2600-0x26FF
	result := RemoveEmoji("test ☀ end")
	assert.Equal(t, "test  end", result)
}

func TestRemoveEmojiVariationSelectors(t *testing.T) {
	// Variation selectors U+FE00-U+FE0F
	result := RemoveEmoji("test️ end")
	assert.Equal(t, "test end", result)
}

func TestRemoveEmojiSupplemental(t *testing.T) {
	// Supplemental symbols 0x1F000+
	result := RemoveEmoji("test 🎉🎊 end")
	assert.Equal(t, "test  end", result)
}

func TestRemoveEmojiCJK(t *testing.T) {
	// CJK characters should be preserved
	assert.Equal(t, "你好世界", RemoveEmoji("你好世界"))
}

func TestRemoveEmojiEmpty(t *testing.T) {
	assert.Empty(t, RemoveEmoji(""))
}

func TestRemoveEmojiOnlyEmoji(t *testing.T) {
	result := RemoveEmoji("🎉🎊")
	assert.Empty(t, result)
}

func TestRemoveEmojiLatinExtended(t *testing.T) {
	assert.Equal(t, "café", RemoveEmoji("café"))
}

func TestRemoveEmojiBacktick(t *testing.T) {
	// Backtick U+0060 should be preserved
	assert.Equal(t, "code `block`", RemoveEmoji("code `block`"))
}

func TestRemoveEmojiMixed(t *testing.T) {
	result := RemoveEmoji("Hello 🌍! 你好 ☀️ world 🎉")
	assert.Contains(t, result, "Hello")
	assert.Contains(t, result, "你好")
	assert.Contains(t, result, "world")
	assert.NotContains(t, result, "🌍")
	assert.NotContains(t, result, "🎉")
}

func TestRemoveEmojiMiscSymbolsNonSo(t *testing.T) {
	// U+266F (♯) is in 0x2600-0x26FF but NOT unicode.So, hits the misc symbols range check
	result := RemoveEmoji("note♯here")
	assert.Equal(t, "notehere", result)
}

func TestRemoveEmojiDingbatsNonSo(t *testing.T) {
	// U+2768 (❨) is in 0x2700-0x27BF but NOT unicode.So, hits the dingbats range check
	result := RemoveEmoji("a❨b")
	assert.Equal(t, "ab", result)
}

func TestRemoveEmojiSupplementalNonSo(t *testing.T) {
	// U+1F02C is above 0x1F000 but NOT unicode.So, hits the supplemental range check
	result := RemoveEmoji("a" + string(rune(0x1F02C)) + "b")
	assert.Equal(t, "ab", result)
}

// --- SanitizeContent tests ---

func TestSanitizeContentPlain(t *testing.T) {
	assert.Equal(t, "hello world", SanitizeContent("hello world"))
}

func TestSanitizeContentRemovesEmoji(t *testing.T) {
	result := SanitizeContent("hello 🌍 world")
	assert.NotContains(t, result, "🌍")
	assert.Contains(t, result, "hello")
	assert.Contains(t, result, "world")
}

func TestSanitizeContentStripsSectionHeaders(t *testing.T) {
	result := SanitizeContent("## Some Section\ncontent here")
	assert.NotContains(t, result, "## Some Section")
	assert.Contains(t, result, "Some Section")
	assert.Contains(t, result, "content here")
}

func TestSanitizeContentPreservesUserHeader(t *testing.T) {
	result := SanitizeContent("## User\nmessage")
	assert.Contains(t, result, "## User")
}

func TestSanitizeContentPreservesClaudeHeader(t *testing.T) {
	result := SanitizeContent("## Claude\nresponse")
	assert.Contains(t, result, "## Claude")
}

func TestSanitizeContentCollapsesDoubleSpaces(t *testing.T) {
	result := SanitizeContent("hello  world   test")
	assert.Equal(t, "hello world test", result)
}

func TestSanitizeContentEmpty(t *testing.T) {
	assert.Empty(t, SanitizeContent(""))
}

func TestSanitizeContentMultiline(t *testing.T) {
	input := "line1\n## Header\nline3"
	result := SanitizeContent(input)
	assert.Contains(t, result, "line1")
	assert.Contains(t, result, "Header")
	assert.Contains(t, result, "line3")
}

func TestSanitizeContentPreservesClaudeUserInHeader(t *testing.T) {
	input := "## User Settings\ncontent"
	result := SanitizeContent(input)
	// "## User Settings" starts with "## User" so it's preserved
	assert.Contains(t, result, "## User Settings")
}

// --- SlugFilename tests ---

func TestSlugFilenameNormal(t *testing.T) {
	assert.Equal(t, "hello-world", SlugFilename("Hello World"))
}

func TestSlugFilenameCJK(t *testing.T) {
	result := SlugFilename("你好世界")
	assert.NotEmpty(t, result)
}

func TestSlugFilenameEmpty(t *testing.T) {
	assert.Equal(t, "untitled", SlugFilename(""))
}

func TestSlugFilenameWhitespace(t *testing.T) {
	assert.Equal(t, "untitled", SlugFilename("   "))
}

func TestSlugFilenameSpecialChars(t *testing.T) {
	result := SlugFilename("Hello! @World# 2024")
	assert.NotEmpty(t, result)
	assert.NotContains(t, result, "!")
	assert.NotContains(t, result, "@")
}

func TestSlugFilenameWithNumbers(t *testing.T) {
	result := SlugFilename("test-123")
	assert.Equal(t, "test-123", result)
}

func TestSlugFilenameUnicode(t *testing.T) {
	result := SlugFilename("Ünïcödé")
	assert.NotEmpty(t, result)
}

// --- TruncateUTF8 supplement tests ---

func TestTruncateUTF8ShortString(t *testing.T) {
	assert.Equal(t, "hello", TruncateUTF8("hello", 100))
}

func TestTruncateUTF8ExactBytes(t *testing.T) {
	assert.Equal(t, "hello", TruncateUTF8("hello", 5))
}

func TestTruncateUTF8ZeroMax(t *testing.T) {
	assert.Empty(t, TruncateUTF8("hello", 0))
}

func TestTruncateUTF8NegativeMax(t *testing.T) {
	assert.Empty(t, TruncateUTF8("hello", -1))
}

func TestTruncateUTF8EmptyString(t *testing.T) {
	assert.Empty(t, TruncateUTF8("", 10))
}

func TestTruncateUTF8OneByteOver(t *testing.T) {
	result := TruncateUTF8("hello", 4)
	assert.Equal(t, "hell...", result)
}

func TestTruncateUTF8MultiByteChars(t *testing.T) {
	// "你好" is 6 bytes in UTF-8
	result := TruncateUTF8("你好世界", 7)
	assert.LessOrEqual(t, len(result), 7+3) // +3 for "..."
	assert.NotEmpty(t, result)
}

func TestTruncateUTF8InvalidBytes(t *testing.T) {
	result := TruncateUTF8("\xff\xfe", 10)
	assert.NotContains(t, result, "\xff")
	assert.Contains(t, result, "�")
}

func TestTruncateUTF8LongASCII(t *testing.T) {
	s := "abcdefghijklmnop"
	result := TruncateUTF8(s, 10)
	assert.Equal(t, "abcdefghij...", result)
}
