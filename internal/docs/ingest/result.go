package wikiingest

import (
	"fmt"

	wikitypes "github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

// AddInput contains inputs for wiki URL ingestion.
type AddInput struct {
	Config *Config
	deps   *dependencies
	URLs   []string
	DryRun bool
}

// DigestInput contains inputs for wiki digest processing.
type DigestInput struct {
	Config *Config
	deps   *dependencies
	DryRun bool
}

// AuditInput contains inputs for read-only wiki auditing.
type AuditInput struct {
	Config      *Config
	RunCmd      CommandRunner // optional; required if ChangedOnly is set
	Paths       []string
	ChangedOnly bool
}

// AuditResult is the structured outcome for wiki audit.
type AuditResult struct {
	Name     string            `json:"-"`
	WikiRoot string            `json:"wikiRoot"`
	Issues   []checkutil.Issue `json:"issues"`
}

// Summary returns count-oriented audit details.
func (r *AuditResult) Summary() map[string]any {
	var errorCount, warnings int
	for _, issue := range r.Issues {
		switch issue.Severity {
		case checkutil.SeverityError:
			errorCount++
		case checkutil.SeverityWarn:
			warnings++
		}
	}

	return map[string]any{
		"issues":   len(r.Issues),
		"errors":   errorCount,
		"warnings": warnings,
	}
}

// OK reports whether audit found no error-severity issues.
func (r *AuditResult) OK() bool {
	return !checkutil.HasErrors(r.Issues)
}

// Result is the structured outcome for wiki commands.
type Result struct {
	Name       string      `json:"-"`
	WikiRoot   string      `json:"wikiRoot"`
	URLResults []URLResult `json:"urls"`
	Flushed    int         `json:"flushed"`
	WouldFlush int         `json:"wouldFlush"`
	DryRun     bool        `json:"dryRun"`
}

// URLResult records the outcome for one URL.
type URLResult struct {
	URL         string                `json:"url"`
	Status      string                `json:"status"`
	OutputPath  string                `json:"outputPath,omitempty"`
	TopicPath   string                `json:"topicPath,omitempty"`
	WikiType    string                `json:"wikiType,omitempty"`
	ContentType string                `json:"contentType,omitempty"`
	FailureType wikitypes.FailureKind `json:"failureType,omitempty"`
	Error       string                `json:"error,omitempty"`
	LineIndex   int                   `json:"lineIndex,omitempty"`
	Handled     bool                  `json:"handled"`
}

// Summary returns count-oriented command details for structured output.
func (r *Result) Summary() map[string]any {
	var succeeded, handledFailures, unhandledFailures, written int
	for i := range r.URLResults {
		item := &r.URLResults[i]
		switch item.Status {
		case StatusSummaryWritten, StatusDryRunSummary:
			succeeded++
		case StatusFailureWritten, StatusDryRunFailure:
			handledFailures++
		case StatusUnhandledError:
			unhandledFailures++
		}
		if item.OutputPath != "" && (item.Status == StatusSummaryWritten || item.Status == StatusFailureWritten) {
			written++
		}
	}

	return map[string]any{
		"processed":         len(r.URLResults),
		"succeeded":         succeeded,
		"handledFailures":   handledFailures,
		"unhandledFailures": unhandledFailures,
		"written":           written,
		"flushed":           r.Flushed,
		"wouldFlush":        r.WouldFlush,
		"dryRun":            r.DryRun,
	}
}

// OK reports whether the workflow had no unhandled URL-level failures.
func (r *Result) OK() bool {
	for i := range r.URLResults {
		item := &r.URLResults[i]
		if item.Status == StatusUnhandledError {
			return false
		}
	}

	return true
}

// Actions returns command actions for human-readable output.
func (r *Result) Actions() []string {
	var actions []string
	if r.DryRun {
		actions = append(actions, "dry-run: skipped wiki writes")
		if r.WouldFlush > 0 {
			actions = append(actions, fmt.Sprintf("dry-run: skipped inbox flush for %d line(s)", r.WouldFlush))
		}

		return actions
	}
	if r.Flushed > 0 {
		actions = append(actions, fmt.Sprintf("flushed %d inbox line(s)", r.Flushed))
	}

	return actions
}
