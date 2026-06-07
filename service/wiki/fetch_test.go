package wiki

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMarkdownFallbackBodyConvertsHTML(t *testing.T) {
	body := markdownFallbackBody([]byte(`<html><head><title>Page Title</title></head><body><h1>Hello</h1><p>Read <a href="https://example.com">more</a>.</p></body></html>`))

	assert.Contains(t, body, "Hello")
	assert.Contains(t, body, "[more](https://example.com)")
	assert.False(t, strings.Contains(body, "<h1>"), "fallback body should not be raw HTML")
}
