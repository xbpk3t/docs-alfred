package gh

import (
	"fmt"
	"net/url"

	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

const maxLines = 1000

// CheckResult holds the gh check results.
type CheckResult struct {
	Issues       []checkutil.Issue
	ScannedFiles int
	TotalEntries int
	TotalRecords int
}

// CheckIssue represents a single check issue.
type CheckIssue = checkutil.Issue

// RunGhCheck validates all data/gh YAML entries.
func RunGhCheck(ghRoot string) (*CheckResult, error) {
	result := &CheckResult{}

	err := WalkGhRepos(ghRoot, func(ev WalkerEvent) error {
		switch ev.Type {
		case evUnreadable:
			result.addIssue(ev.File, "error", "UNREADABLE")
		case evEmpty:
			result.addIssue(ev.File, "error", "EMPTY_FILE")
		case evFile:
			handleFileEvent(result, &ev)
		case evNotArray:
			result.addIssue(ev.File, "error",
				fmt.Sprintf("doc[%d]: expected array at root", ev.DocIndex))
		case evSection:
			handleSectionEvent(result, &ev)
		case evRepo:
			handleRepoEvent(result, &ev)
		}

		return nil
	})

	return result, err
}

// handleFileEvent processes a file event, checking line count limits.
func handleFileEvent(result *CheckResult, ev *WalkerEvent) {
	if ev.LineCount > maxLines {
		result.addIssue(ev.File, "error",
			fmt.Sprintf("FILE_TOO_LONG: %d lines (max %d)", ev.LineCount, maxLines))
	}
}

// handleSectionEvent processes a section event, validating type and record fields.
func handleSectionEvent(result *CheckResult, ev *WalkerEvent) {
	section := ev.Section
	typeVal, _ := section["type"].(string)
	if typeVal == "" {
		result.addIssue(ev.File, "error",
			fmt.Sprintf("section[%d]: missing or invalid 'type' field", ev.SectionIndex))
	} else if typeVal != ev.FilenameStem {
		result.addIssue(ev.File, "error",
			fmt.Sprintf("section[%d]: TYPE_MUST_MATCH_FILENAME expected %q, got %q",
				ev.SectionIndex, ev.FilenameStem, typeVal))
	}

	if _, hasRecord := section["record"]; !hasRecord {
		result.addIssue(ev.File, "warn",
			fmt.Sprintf("section[%d]: missing 'record' field", ev.SectionIndex))
	} else if record, ok := section["record"].([]any); !ok || record == nil {
		result.addIssue(ev.File, "error",
			fmt.Sprintf("section[%d]: 'record' must be an array", ev.SectionIndex))
	}
}

// handleRepoEvent processes a repo event, validating URL, topics, and records.
//

func handleRepoEvent(result *CheckResult, ev *WalkerEvent) {
	result.TotalEntries++
	repo := ev.Repo

	// Check URL
	urlStr, _ := repo["url"].(string)
	if urlStr == "" {
		result.addIssue(ev.File, "error",
			fmt.Sprintf("repo[%d]: missing or invalid url", ev.RepoIndex))
	} else if !isValidURL(urlStr) {
		result.addIssue(ev.File, "error",
			fmt.Sprintf("repo[%d]: invalid url format: %q", ev.RepoIndex, urlStr))
	}

	// Check record at repo level
	if record, hasRecord := repo["record"]; hasRecord {
		if _, ok := record.([]any); !ok {
			result.addIssue(ev.File, "error",
				fmt.Sprintf("repo[%d]: 'record' must be an array", ev.RepoIndex))
		}
	}

	// Check topics and their records
	checkRepoTopicRecords(result, ev.File, ev.RepoIndex, repo)

	// Check repo-level records
	checkRepoLevelRecords(result, ev.File, ev.RepoIndex, repo)
}

// checkRepoTopicRecords validates topic records in a repo entry.
func checkRepoTopicRecords(result *CheckResult, file string, repoIndex int, repo map[string]any) {
	topics, _ := repo["topics"].([]any)
	for topicIdx, t := range topics {
		topic, ok := t.(map[string]any)
		if !ok {
			continue
		}
		records, _ := topic["record"].([]any)
		result.TotalRecords += len(records)
		for recIdx, r := range records {
			rec, ok := r.(map[string]any)
			if !ok {
				continue
			}
			result.addIssueForRecord(file, repoIndex, topicIdx, recIdx, rec)
		}
	}
}

// checkRepoLevelRecords validates repo-level records.
func checkRepoLevelRecords(result *CheckResult, file string, repoIndex int, repo map[string]any) {
	if records, ok := repo["record"].([]any); ok {
		result.TotalRecords += len(records)
		for recIdx, r := range records {
			rec, ok := r.(map[string]any)
			if !ok {
				continue
			}
			result.addIssueForRecord(file, repoIndex, -1, recIdx, rec)
		}
	}
}

func (r *CheckResult) addIssue(file, severity, message string) {
	r.Issues = append(r.Issues, checkutil.Issue{
		File: file, Severity: severity, Message: message,
	})
}

func (r *CheckResult) addIssueForRecord(file string, repoIndex, topicIdx, recIdx int, rec map[string]any) {
	prefix := fmt.Sprintf("repo[%d]", repoIndex)
	if topicIdx >= 0 {
		prefix = fmt.Sprintf("repo[%d].topics[%d]", repoIndex, topicIdx)
	}

	dateStr, _ := rec["date"].(string)
	if dateStr != "" && !checkutil.DateFullPattern.MatchString(dateStr) {
		r.addIssue(file, "error",
			fmt.Sprintf("%s.record[%d]: invalid date format %q (expected YYYY-MM-DD)", prefix, recIdx, dateStr))
	}

	des, _ := rec["des"].(string)
	if des == "" {
		r.addIssue(file, "error",
			fmt.Sprintf("%s.record[%d]: missing or empty des", prefix, recIdx))
	}
}

// Report prints the check result.
func (r *CheckResult) Report(command string) {
	// Use checkutil for base formatting
	r2 := &checkutil.Result{Issues: r.Issues}
	r2.Report(command)
}

// HasErrors returns true if the check result has any error-severity issues.
func HasErrors(r *CheckResult) bool {
	return checkutil.HasErrors(r.Issues)
}

func isValidURL(str string) bool {
	u, err := url.Parse(str)

	return err == nil && u.Scheme != "" && u.Host != ""
}
