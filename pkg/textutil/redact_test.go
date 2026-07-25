package textutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedactSensitivePaths_Home(t *testing.T) {
	in := "see /Users/luck/Desktop/docs/wiki/AI/log.md please"
	out := RedactSensitivePaths(in)
	assert.Equal(t, "see ~/Desktop/docs/wiki/AI/log.md please", out)
	assert.NotContains(t, out, "/Users/luck")
}

func TestRedactSensitivePaths_HomeLinux(t *testing.T) {
	in := "/home/alice/.config/app"
	out := RedactSensitivePaths(in)
	assert.Equal(t, "~/.config/app", out)
}

func TestRedactSensitivePaths_TmpClaude(t *testing.T) {
	in := "out: /private/tmp/claude-501/-Users-luck-Desktop-docs/bf5cc908/tasks/a.output"
	out := RedactSensitivePaths(in)
	assert.Equal(t, "out: <tmp>", out)
	assert.NotContains(t, out, "/private/tmp")
}

func TestRedactSensitivePaths_ClaudeProjects(t *testing.T) {
	in := "Transcript: /Users/luck/.claude/projects/-Users-luck-Desktop-docs/bf5cc908.jsonl"
	out := RedactSensitivePaths(in)
	// home → ~ first, then .claude data path collapsed
	assert.NotContains(t, out, "/Users/luck")
	assert.NotContains(t, out, ".claude/projects")
	assert.Contains(t, out, "<claude-path>")
}

func TestRedactSensitivePaths_PreservesRepoRelative(t *testing.T) {
	in := "see cmd/ccx/internal/export.go and wiki/AI/LLM/log.md"
	out := RedactSensitivePaths(in)
	assert.Equal(t, in, out)
}

func TestRedactSensitivePaths_Empty(t *testing.T) {
	assert.Empty(t, RedactSensitivePaths(""))
}

func TestDecodeCommonHTMLEntities(t *testing.T) {
	assert.Equal(t, "> quote", DecodeCommonHTMLEntities("&gt; quote"))
	assert.Equal(t, "<tag>", DecodeCommonHTMLEntities("&lt;tag&gt;"))
	assert.Equal(t, `a "b" c`, DecodeCommonHTMLEntities("a &quot;b&quot; c"))
	assert.Equal(t, "a & b", DecodeCommonHTMLEntities("a &amp; b"))
	assert.Equal(t, "it's", DecodeCommonHTMLEntities("it&#39;s"))
	// double-encoded then single pass: &amp;gt; → &gt; (amp first) → still &gt; if only one pass...
	// our order: amp first so &amp;gt; → &gt; → >
	assert.Equal(t, ">", DecodeCommonHTMLEntities("&amp;gt;"))
}

func TestSanitizeContent_RedactsPathsAndEntities(t *testing.T) {
	in := "path /Users/luck/work and &gt; block"
	out := SanitizeContent(in)
	assert.Contains(t, out, "~/work")
	assert.NotContains(t, out, "/Users/luck")
	assert.Contains(t, out, "> block")
	assert.NotContains(t, out, "&gt;")
}
