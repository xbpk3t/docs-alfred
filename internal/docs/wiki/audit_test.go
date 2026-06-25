package wiki

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

func TestAuditWikiReportsPollutedSummaryAndMalformedURL(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "topic", "path")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "summary.md"), []byte(`### Bad

- URL: https://t.co/a](https://x.com/a)
- Type: deep_dive

This page requires JavaScript.
`), 0o600))

	issues, err := AuditWiki(root)

	require.NoError(t, err)
	require.NotEmpty(t, issues)
	assert.True(t, checkutil.HasErrors(issues))
	assertIssueContains(t, issues, "malformed URL")
	assertIssueContains(t, issues, "low-quality extraction")
}

func TestAuditWikiReportsMalformedFailedAndInboxURLs(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "fetch-failed.md"), []byte("https://t.co/a](https://x.com/a)"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "inbox.md"), []byte("https://t.co/b](https://x.com/b)"), 0o600))

	issues, err := AuditWiki(root)

	require.NoError(t, err)
	require.Len(t, issues, 2)
}

func TestAuditWikiPathsIgnoresUnscopedHistoricalPollution(t *testing.T) {
	root := t.TempDir()
	pollutedDir := filepath.Join(root, "old", "polluted")
	cleanDir := filepath.Join(root, "new", "clean")
	require.NoError(t, os.MkdirAll(pollutedDir, 0o700))
	require.NoError(t, os.MkdirAll(cleanDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(pollutedDir, "summary.md"), []byte(`### Bad

- URL: https://t.co/a](https://x.com/a)
- Type: deep_dive

This page requires JavaScript.
`), 0o600))
	cleanPath := filepath.Join(cleanDir, "summary.md")
	require.NoError(t, os.WriteFile(cleanPath, []byte(`### Good

- URL: https://example.com/a
- Type: deep_dive

This is a long enough clean summary entry that should not be flagged by the scoped wiki audit.
`), 0o600))

	issues, err := AuditWikiPaths(root, []string{cleanPath})

	require.NoError(t, err)
	require.Empty(t, issues)
}

func assertIssueContains(t *testing.T, issues []checkutil.Issue, want string) {
	t.Helper()
	for _, issue := range issues {
		if strings.Contains(issue.Message, want) {
			return
		}
	}

	assert.Failf(t, "missing issue", "want %q in %#v", want, issues)
}

// --- auditWikiDir ---

func TestAuditWikiDirEmptyDir(t *testing.T) {
	root := t.TempDir()
	emptyDir := filepath.Join(root, "empty")
	require.NoError(t, os.MkdirAll(emptyDir, 0o700))

	issues, err := auditWikiDir(root, emptyDir, make(map[string]bool))

	require.NoError(t, err)
	assert.Empty(t, issues)
}

func TestAuditWikiDirWithValidMarkdownFiles(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "topic", "path")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "summary.md"), []byte(`### Good

- URL: https://example.com/a
- Type: deep_dive

This is a long enough clean summary entry that should not be flagged by the audit.
`), 0o600))

	issues, err := auditWikiDir(root, root, make(map[string]bool))

	require.NoError(t, err)
	assert.Empty(t, issues)
}

func TestAuditWikiDirSkipsDuplicateFiles(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "topic")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.md"), []byte("content"), 0o600))

	seen := map[string]bool{filepath.Join(dir, "test.md"): true}
	issues, err := auditWikiDir(root, root, seen)

	require.NoError(t, err)
	assert.Empty(t, issues)
}

// --- auditIssue ---

func TestAuditIssue(t *testing.T) {
	root := "/wiki"
	issue := auditIssue(root, "/wiki/topic/file.md", 5, checkutil.SeverityError, "test message")

	assert.Equal(t, "topic/file.md", issue.File)
	assert.Equal(t, 5, issue.Line)
	assert.Equal(t, checkutil.SeverityError, issue.Severity)
	assert.Equal(t, "test message", issue.Message)
}

// --- slashRel ---

func TestSlashRel(t *testing.T) {
	assert.Equal(t, "topic/file.md", slashRel("/wiki", "/wiki/topic/file.md"))
	assert.Equal(t, "file.md", slashRel("/wiki", "/wiki/file.md"))
}

func TestSlashRelFallsBackToPath(t *testing.T) {
	// When Rel fails (different drives on Windows, etc.), falls back to ToSlash(path).
	result := slashRel("/root", "relative/path")
	assert.NotEmpty(t, result)
}

// --- absEvalSymlink ---

func TestAbsEvalSymlink(t *testing.T) {
	dir := t.TempDir()
	result, err := absEvalSymlink(dir)
	require.NoError(t, err)
	assert.True(t, filepath.IsAbs(result))
}

func TestAbsEvalSymlinkRelativePath(t *testing.T) {
	result, err := absEvalSymlink("relative/path")
	require.NoError(t, err)
	assert.True(t, filepath.IsAbs(result))
}

// --- evalSymlinks ---

func TestEvalSymlinksValidPath(t *testing.T) {
	dir := t.TempDir()
	resolved, ok := evalSymlinks(dir)
	assert.True(t, ok)
	assert.NotEmpty(t, resolved)
}

func TestEvalSymlinksInvalidPath(t *testing.T) {
	resolved, ok := evalSymlinks("/nonexistent/path/that/does/not/exist")
	assert.False(t, ok)
	assert.Empty(t, resolved)
}

// --- pathWithinRoot ---

func TestPathWithinRoot(t *testing.T) {
	tests := []struct {
		name string
		root string
		path string
		want bool
	}{
		{"within root", "/wiki", "/wiki/topic/file.md", true},
		{"same as root", "/wiki", "/wiki", true},
		{"outside root", "/wiki", "/other/file.md", false},
		{"parent of root", "/wiki/sub", "/wiki", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := pathWithinRoot(tt.root, tt.path)
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

// --- resolveAuditPath ---

func TestResolveAuditPathValidFile(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "topic")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.md"), []byte("content"), 0o600))

	path, err := resolveAuditPath(root, filepath.Join(dir, "test.md"))
	require.NoError(t, err)
	assert.True(t, filepath.IsAbs(path))
}

func TestResolveAuditPathOutsideRoot(t *testing.T) {
	root := t.TempDir()
	other := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(other, "test.md"), []byte("content"), 0o600))

	_, err := resolveAuditPath(root, filepath.Join(other, "test.md"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside wiki root")
}

func TestResolveAuditPathEmptyPath(t *testing.T) {
	root := t.TempDir()
	_, err := resolveAuditPath(root, "  ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

// --- auditPathCandidate ---

func TestAuditPathCandidateValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	require.NoError(t, os.WriteFile(path, []byte("content"), 0o600))

	result, err := auditPathCandidate(dir, path)
	require.NoError(t, err)
	assert.Equal(t, path, result)
}

func TestAuditPathCandidateRelativePath(t *testing.T) {
	root := t.TempDir()
	result, err := auditPathCandidate(root, "relative/path.md")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root, "relative/path.md"), result)
}

func TestAuditPathCandidateEmptyPath(t *testing.T) {
	_, err := auditPathCandidate("/wiki", "  ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

// --- auditCanonicalHeadings ---

func TestAuditCanonicalHeadingsValid(t *testing.T) {
	lines := []string{"#### overview", "#### detail", "#### keyPoints", "#### keyQuotes", "#### actionableAdvice", "#### worthNoting", "#### criticalThinking"}
	issues := auditCanonicalHeadings("test.md", lines)
	assert.Empty(t, issues)
}

func TestAuditCanonicalHeadingsInvalid(t *testing.T) {
	lines := []string{"#### Invalid Heading"}
	issues := auditCanonicalHeadings("test.md", lines)
	require.Len(t, issues, 1)
	assert.Contains(t, issues[0].Message, "non-canonical section heading")
}

func TestAuditCanonicalHeadingsNonHeadingLines(t *testing.T) {
	lines := []string{"# Title", "Some text", "## Section"}
	issues := auditCanonicalHeadings("test.md", lines)
	assert.Empty(t, issues)
}

// --- auditCodeblockFields ---

func TestAuditCodeblockFieldsValid(t *testing.T) {
	lines := []string{"```", "URL: https://example.com", "Type: text", "quality: high", "```"}
	issues := auditCodeblockFields("test.md", lines)
	assert.Empty(t, issues)
}

func TestAuditCodeblockFieldsInvalid(t *testing.T) {
	lines := []string{"```", "unknownField: value", "```"}
	issues := auditCodeblockFields("test.md", lines)
	require.Len(t, issues, 1)
	assert.Contains(t, issues[0].Message, "unknown codeblock field")
}

func TestAuditCodeblockFieldsOutsideCodeblock(t *testing.T) {
	lines := []string{"unknownField: value"}
	issues := auditCodeblockFields("test.md", lines)
	assert.Empty(t, issues)
}

func TestAuditCodeblockFieldsNoColon(t *testing.T) {
	lines := []string{"```", "no colon line", "```"}
	issues := auditCodeblockFields("test.md", lines)
	assert.Empty(t, issues)
}

func TestAuditCodeblockFieldsEmptyFieldName(t *testing.T) {
	lines := []string{"```", ": value", "```"}
	issues := auditCodeblockFields("test.md", lines)
	assert.Empty(t, issues)
}

// --- AuditWikiPaths edge cases ---

func TestAuditWikiPathsEmptyPaths(t *testing.T) {
	issues, err := AuditWikiPaths("/wiki", []string{})
	require.NoError(t, err)
	assert.Nil(t, issues)
}

func TestAuditWikiPathsDirPath(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "topic")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.md"), []byte("content"), 0o600))

	issues, err := AuditWikiPaths(root, []string{dir})
	require.NoError(t, err)
	assert.Empty(t, issues)
}

func TestAuditWikiPathsNonMDFile(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "test.txt"), []byte("content"), 0o600))

	issues, err := AuditWikiPaths(root, []string{filepath.Join(root, "test.txt")})
	require.NoError(t, err)
	assert.Empty(t, issues)
}

// --- lineHasRawMalformedURL edge cases ---

func TestLineHasRawMalformedURLMarkdownLink(t *testing.T) {
	// Standard markdown link [text](url) should NOT be flagged
	assert.False(t, lineHasRawMalformedURL("[click here](https://example.com)"))
}

func TestLineHasRawMalformedURLSummaryHeading(t *testing.T) {
	// Summary heading with markdown link should NOT be flagged
	assert.False(t, lineHasRawMalformedURL("## [Title](https://example.com) - URL: https://example.com"))
}

// --- isFailureFile ---

func TestIsFailureFile(t *testing.T) {
	assert.True(t, isFailureFile("/wiki", "/wiki/fetch-failed.md"))
	assert.True(t, isFailureFile("/wiki", "/wiki/digest-fetch-error.md"))
	assert.False(t, isFailureFile("/wiki", "/wiki/summary.md"))
	assert.False(t, isFailureFile("/wiki", "/wiki/inbox.md"))
}

// --- AuditWiki with walk error ---

func TestAuditWikiWithWalkError(t *testing.T) {
	root := t.TempDir()
	// Create a directory with a file, then remove the directory to cause walk error
	dir := filepath.Join(root, "topic")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.md"), []byte("content"), 0o600))

	issues, err := AuditWiki(root)
	require.NoError(t, err)
	assert.Empty(t, issues)
}

// --- auditWikiDir with non-md files ---

func TestAuditWikiDirSkipsNonMDFiles(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "topic")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.txt"), []byte("content"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.json"), []byte("{}"), 0o600))

	issues, err := auditWikiDir(root, root, make(map[string]bool))
	require.NoError(t, err)
	assert.Empty(t, issues)
}

// --- auditWikiDir with summary.md containing issues ---

func TestAuditWikiDirWithPollutedSummary(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "topic", "path")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "summary.md"), []byte(`### Bad

- URL: https://t.co/a](https://x.com/a)
- Type: deep_dive

This page requires JavaScript.
`), 0o600))

	issues, err := auditWikiDir(root, root, make(map[string]bool))
	require.NoError(t, err)
	assert.NotEmpty(t, issues)
}

// --- AuditWikiPaths with absolute path ---

func TestAuditWikiPathsAbsolutePath(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "test.md"), []byte("content"), 0o600))

	issues, err := AuditWikiPaths(root, []string{filepath.Join(root, "test.md")})
	require.NoError(t, err)
	assert.Empty(t, issues)
}

// --- auditWikiPath with directory ---

func TestAuditWikiPathWithDirectory(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "topic")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.md"), []byte("content"), 0o600))

	issues, err := auditWikiPath(root, dir, make(map[string]bool))
	require.NoError(t, err)
	assert.Empty(t, issues)
}

// --- resolveAuditPath with relative path ---

func TestResolveAuditPathRelativePath(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "topic")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.md"), []byte("content"), 0o600))

	path, err := resolveAuditPath(root, "topic/test.md")
	require.NoError(t, err)
	assert.True(t, filepath.IsAbs(path))
}

// --- absEvalSymlink with existing path ---

func TestAbsEvalSymlinkExistingPath(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.md"), []byte("content"), 0o600))

	result, err := absEvalSymlink(filepath.Join(dir, "test.md"))
	require.NoError(t, err)
	assert.True(t, filepath.IsAbs(result))
}

// --- pathWithinRoot edge cases ---

func TestPathWithinRootNestedPath(t *testing.T) {
	result, err := pathWithinRoot("/wiki", "/wiki/a/b/c/d/file.md")
	require.NoError(t, err)
	assert.True(t, result)
}

func TestPathWithinRootSimilarPrefix(t *testing.T) {
	result, err := pathWithinRoot("/wiki", "/wiki2/file.md")
	require.NoError(t, err)
	assert.False(t, result)
}

// --- auditCanonicalHeadings with mixed content ---

func TestAuditCanonicalHeadingsMixedContent(t *testing.T) {
	lines := []string{
		"# Title",
		"#### overview",
		"Some text",
		"#### Invalid",
		"#### keyPoints",
	}
	issues := auditCanonicalHeadings("test.md", lines)
	require.Len(t, issues, 1)
	assert.Contains(t, issues[0].Message, "Invalid")
}

// --- auditCodeblockFields with nested codeblocks ---

func TestAuditCodeblockFieldsNestedCodeblocks(t *testing.T) {
	lines := []string{
		"```",
		"URL: https://example.com",
		"```",
		"```",
		"unknownField: value",
		"```",
	}
	issues := auditCodeblockFields("test.md", lines)
	require.Len(t, issues, 1)
	assert.Contains(t, issues[0].Message, "unknown codeblock field")
}

// --- lineHasRawMalformedURL edge cases ---

func TestLineHasRawMalformedURLNormalLine(t *testing.T) {
	assert.False(t, lineHasRawMalformedURL("Just a normal line of text"))
}

func TestLineHasRawMalformedURLLinkInMiddle(t *testing.T) {
	// URL in middle of line that's not a malformed capture
	assert.False(t, lineHasRawMalformedURL("Check https://example.com for more"))
}
