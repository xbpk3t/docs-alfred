package wikiingest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"sort"
	"strings"
	"time"

	wikicompact "github.com/xbpk3t/docs-alfred/internal/docs/wiki/compact"
	wikitypes "github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
	wikiwrite "github.com/xbpk3t/docs-alfred/internal/docs/wiki/write"
)

const digestNotifyTitle = "wiki digest"

// NotifyInput drives the digest run notification sent by the dagu failure
// handler.
type NotifyInput struct {
	Config *Config
	// MailToken is the Resend token, resolved from env by the CLI.
	MailToken string
	// Failed marks the notification as a failed DAG run even when the latest
	// run log is stale or missing.
	Failed bool
	DryRun bool
}

// NotifyResult is the rendered notification.
type NotifyResult struct {
	Subject string
	HTML    string
	Sent    bool
}

// RunDigestNotify reads the latest run summary from R2 and sends it via Resend.
// A missing run log still produces an alert (the run may have been killed
// before it could write one).
func RunDigestNotify(ctx context.Context, input NotifyInput) (*NotifyResult, error) {
	if input.Config == nil {
		return nil, fmt.Errorf("wiki config is required")
	}

	var (
		entry   *wikitypes.DigestRunEntry
		readErr error
	)
	if store := newMediaStore(input.Config); store != nil {
		data, err := store.GetObject(ctx, wikiwrite.DigestRunLogKey)
		if err != nil {
			readErr = err
		} else {
			entry = lastRunLogEntry(data)
		}
	}

	subject, body := buildDigestNotify(input.Config, entry, input.Failed)
	if readErr != nil {
		body += fmt.Sprintf("<p><b>Run log unavailable:</b> %s</p>", html.EscapeString(readErr.Error()))
	}
	result := &NotifyResult{Subject: subject, HTML: body}
	if input.DryRun {
		return result, nil
	}

	if input.MailToken == "" {
		return result, fmt.Errorf("RESEND_TOKEN is required to send digest notifications")
	}
	mailTo := input.Config.Compact.Send.Resend.MailTo
	if len(mailTo) == 0 {
		return result, fmt.Errorf("compact.send.resend.mailTo is required to send digest notifications")
	}
	if err := wikicompact.SendCompactEmail(ctx, &wikicompact.MailConfig{
		Token:  input.MailToken,
		MailTo: mailTo,
	}, digestNotifyTitle, subject, body); err != nil {
		return result, err
	}
	result.Sent = true

	return result, nil
}

// lastRunLogEntry returns the last parseable run-summary line, or nil when the
// log has no entries.
func lastRunLogEntry(data []byte) *wikitypes.DigestRunEntry {
	remaining := bytes.TrimRight(data, "\n")
	for len(remaining) > 0 {
		i := bytes.LastIndexByte(remaining, '\n')
		line := bytes.TrimSpace(remaining[i+1:])
		if len(line) > 0 {
			var entry wikitypes.DigestRunEntry
			if json.Unmarshal(line, &entry) == nil {
				return &entry
			}
		}
		if i < 0 {
			break
		}
		remaining = remaining[:i]
	}

	return nil
}

// buildDigestNotify renders the subject and HTML body. It is pure so it can be
// tested without Resend.
func buildDigestNotify(cfg *Config, entry *wikitypes.DigestRunEntry, failed bool) (subject, body string) {
	status := digestNotifyStatus(entry, failed)
	title := digestNotifyTitle
	batch := "no run log"
	if entry != nil && entry.BatchID != "" {
		batch = entry.BatchID
	}
	subject = fmt.Sprintf("[%s] %s — %s", title, status, batch)

	var b strings.Builder
	fmt.Fprintf(&b, "<h2>%s: %s</h2>", html.EscapeString(title), html.EscapeString(status))
	if entry == nil {
		b.WriteString("<p>No digest run summary was produced. Check the dagu run log on the runner.</p>")

		return subject, b.String()
	}

	fmt.Fprintf(&b,
		"<p>Batch: %s<br>Started: %s<br>Duration: %d ms<br>Dry run: %v</p>",
		html.EscapeString(entry.BatchID), html.EscapeString(entry.StartedAt), entry.DurationMs, entry.DryRun)
	if started, err := time.Parse(time.RFC3339, entry.StartedAt); err == nil && time.Since(started) > 6*time.Hour {
		fmt.Fprintf(&b,
			"<p><b>Note:</b> this summary is stale (started %s). The current run may have been killed before writing its summary.</p>",
			html.EscapeString(entry.StartedAt))
	}
	fmt.Fprintf(&b,
		"<table border=\"1\" cellpadding=\"4\" cellspacing=\"0\">"+
			"<tr><td>processed</td><td>%d</td></tr>"+
			"<tr><td>succeeded</td><td>%d</td></tr>"+
			"<tr><td>retryable</td><td>%d</td></tr>"+
			"<tr><td>permanent</td><td>%d</td></tr>"+
			"<tr><td>unhandled</td><td>%d</td></tr>"+
			"<tr><td>skipped</td><td>%d</td></tr>"+
			"<tr><td>flushed</td><td>%d</td></tr></table>",
		entry.Processed, entry.Succeeded, entry.Retryable, entry.Permanent,
		entry.Unhandled, entry.Skipped, entry.Flushed)

	writeFailureReasonTable(&b, entry.FailureByReason)
	if entry.AbortReason != "" {
		fmt.Fprintf(&b, "<p><b>Aborted:</b> %s</p>", html.EscapeString(entry.AbortReason))
	}
	if entry.Error != "" {
		fmt.Fprintf(&b, "<p><b>Error:</b> %s</p>", html.EscapeString(entry.Error))
	}
	if base := strings.TrimRight(cfg.R2.PublicBaseURL, "/"); base != "" {
		fmt.Fprintf(&b, "<p><a href=\"%s/%s\">run log</a></p>", html.EscapeString(base), wikiwrite.DigestRunLogKey)
	}

	return subject, b.String()
}

func digestNotifyStatus(entry *wikitypes.DigestRunEntry, failed bool) string {
	status := "OK"
	if failed {
		status = "FAILED"
	}
	if entry == nil {
		return status
	}
	switch {
	case entry.AbortReason != "":
		return "ABORTED (" + entry.AbortReason + ")"
	case entry.Error != "":
		return "ERROR"
	}

	return status
}

func writeFailureReasonTable(b *strings.Builder, reasons map[string]int) {
	if len(reasons) == 0 {
		return
	}
	keys := make([]string, 0, len(reasons))
	for reason := range reasons {
		keys = append(keys, reason)
	}
	sort.Strings(keys)
	b.WriteString("<h3>Failures by reason</h3><table border=\"1\" cellpadding=\"4\" cellspacing=\"0\">")
	for _, reason := range keys {
		fmt.Fprintf(b, "<tr><td>%s</td><td>%d</td></tr>", html.EscapeString(reason), reasons[reason])
	}
	b.WriteString("</table>")
}
