package wikiingest

import (
	"fmt"
	"time"

	wikitypes "github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

// AddInput contains inputs for wiki URL ingestion.
type AddInput struct {
	Config *Config
	deps   *dependencies
	URLs   []string
	DryRun bool
	// Force re-digests a URL even if it is already in success history. Without
	// it, already digested URLs are skipped (status=skipped) rather than written
	// again; duplicates within a single call always collapse.
	Force bool
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
	Name        string      `json:"-"`
	WikiRoot    string      `json:"wikiRoot"`
	AbortReason string      `json:"abortReason,omitempty"`
	URLResults  []URLResult `json:"urls"`
	Flushed     int         `json:"flushed"`
	WouldFlush  int         `json:"wouldFlush"`
	DryRun      bool        `json:"dryRun"`
}

// URLResult records the outcome for one URL.
type URLResult struct {
	URL           string                  `json:"url"`
	Status        string                  `json:"status"`
	OutputPath    string                  `json:"outputPath,omitempty"`
	TopicPath     string                  `json:"topicPath,omitempty"`
	WikiType      string                  `json:"wikiType,omitempty"`
	ContentType   string                  `json:"contentType,omitempty"`
	FailureType   wikitypes.FailureKind   `json:"failureType,omitempty"`
	FailureReason wikitypes.FailureReason `json:"failureReason,omitempty"`
	Error         string                  `json:"error,omitempty"`
	LineIndex     int                     `json:"lineIndex,omitempty"`
	Retryable     bool                    `json:"retryable,omitempty"`
	Handled       bool                    `json:"handled"`
}

// Summary returns count-oriented command details for structured output.
func (r *Result) Summary() map[string]any {
	var succeeded, handledFailures, retryableFailures, permanentFailures, unhandledFailures, skipped, written int
	for i := range r.URLResults {
		item := &r.URLResults[i]
		switch item.Status {
		case StatusSummaryWritten, StatusDryRunSummary:
			succeeded++
		case StatusFailureWritten, StatusDryRunFailure:
			handledFailures++
			if item.Retryable {
				retryableFailures++
			} else {
				permanentFailures++
			}
		case StatusUnhandledError:
			unhandledFailures++
		case StatusSkipped:
			skipped++
		}
		if item.OutputPath != "" && (item.Status == StatusSummaryWritten || item.Status == StatusFailureWritten) {
			written++
		}
	}

	return map[string]any{
		"processed":         len(r.URLResults),
		"succeeded":         succeeded,
		"handledFailures":   handledFailures,
		"retryableFailures": retryableFailures,
		"permanentFailures": permanentFailures,
		"unhandledFailures": unhandledFailures,
		"skipped":           skipped,
		"written":           written,
		"flushed":           r.Flushed,
		"wouldFlush":        r.WouldFlush,
		"dryRun":            r.DryRun,
		"abortReason":       r.AbortReason,
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
	if r.AbortReason != "" {
		actions = append(actions, fmt.Sprintf("aborted: %s", r.AbortReason))
	}

	return actions
}

// RunLog builds the run-level summary written to logs/digest-runs.jsonl.
func (r *Result) RunLog(started time.Time, runErr error) *wikitypes.DigestRunEntry {
	now := time.Now()
	entry := &wikitypes.DigestRunEntry{
		Timestamp:       now.Format(time.RFC3339),
		BatchID:         "wiki-" + now.Format("2006-01-02"),
		StartedAt:       started.Format(time.RFC3339),
		DurationMs:      now.Sub(started).Milliseconds(),
		DryRun:          r.DryRun,
		Processed:       len(r.URLResults),
		Flushed:         r.Flushed,
		AbortReason:     r.AbortReason,
		FailureByReason: map[string]int{},
	}
	for i := range r.URLResults {
		item := &r.URLResults[i]
		switch item.Status {
		case StatusSummaryWritten, StatusDryRunSummary:
			entry.Succeeded++
		case StatusFailureWritten, StatusDryRunFailure:
			if item.Retryable {
				entry.Retryable++
			} else {
				entry.Permanent++
			}
		case StatusUnhandledError:
			entry.Unhandled++
		case StatusSkipped:
			entry.Skipped++
		}
		if item.OutputPath != "" && (item.Status == StatusSummaryWritten || item.Status == StatusFailureWritten) {
			entry.Written++
		}
		if item.FailureReason != "" {
			entry.FailureByReason[string(item.FailureReason)]++
		}
	}
	if len(entry.FailureByReason) == 0 {
		entry.FailureByReason = nil
	}
	if runErr != nil {
		entry.Error = runErr.Error()
	}

	return entry
}
