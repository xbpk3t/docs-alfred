package textutil

import (
	"html"
	"regexp"
)

// homePathRe matches /Users/<name> or /home/<name> path prefixes.
var homePathRe = regexp.MustCompile(`(?i)(/Users|/home)/[^/\s"'` + "`" + `]+`)

// tmpClaudeRe matches Claude harness temp dirs under /tmp or /private/tmp.
var tmpClaudeRe = regexp.MustCompile(`(?i)(/private)?/tmp/claude-[^\s"'` + "`" + `]*`)

// claudeDataRe matches session/project/task/plan paths under .claude.
// After home redaction these often look like ~/.claude/projects/...
var claudeDataRe = regexp.MustCompile(
	`(?i)(?:~|/[^/\s"'` + "`" + `]*)?/\.claude/(?:projects|session-env|file-history|tasks|plans)/[^\s"'` + "`" + `]*`,
)

// RedactSensitivePaths masks home directories, Claude tmp dirs, and .claude
// session/project paths while leaving repo-relative paths intact.
//
//	/Users/alice/work/repo  →  ~/work/repo
//	/private/tmp/claude-…   →  <tmp>
//	~/.claude/projects/…    →  <claude-path>
func RedactSensitivePaths(s string) string {
	if s == "" {
		return s
	}

	// Order: tmp first (absolute), then home prefix, then .claude data paths.
	s = tmpClaudeRe.ReplaceAllString(s, "<tmp>")
	s = homePathRe.ReplaceAllString(s, "~")
	s = claudeDataRe.ReplaceAllString(s, "<claude-path>")

	return s
}

// DecodeCommonHTMLEntities decodes HTML entities via html.UnescapeString
// (same stdlib approach as pkg/urlutil). Runs a few passes so double-encoded
// forms like &amp;gt; become ">".
func DecodeCommonHTMLEntities(s string) string {
	if s == "" {
		return s
	}

	// Cap passes: real content is 1–2 encodings deep; avoid pathological loops.
	for range 3 {
		next := html.UnescapeString(s)
		if next == s {
			break
		}
		s = next
	}

	return s
}
