package cmd

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	wikiuc "github.com/xbpk3t/docs-alfred/internal/docs/ingest"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

// --- writeWikiResult text ---

func TestWriteWikiResult_TextPassed(t *testing.T) {
	stdout := captureStdout(t)

	err := writeWikiResult(&wikiuc.Result{
		Name: "wiki add",
		URLResults: []wikiuc.URLResult{
			{URL: "https://example.com/a", Status: "summary_written", OutputPath: "wiki/topic/a.md", TopicPath: "topic/a", Handled: true},
		},
	}, "text")
	require.NoError(t, err)

	out := stdout()
	assert.Contains(t, out, "wiki add passed")
	assert.Contains(t, out, "https://example.com/a")
}

func TestWriteWikiResult_TextFailed(t *testing.T) {
	stdout := captureStdout(t)

	err := writeWikiResult(&wikiuc.Result{
		Name: "wiki add",
		URLResults: []wikiuc.URLResult{
			{URL: "https://example.com/b", Status: "unhandled_error", Error: "timeout"},
		},
	}, "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "wiki add failed")
	_ = stdout()
}

func TestWriteWikiResult_TextMixedSuccessAndFailure(t *testing.T) {
	stdout := captureStdout(t)

	err := writeWikiResult(&wikiuc.Result{
		Name: "wiki add",
		URLResults: []wikiuc.URLResult{
			{URL: "https://example.com/good", Status: "summary_written", OutputPath: "wiki/g.md", TopicPath: "g", Handled: true},
			{URL: "https://example.com/bad", Status: "unhandled_error", Error: "timeout", Handled: false},
		},
	}, "text")
	require.Error(t, err)

	out := stdout()
	assert.Contains(t, out, "https://example.com/good")
	assert.Contains(t, out, "https://example.com/bad")
	assert.Contains(t, out, "error=timeout")
}

func TestWriteWikiResult_TextFailureType(t *testing.T) {
	stdout := captureStdout(t)

	err := writeWikiResult(&wikiuc.Result{
		Name: "wiki add",
		URLResults: []wikiuc.URLResult{
			{URL: "https://example.com/x", Status: "failure_written", OutputPath: "wiki/x.md", FailureType: "rate_limit", Handled: true},
		},
	}, "text")
	require.NoError(t, err)

	out := stdout()
	assert.Contains(t, out, "failure=rate_limit")
}

func TestWriteWikiResult_TextDryRun(t *testing.T) {
	stdout := captureStdout(t)

	err := writeWikiResult(&wikiuc.Result{
		Name:     "wiki digest",
		DryRun:   true,
		URLResults: []wikiuc.URLResult{
			{URL: "https://example.com/a", Status: "dry_run_summary", Handled: true},
		},
	}, "text")
	require.NoError(t, err)

	out := stdout()
	assert.Contains(t, out, "dryRun=true")
}

func TestWriteWikiResult_TextFlushed(t *testing.T) {
	stdout := captureStdout(t)

	err := writeWikiResult(&wikiuc.Result{
		Name:    "wiki digest",
		Flushed: 3,
		URLResults: []wikiuc.URLResult{
			{URL: "https://example.com/a", Status: "summary_written", OutputPath: "wiki/a.md", Handled: true},
		},
	}, "text")
	require.NoError(t, err)

	out := stdout()
	assert.Contains(t, out, "flushed=3")
}

func TestWriteWikiResult_TextWithActions(t *testing.T) {
	stdout := captureStdout(t)

	err := writeWikiResult(&wikiuc.Result{
		Name:    "wiki digest",
		Flushed: 2,
		URLResults: []wikiuc.URLResult{
			{URL: "https://example.com/a", Status: "summary_written", OutputPath: "wiki/a.md", Handled: true},
		},
	}, "text")
	require.NoError(t, err)

	out := stdout()
	assert.Contains(t, out, "[actions]")
	assert.Contains(t, out, "flushed 2 inbox line(s)")
}

// --- writeWikiResult JSON ---

func TestWriteWikiResult_JSONPassed(t *testing.T) {
	stdout := captureStdout(t)

	err := writeWikiResult(&wikiuc.Result{
		Name: "wiki add",
		URLResults: []wikiuc.URLResult{
			{URL: "https://example.com/a", Status: "summary_written", OutputPath: "wiki/a.md", Handled: true},
		},
	}, "json")
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout()), &result))
	assert.Equal(t, "wiki add", result["name"])
	assert.Equal(t, true, result["ok"])
}

func TestWriteWikiResult_JSONFailed(t *testing.T) {
	stdout := captureStdout(t)

	err := writeWikiResult(&wikiuc.Result{
		Name: "wiki add",
		URLResults: []wikiuc.URLResult{
			{URL: "https://example.com/b", Status: "unhandled_error", Error: "timeout"},
		},
	}, "json")
	require.Error(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout()), &result))
	assert.Equal(t, false, result["ok"])
}

func TestWriteWikiResult_JSONMixed(t *testing.T) {
	stdout := captureStdout(t)

	err := writeWikiResult(&wikiuc.Result{
		Name: "wiki add",
		URLResults: []wikiuc.URLResult{
			{URL: "https://example.com/good", Status: "summary_written", OutputPath: "wiki/g.md", Handled: true},
			{URL: "https://example.com/bad", Status: "unhandled_error", Error: "timeout"},
		},
	}, "json")
	require.Error(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout()), &result))
	assert.Equal(t, false, result["ok"])

	urls := result["results"].([]any)
	assert.Len(t, urls, 2)
}

func TestWriteWikiResult_TextEmptyURLs(t *testing.T) {
	stdout := captureStdout(t)

	err := writeWikiResult(&wikiuc.Result{
		Name:       "wiki digest",
		URLResults: nil,
	}, "text")
	require.NoError(t, err)

	out := stdout()
	assert.Contains(t, out, "wiki digest passed")
	assert.Contains(t, out, "processed=0")
}

func TestWriteWikiResult_TextWouldFlushDryRun(t *testing.T) {
	stdout := captureStdout(t)

	err := writeWikiResult(&wikiuc.Result{
		Name:       "wiki digest",
		DryRun:     true,
		WouldFlush: 5,
		URLResults: []wikiuc.URLResult{
			{URL: "https://example.com/a", Status: "dry_run_summary", Handled: true},
		},
	}, "text")
	require.NoError(t, err)

	out := stdout()
	assert.Contains(t, out, "wouldFlush=5")
	assert.Contains(t, out, "dry-run: skipped inbox flush for 5 line(s)")
}

func TestWriteWikiResult_JSONDryRun(t *testing.T) {
	stdout := captureStdout(t)

	err := writeWikiResult(&wikiuc.Result{
		Name:       "wiki digest",
		DryRun:     true,
		WouldFlush: 3,
		URLResults: nil,
	}, "json")
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout()), &result))
	summary := result["summary"].(map[string]any)
	assert.Equal(t, float64(0), summary["processed"])
	assert.Equal(t, float64(3), summary["wouldFlush"])
	assert.Equal(t, true, summary["dryRun"])
}

func TestWriteWikiResult_JSONEmptyURLs(t *testing.T) {
	stdout := captureStdout(t)

	err := writeWikiResult(&wikiuc.Result{
		Name:       "wiki digest",
		URLResults: nil,
	}, "json")
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout()), &result))
	assert.Equal(t, true, result["ok"])

	urls := result["results"]
	assert.Nil(t, urls)
}

// --- writeWikiResult invalid format ---

func TestWriteWikiResult_InvalidFormat(t *testing.T) {
	err := writeWikiResult(&wikiuc.Result{
		Name: "wiki add",
	}, "xml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported output format")
}

func TestWriteWikiResult_EmptyFormat(t *testing.T) {
	stdout := captureStdout(t)

	err := writeWikiResult(&wikiuc.Result{
		Name: "wiki add",
		URLResults: []wikiuc.URLResult{
			{URL: "https://example.com/a", Status: "summary_written", OutputPath: "wiki/a.md", Handled: true},
		},
	}, "")
	require.NoError(t, err, "empty format defaults to text")

	out := stdout()
	assert.Contains(t, out, "wiki add passed")
}

// --- writeWikiAuditResult text ---

func TestWriteWikiAuditResult_TextPassed(t *testing.T) {
	stdout := captureStdout(t)

	err := writeWikiAuditResult(&wikiuc.AuditResult{
		Name: "wiki audit",
	}, "text")
	require.NoError(t, err)

	out := stdout()
	assert.Contains(t, out, "wiki audit passed")
}

func TestWriteWikiAuditResult_TextWithWarnings(t *testing.T) {
	stdout := captureStdout(t)

	err := writeWikiAuditResult(&wikiuc.AuditResult{
		Name: "wiki audit",
		Issues: []checkutil.Issue{
			{Severity: checkutil.SeverityWarn, Message: "minor issue"},
		},
	}, "text")
	require.NoError(t, err, "warnings only should not fail")

	out := stdout()
	assert.Contains(t, out, "warnings=1")
}

func TestWriteWikiAuditResult_TextWithErrorsAndWarnings(t *testing.T) {
	stdout := captureStdout(t)

	err := writeWikiAuditResult(&wikiuc.AuditResult{
		Name: "wiki audit",
		Issues: []checkutil.Issue{
			{Severity: checkutil.SeverityError, Message: "bad"},
			{Severity: checkutil.SeverityWarn, Message: "warn"},
		},
	}, "text")
	require.Error(t, err)

	out := stdout()
	assert.Contains(t, out, "wiki audit failed")
	assert.Contains(t, out, "issues=2")
	assert.Contains(t, out, "errors=1")
	assert.Contains(t, out, "warnings=1")
}

// --- writeWikiAuditResult JSON ---

func TestWriteWikiAuditResult_JSONPassed(t *testing.T) {
	stdout := captureStdout(t)

	err := writeWikiAuditResult(&wikiuc.AuditResult{
		Name: "wiki audit",
	}, "json")
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout()), &result))
	assert.Equal(t, "wiki audit", result["name"])
	assert.Equal(t, true, result["ok"])
}

func TestWriteWikiAuditResult_JSONFailed(t *testing.T) {
	stdout := captureStdout(t)

	err := writeWikiAuditResult(&wikiuc.AuditResult{
		Name: "wiki audit",
		Issues: []checkutil.Issue{
			{Severity: checkutil.SeverityError, Message: "critical"},
		},
	}, "json")
	require.Error(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout()), &result))
	assert.Equal(t, false, result["ok"])
}

func TestWriteWikiAuditResult_TextEmptyIssues(t *testing.T) {
	stdout := captureStdout(t)

	err := writeWikiAuditResult(&wikiuc.AuditResult{
		Name:   "wiki audit",
		Issues: []checkutil.Issue{},
	}, "text")
	require.NoError(t, err)

	out := stdout()
	assert.Contains(t, out, "wiki audit passed")
	assert.Contains(t, out, "issues=0")
	assert.Contains(t, out, "errors=0")
	assert.Contains(t, out, "warnings=0")
}

func TestWriteWikiAuditResult_EmptyFormat(t *testing.T) {
	stdout := captureStdout(t)

	err := writeWikiAuditResult(&wikiuc.AuditResult{
		Name: "wiki audit",
	}, "")
	require.NoError(t, err, "empty format defaults to text")

	out := stdout()
	assert.Contains(t, out, "wiki audit passed")
}

func TestWriteWikiAuditResult_JSONWithWarningsOnly(t *testing.T) {
	stdout := captureStdout(t)

	err := writeWikiAuditResult(&wikiuc.AuditResult{
		Name: "wiki audit",
		Issues: []checkutil.Issue{
			{Severity: checkutil.SeverityWarn, Message: "minor warning"},
		},
	}, "json")
	require.NoError(t, err, "warnings only should not fail")

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout()), &result))
	assert.Equal(t, true, result["ok"])
}

func TestWriteWikiAuditResult_JSONWithWarningsSummary(t *testing.T) {
	stdout := captureStdout(t)

	err := writeWikiAuditResult(&wikiuc.AuditResult{
		Name: "wiki audit",
		Issues: []checkutil.Issue{
			{Severity: checkutil.SeverityWarn, Message: "w1"},
			{Severity: checkutil.SeverityWarn, Message: "w2"},
		},
	}, "json")
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout()), &result))
	summary := result["summary"].(map[string]any)
	assert.Equal(t, float64(2), summary["issues"])
	assert.Equal(t, float64(0), summary["errors"])
	assert.Equal(t, float64(2), summary["warnings"])
}

func TestWriteWikiAuditResult_JSONSummary(t *testing.T) {
	stdout := captureStdout(t)

	err := writeWikiAuditResult(&wikiuc.AuditResult{
		Name: "wiki audit",
		Issues: []checkutil.Issue{
			{Severity: checkutil.SeverityError, Message: "e1"},
			{Severity: checkutil.SeverityError, Message: "e2"},
			{Severity: checkutil.SeverityWarn, Message: "w1"},
		},
	}, "json")
	require.Error(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout()), &result))
	summary := result["summary"].(map[string]any)
	assert.Equal(t, float64(3), summary["issues"])
	assert.Equal(t, float64(2), summary["errors"])
	assert.Equal(t, float64(1), summary["warnings"])
}

// --- writeWikiAuditResult invalid format ---

func TestWriteWikiAuditResult_InvalidFormat(t *testing.T) {
	err := writeWikiAuditResult(&wikiuc.AuditResult{
		Name: "wiki audit",
	}, "yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported output format")
}

// --- wiki check ---

func TestWikiCheckCommandDefaults(t *testing.T) {
	wikiCheck, _, err := newRootCmd().Find([]string{wikiCommandName, wikiCheckCommandName})
	require.NoError(t, err)

	ghRoot, _ := wikiCheck.Flags().GetString("gh-root")
	assert.Equal(t, "data/gh", ghRoot)

	wikiRoot, _ := wikiCheck.Flags().GetString("wiki-root")
	assert.Equal(t, "wiki", wikiRoot)
}

func TestWikiCheck_NonExistentGhRoot(t *testing.T) {
	// RunE directly calls workspaceuc.RunWikiCheck — test error propagation
	wikiCheck, _, err := newRootCmd().Find([]string{wikiCommandName, wikiCheckCommandName})
	require.NoError(t, err)

	// Set flags to non-existent paths
	require.NoError(t, wikiCheck.Flags().Set("gh-root", "/tmp/nonexistent-"+t.Name()))
	require.NoError(t, wikiCheck.Flags().Set("wiki-root", t.TempDir()))

	err = wikiCheck.RunE(wikiCheck, nil)
	require.Error(t, err)
}

func TestWikiCheck_PassedWithEmptyDirs(t *testing.T) {
	stdout := captureStdout(t)

	wikiCheck, _, err := newRootCmd().Find([]string{wikiCommandName, wikiCheckCommandName})
	require.NoError(t, err)

	require.NoError(t, wikiCheck.Flags().Set("gh-root", t.TempDir()))
	require.NoError(t, wikiCheck.Flags().Set("wiki-root", t.TempDir()))

	err = wikiCheck.RunE(wikiCheck, nil)
	require.NoError(t, err)

	out := stdout()
	assert.Contains(t, out, "wiki check passed")
	assert.Contains(t, out, "summary:")
}

// --- wiki root command ---

func TestWikiRootCommand_ShowsHelpWithArgs(t *testing.T) {
	// Verify error message when args are passed directly to wiki
	wikiCmd, _, err := newRootCmd().Find([]string{wikiCommandName})
	require.NoError(t, err)

	err = wikiCmd.RunE(wikiCmd, []string{"some-url"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "use `docs-cli wiki add")

	// No args → help (no error, RunE returns cmd.Help())
	err = wikiCmd.RunE(wikiCmd, nil)
	require.NoError(t, err)
}

// --- digest-local command ---

func TestDigestLocalCommand_RequiresFromDir(t *testing.T) {
	digestLocal, _, err := newRootCmd().Find([]string{wikiCommandName, "digest-local"})
	require.NoError(t, err)

	err = digestLocal.RunE(digestLocal, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--from-dir is required")
}
