package dotfiles

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
	"github.com/xbpk3t/docs-alfred/pkg/output"
)

// FormatText returns the full text report for a DiffResult.
func FormatText(r *DiffResult) string {
	var b strings.Builder
	b.WriteString((&checkutil.Result{Issues: r.Issues}).ReportResult("dotfiles check"))
	b.WriteString(FormatCompact(r))
	return b.String()
}

// FormatJSON returns the JSON-serializable structure for a DiffResult.
func FormatJSON(r *DiffResult) map[string]any {
	return map[string]any{
		"name":    "dotfiles check",
		"ok":      !checkutil.HasErrors(r.Issues),
		"issues":  r.Issues,
		"summary": r.Summary,
	}
}

// WriteOutput writes the result in the given format to stdout.
func WriteOutput(format string, r *DiffResult) error {
	if format == output.FormatJSON {
		return output.WriteJSON(FormatJSON(r))
	}
	return writeString(FormatText(r))
}

// FormatDedupText returns the text report for dedup results.
func FormatDedupText(dups map[string][]string) string {
	if len(dups) == 0 {
		return "no duplicates found\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "found %d duplicate package references:\n", len(dups))
	for _, cats := range dups {
		sort.Strings(cats)
	}
	// Build sorted issues for consistent output
	type entry struct {
		pkg  string
		cats []string
	}
	entries := make([]entry, 0, len(dups))
	for pkg, cats := range dups {
		entries = append(entries, entry{pkg, cats})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].pkg < entries[j].pkg
	})
	for _, e := range entries {
		fmt.Fprintf(&b, "  pkgs.%s referenced in multiple categories: %s\n", e.pkg, strings.Join(e.cats, ", "))
	}
	return b.String()
}

// FormatDedupJSON returns the JSON-serializable structure for dedup results.
func FormatDedupJSON(dups map[string][]string) map[string]any {
	if len(dups) == 0 {
		return map[string]any{
			"name":    "dotfiles dedup",
			"ok":      true,
			"total":   0,
			"results": []checkutil.Issue{},
		}
	}
	var issues []checkutil.Issue
	for pkg, cats := range dups {
		sort.Strings(cats)
		issues = append(issues, checkutil.Issue{
			File:     pkg,
			Severity: checkutil.SeverityWarn,
			Message:  fmt.Sprintf("pkgs.%s referenced in multiple categories: %s", pkg, strings.Join(cats, ", ")),
		})
	}
	sort.Slice(issues, func(i, j int) bool {
		return issues[i].Message < issues[j].Message
	})
	return map[string]any{
		"name":    "dotfiles dedup",
		"ok":      true,
		"total":   len(dups),
		"results": issues,
	}
}

// WriteDedupOutput writes dedup results in the given format to stdout.
func WriteDedupOutput(format string, dups map[string][]string) error {
	if format == output.FormatJSON {
		return output.WriteJSON(FormatDedupJSON(dups))
	}
	return writeString(FormatDedupText(dups))
}

func writeString(s string) error {
	_, err := os.Stdout.WriteString(s)
	return err
}
