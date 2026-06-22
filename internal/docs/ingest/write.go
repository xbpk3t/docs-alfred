package wikiingest

import (
	"fmt"
	"strings"

	wikisvc "github.com/xbpk3t/docs-alfred/internal/docs/wiki"
)

func writePendingURL(deps *dependencies, wikiRoot string, pending *pendingURLWrite, dryRun bool) URLResult {
	if pending == nil {
		return URLResult{Status: StatusUnhandledError, Error: "missing pending wiki write"}
	}
	switch pending.Kind {
	case pendingSummary:
		return writeSummary(deps, wikiRoot, pending.Item, dryRun)
	case pendingClassifyFailure:
		return writeClassifyFailure(deps, wikiRoot, pending.Item, pending.ExtraInfo, dryRun)
	case pendingExtractFailure:
		return writeExtractFailure(deps, wikiRoot, pending.Item, pending.ExtraInfo, dryRun)
	case pendingFetchFailure:
		return writeFetchFailure(deps, wikiRoot, pending.URL, pending.FailureType, pending.ExtraInfo, dryRun)
	case pendingAIError:
		return writeAIError(deps, wikiRoot, pending.URL, pending.Error, dryRun)
	case pendingUnhandled:
		return URLResult{URL: pending.URL, Status: StatusUnhandledError, Error: pending.Error}
	default:
		return URLResult{URL: pending.URL, Status: StatusUnhandledError, Error: "missing pending wiki write"}
	}
}

func writeSummary(deps *dependencies, wikiRoot string, item *wikisvc.ClassifyItem, dryRun bool) URLResult {
	// Items with NeedsManualReview and good content get written to wiki/uncat.md
	// for manual triage, not under a topic dir.
	if item.NeedsManualReview {
		path, err := deps.writer.WriteManualReviewEntry(
			item,
			&wikisvc.WriteOptions{WikiRoot: wikiRoot, DryRun: dryRun},
		)
		if err != nil {
			return URLResult{URL: item.URL, Status: StatusUnhandledError, Error: fmt.Sprintf("write manual review: %v", err)}
		}

		status := StatusSummaryWritten
		if dryRun {
			status = StatusDryRunSummary
		}

		return URLResult{
			URL:         item.URL,
			Status:      status,
			Handled:     true,
			OutputPath:  path,
			TopicPath:   item.TopicPath,
			WikiType:    string(item.Type),
			ContentType: item.ContentType,
		}
	}

	path, err := deps.writer.WriteSummary(item, &wikisvc.WriteOptions{WikiRoot: wikiRoot, DryRun: dryRun})
	if err != nil {
		return URLResult{URL: item.URL, Status: StatusUnhandledError, Error: fmt.Sprintf("write summary: %v", err)}
	}

	status := StatusSummaryWritten
	if dryRun {
		status = StatusDryRunSummary
	}

	return URLResult{
		URL:         item.URL,
		Status:      status,
		Handled:     true,
		OutputPath:  path,
		TopicPath:   item.TopicPath,
		WikiType:    string(item.Type),
		ContentType: item.ContentType,
	}
}

func shouldWriteClassifyFailure(result *wikisvc.ClassifyResult) bool {
	if result == nil {
		return true
	}
	// If NeedsManualReview but content was good (AI produced a valid summary),
	// treat as success — write layer will route to uncat.md for manual review.
	if result.NeedsManualReview && result.Summary != nil && strings.TrimSpace(result.Summary.Overview) != "" {
		return false
	}

	return result.RejectReason != "" || result.NeedsManualReview || result.WikiType == wikisvc.TypeInbox ||
		result.TopicPath == unclassifiedTopicPath || result.TopicPath == inboxTopicPath
}

func classifyFailureInfo(result *wikisvc.ClassifyResult) string {
	if result == nil {
		return "classification failed (returned nil)"
	}
	var lines []string
	reason := strings.TrimSpace(result.RejectReason)
	if reason == "" {
		reason = "AI marked the item as inbox/manual review"
	}
	lines = append(lines, reason)
	if result.TopicPath != "" {
		lines = append(lines, "Topic: "+result.TopicPath)
	}
	if result.WikiType != "" {
		lines = append(lines, "WikiType: "+string(result.WikiType))
	}
	if result.Confidence > 0 {
		lines = append(lines, fmt.Sprintf("Confidence: %.2f", result.Confidence))
	}
	if result.NeedsManualReview {
		lines = append(lines, "NeedsManualReview: true")
	}
	if result.Summary != nil {
		if s := strings.TrimSpace(wikisvc.RenderStructuredSummary(result.Summary)); s != "" {
			lines = append(lines, "Summary: "+s)
		}
	}

	return strings.Join(lines, "\n")
}

func writeClassifyFailure(
	deps *dependencies,
	wikiRoot string,
	item *wikisvc.ClassifyItem,
	extraInfo string,
	dryRun bool,
) URLResult {
	path, err := deps.writer.WriteFailureEntry(
		item,
		wikisvc.FailureClassify,
		extraInfo,
		&wikisvc.WriteOptions{WikiRoot: wikiRoot, DryRun: dryRun},
	)
	if err != nil {
		return URLResult{
			URL:         item.URL,
			Status:      StatusUnhandledError,
			FailureType: wikisvc.FailureClassify,
			Error:       fmt.Sprintf("write classify failure: %v", err),
		}
	}

	status := StatusFailureWritten
	if dryRun {
		status = StatusDryRunFailure
	}

	return URLResult{
		URL:         item.URL,
		Status:      status,
		Handled:     true,
		OutputPath:  path,
		TopicPath:   item.TopicPath,
		WikiType:    string(item.Type),
		ContentType: item.ContentType,
		FailureType: wikisvc.FailureClassify,
	}
}

func writeExtractFailure(
	deps *dependencies,
	wikiRoot string,
	item *wikisvc.ClassifyItem,
	extraInfo string,
	dryRun bool,
) URLResult {
	path, err := deps.writer.WriteFailureEntry(
		item,
		wikisvc.FailureExtract,
		extraInfo,
		&wikisvc.WriteOptions{WikiRoot: wikiRoot, DryRun: dryRun},
	)
	if err != nil {
		return URLResult{
			URL:         item.URL,
			Status:      StatusUnhandledError,
			FailureType: wikisvc.FailureExtract,
			Error:       fmt.Sprintf("write extract failure: %v", err),
		}
	}

	status := StatusFailureWritten
	if dryRun {
		status = StatusDryRunFailure
	}

	return URLResult{
		URL:         item.URL,
		Status:      status,
		Handled:     true,
		OutputPath:  path,
		FailureType: wikisvc.FailureExtract,
	}
}

func writeFetchFailure(
	deps *dependencies,
	wikiRoot,
	urlStr string,
	failureType wikisvc.FailureKind,
	extraInfo string,
	dryRun bool,
) URLResult {
	item := &wikisvc.ClassifyItem{URL: urlStr, Title: urlStr}
	path, err := deps.writer.WriteFailureEntry(
		item,
		failureType,
		extraInfo,
		&wikisvc.WriteOptions{WikiRoot: wikiRoot, DryRun: dryRun},
	)
	if err != nil {
		return URLResult{
			URL:         urlStr,
			Status:      StatusUnhandledError,
			FailureType: failureType,
			Error:       fmt.Sprintf("write %s failure: %v", failureType, err),
		}
	}

	status := StatusFailureWritten
	if dryRun {
		status = StatusDryRunFailure
	}

	return URLResult{
		URL:         urlStr,
		Status:      status,
		Handled:     true,
		OutputPath:  path,
		FailureType: failureType,
	}
}

func writeAIError(
	deps *dependencies,
	wikiRoot,
	urlStr,
	message string,
	dryRun bool,
) URLResult {
	item := &wikisvc.ClassifyItem{URL: urlStr, Title: urlStr}
	path, err := deps.writer.WriteFailureEntry(
		item,
		wikisvc.FailureAI,
		message,
		&wikisvc.WriteOptions{WikiRoot: wikiRoot, DryRun: dryRun},
	)
	if err != nil {
		return URLResult{
			URL:         urlStr,
			Status:      StatusUnhandledError,
			FailureType: wikisvc.FailureAI,
			Error:       fmt.Sprintf("write AI error: %v", err),
		}
	}

	status := StatusFailureWritten
	if dryRun {
		status = StatusDryRunFailure
	}

	return URLResult{
		URL:         urlStr,
		Status:      status,
		Handled:     true,
		OutputPath:  path,
		FailureType: wikisvc.FailureAI,
	}
}
