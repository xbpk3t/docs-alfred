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
	require.NoError(t, os.MkdirAll(filepath.Join(root, "failed"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(root, "failed", "fetch-failed.md"), []byte("https://t.co/a](https://x.com/a)"), 0o600))
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
