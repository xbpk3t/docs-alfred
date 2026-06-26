package checkutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Result.HasErrors tests ---

func TestResultHasErrorsTrue(t *testing.T) {
	r := &Result{
		Issues: []Issue{
			{File: "test.go", Severity: SeverityError, Message: "bad"},
		},
	}
	assert.True(t, r.HasErrors())
}

func TestResultHasErrorsFalse(t *testing.T) {
	r := &Result{
		Issues: []Issue{
			{File: "test.go", Severity: SeverityWarn, Message: "warning"},
		},
	}
	assert.False(t, r.HasErrors())
}

func TestResultHasErrorsEmpty(t *testing.T) {
	r := &Result{}
	assert.False(t, r.HasErrors())
}

func TestResultHasErrorsMixed(t *testing.T) {
	r := &Result{
		Issues: []Issue{
			{File: "a.go", Severity: SeverityWarn, Message: "warn"},
			{File: "b.go", Severity: SeverityError, Message: "err"},
		},
	}
	assert.True(t, r.HasErrors())
}

// --- Result.ReportResult tests ---

func TestReportResultNoIssues(t *testing.T) {
	r := &Result{}
	result := r.ReportResult("check")
	assert.Contains(t, result, "passed")
	assert.Contains(t, result, "check")
}

func TestReportResultWithErrors(t *testing.T) {
	r := &Result{
		Issues: []Issue{
			{File: "bad.go", Severity: SeverityError, Message: "syntax error"},
		},
	}
	result := r.ReportResult("lint")
	assert.Contains(t, result, "ERROR")
	assert.Contains(t, result, "bad.go")
	assert.Contains(t, result, "syntax error")
	assert.Contains(t, result, "failed")
}

func TestReportResultWithWarnings(t *testing.T) {
	r := &Result{
		Issues: []Issue{
			{File: "warn.go", Severity: SeverityWarn, Message: "style issue"},
		},
	}
	result := r.ReportResult("style")
	assert.Contains(t, result, "WARN")
	assert.Contains(t, result, "warn.go")
	assert.Contains(t, result, "passed (with warnings)")
}

func TestReportResultWithLine(t *testing.T) {
	r := &Result{
		Issues: []Issue{
			{File: "bad.go", Severity: SeverityError, Message: "err", Line: 42},
		},
	}
	result := r.ReportResult("check")
	assert.Contains(t, result, "bad.go:42")
}

func TestReportResultMultipleIssues(t *testing.T) {
	r := &Result{
		Issues: []Issue{
			{File: "a.go", Severity: SeverityWarn, Message: "w1"},
			{File: "b.go", Severity: SeverityError, Message: "e1"},
			{File: "c.go", Severity: SeverityError, Message: "e2"},
		},
	}
	result := r.ReportResult("check")
	assert.Contains(t, result, "3 issues")
	assert.Contains(t, result, "WARN")
	assert.Contains(t, result, "ERROR")
}

// --- HasErrors function tests ---

func TestHasErrorsTrue(t *testing.T) {
	issues := []Issue{
		{Severity: SeverityWarn},
		{Severity: SeverityError},
	}
	assert.True(t, HasErrors(issues))
}

func TestHasErrorsFalse(t *testing.T) {
	issues := []Issue{
		{Severity: SeverityWarn},
	}
	assert.False(t, HasErrors(issues))
}

func TestHasErrorsEmpty(t *testing.T) {
	assert.False(t, HasErrors(nil))
}

// --- ReportIssues tests ---

func TestReportIssuesNoErrors(t *testing.T) {
	issues := []Issue{
		{File: "a.go", Severity: SeverityWarn, Message: "warn"},
	}
	report, ok := ReportIssues(issues, "check")
	assert.True(t, ok)
	assert.Contains(t, report, "WARN")
	assert.Contains(t, report, "passed (with warnings)")
}

func TestReportIssuesWithErrors(t *testing.T) {
	issues := []Issue{
		{File: "a.go", Severity: SeverityError, Message: "err"},
	}
	report, ok := ReportIssues(issues, "check")
	assert.False(t, ok)
	assert.Contains(t, report, "ERROR")
	assert.Contains(t, report, "failed")
}

func TestReportIssuesEmpty(t *testing.T) {
	report, ok := ReportIssues(nil, "check")
	assert.True(t, ok)
	assert.Contains(t, report, "passed")
}

// --- DateFullPattern tests ---

func TestDateFullPatternValid(t *testing.T) {
	assert.True(t, DateFullPattern.MatchString("2024-01-15"))
	assert.True(t, DateFullPattern.MatchString("2023-12-31"))
}

func TestDateFullPatternInvalid(t *testing.T) {
	assert.False(t, DateFullPattern.MatchString("2024-1-5"))
	assert.False(t, DateFullPattern.MatchString("2024/01/15"))
	assert.False(t, DateFullPattern.MatchString("not-a-date"))
	assert.False(t, DateFullPattern.MatchString(""))
}

// --- Severity constants ---

func TestSeverityConstants(t *testing.T) {
	require.Equal(t, "error", SeverityError)
	require.Equal(t, "warn", SeverityWarn)
}

// --- Issue struct ---

func TestIssueFields(t *testing.T) {
	issue := Issue{
		File:     "test.go",
		Severity: SeverityError,
		Message:  "test message",
		Line:     10,
	}
	assert.Equal(t, "test.go", issue.File)
	assert.Equal(t, "error", issue.Severity)
	assert.Equal(t, "test message", issue.Message)
	assert.Equal(t, 10, issue.Line)
}
