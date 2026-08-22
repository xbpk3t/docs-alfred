// Package checkutil provides shared types for check/result/report patterns.
package checkutil

import (
	"fmt"
	"path"
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
	File     string `json:"file"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Line     int    `json:"line,omitempty"`
}

// Result is the common check result type.
type Result struct {
	Issues []Issue
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

// ReportIssues returns the formatted report string and whether there are no errors.
func ReportIssues(issues []Issue, command string) (string, bool) {
	r := &Result{Issues: issues}

	return r.ReportResult(command), !r.HasErrors()
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

// HasSegmentDir reports whether any parent directory segment of rel equals
// name. rel is a slash-normalized path relative to the wiki root; files placed
// directly at the root (no parent dir) return false.
func HasSegmentDir(rel, name string) bool {
	dir := path.Dir(rel)
	if dir == "." {
		return false
	}
	for _, seg := range strings.Split(dir, "/") {
		if seg == name {
			return true
		}
	}
	return false
}

// DateFullPattern matches YYYY-MM-DD dates.
var DateFullPattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
