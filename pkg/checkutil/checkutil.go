// Package checkutil provides shared types for check/result/report patterns.
package checkutil

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Standard severity levels.
const (
	SeverityError = "error"
	SeverityWarn  = "warn"
)

// Issue represents a single validation issue.
type Issue struct {
	File     string
	Severity string
	Message  string
	Line     int
}

// Result is the common check result type.
type Result struct {
	Issues []Issue
}

// AddIssue appends a validation issue.
func (r *Result) AddIssue(file, severity, message string) {
	r.Issues = append(r.Issues, Issue{
		File: file, Severity: severity, Message: message,
	})
}

// HasErrors returns true if any error-severity issues exist.
func (r *Result) HasErrors() bool {
	for _, issue := range r.Issues {
		if issue.Severity == SeverityError {
			return true
		}
	}

	return false
}

// Report prints the check result to stderr.
func (r *Result) Report(name string) {
	result := r.ReportResult(name)
	if result != "" {
		fmt.Fprint(os.Stderr, result)
	}
}

// ReportResult returns the formatted check result.
func (r *Result) ReportResult(name string) string {
	if len(r.Issues) == 0 {
		return fmt.Sprintf("✅ %s passed\n", name)
	}

	var hasError bool
	var output string
	var outputSb62 strings.Builder
	for _, issue := range r.Issues {
		prefix := "WARN"
		if issue.Severity == SeverityError {
			prefix = "ERROR"
			hasError = true
		}
		fmt.Fprintf(&outputSb62, "%s %s", prefix, issue.File)
		if issue.Line > 0 {
			fmt.Fprintf(&outputSb62, ":%d", issue.Line)
		}
		fmt.Fprintf(&outputSb62, ": %s\n", issue.Message)
	}
	output += outputSb62.String()

	if !hasError {
		output += fmt.Sprintf("✅ %s passed (with warnings)\n", name)
	} else {
		output += fmt.Sprintf("❌ %s failed (%d issues)\n", name, len(r.Issues))
	}

	return output
}

// ReportIssues prints issues and returns true if no errors.
func ReportIssues(issues []Issue, command string) bool {
	r := &Result{Issues: issues}
	r.Report(command)

	return !r.HasErrors()
}

// HasErrors is a convenience function for checking a slice of issues.
func HasErrors(issues []Issue) bool {
	for _, issue := range issues {
		if issue.Severity == SeverityError {
			return true
		}
	}

	return false
}

// DateFullPattern matches YYYY-MM-DD dates.
var DateFullPattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
