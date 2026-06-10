package transcript

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeTranscriptContentHTML(t *testing.T) {
	got := normalizeTranscriptContent(`<html><body><h1>Transcript</h1><p>Hello <strong>world</strong>.</p></body></html>`, htmlContentType)

	assert.Contains(t, got, "Transcript")
	assert.Contains(t, got, "Hello")
	assert.Contains(t, got, "world")
	assert.False(t, strings.Contains(got, "<p>"), "HTML transcripts should be converted to Markdown")
}

func TestDetectTranscriptContentType(t *testing.T) {
	assert.Equal(t, srtContentType,
		detectTranscriptContentType("https://example.com/transcript", "application/x-subrip; charset=utf-8", nil))
	assert.Equal(t, jsonContentType,
		detectTranscriptContentType("https://example.com/transcript.json", "", []byte("not json, but URL wins")))
	assert.Equal(t, htmlContentType,
		detectTranscriptContentType("https://example.com/transcript", "", []byte(`<!doctype html><html><body>Transcript</body></html>`)))
}

func TestIsTranscriptURLHTMLRequiresTranscriptHint(t *testing.T) {
	assert.True(t, isTranscriptURL("https://example.com/transcript.html"))
	assert.True(t, isTranscriptURL("https://example.com/file.vtt"))
	assert.False(t, isTranscriptURL("https://example.com/article.html"))
}
