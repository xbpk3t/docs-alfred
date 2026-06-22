package wikiingest

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/avast/retry-go/v4"
	"golang.org/x/sync/errgroup"

	wikisvc "github.com/xbpk3t/docs-alfred/internal/docs/wiki"
)

func processAddURL(ctx context.Context, deps *dependencies, wikiRoot, urlStr string, dryRun bool) URLResult {
	pending, err := prepareURLAttempt(ctx, deps, urlStr)
	if err != nil {
		var fetchErr *fetchFailureError
		if errors.As(err, &fetchErr) {
			fetchPending := newPendingFetchFailure(urlStr, fetchErr.failureType, fetchErr.Error())

			return writePendingURL(deps, wikiRoot, &fetchPending, dryRun)
		}

		return URLResult{URL: urlStr, Status: StatusUnhandledError, Error: err.Error()}
	}

	return writePendingURL(deps, wikiRoot, &pending, dryRun)
}

func runInboxEntries(
	ctx context.Context,
	deps *dependencies,
	wikiRoot string,
	entries []wikisvc.InboxEntry,
	inboxCfg inboxConfig,
	dryRun bool,
) []URLResult {
	pending := make([]pendingURLWrite, len(entries))
	var mu sync.Mutex

	g, groupCtx := errgroup.WithContext(ctx)
	g.SetLimit(inboxCfg.concurrency)

	for i, entry := range entries {
		g.Go(func() error {
			prepared := prepareInboxEntry(groupCtx, deps, entry, inboxCfg)

			mu.Lock()
			pending[i] = prepared
			mu.Unlock()

			return nil
		})
	}
	_ = g.Wait()

	results := make([]URLResult, len(entries))
	for i, prepared := range pending {
		result := writePendingURL(deps, wikiRoot, &prepared, dryRun)
		result.LineIndex = entries[i].LineIndex
		results[i] = result
	}

	return results
}

func prepareInboxEntry(
	ctx context.Context,
	deps *dependencies,
	entry wikisvc.InboxEntry,
	inboxCfg inboxConfig,
) pendingURLWrite {
	urlCtx, cancel := context.WithTimeout(ctx, inboxCfg.perURLTimeout)
	defer cancel()

	var result pendingURLWrite
	err := retry.Do(
		func() error {
			attemptResult, attemptErr := prepareURLAttempt(urlCtx, deps, entry.URL)
			if attemptErr == nil {
				result = attemptResult
			}

			return attemptErr
		},
		retry.Context(urlCtx),
		retry.Attempts(inboxCfg.maxRetries),
		retry.Delay(5*time.Second),
		retry.DelayType(retry.BackOffDelay),
		retry.LastErrorOnly(true),
		retry.RetryIf(func(err error) bool {
			var fetchErr *fetchFailureError
			if errors.As(err, &fetchErr) {
				return true
			}
			var classifyErr *classifyRetryError

			return errors.As(err, &classifyErr)
		}),
		retry.OnRetry(func(n uint, retryErr error) {
			slog.Warn("Retrying processing wiki URL", "url", entry.URL, "attempt", n+1, "error", retryErr)
		}),
	)
	if err != nil {
		var fetchErr *fetchFailureError
		if errors.As(err, &fetchErr) {
			return newPendingFetchFailure(entry.URL, fetchErr.failureType, fetchErr.Error())
		}

		var classifyErr *classifyRetryError
		if errors.As(err, &classifyErr) {
			return newPendingAIError(entry.URL, classifyErr.Error())
		}

		return newPendingUnhandled(entry.URL, err.Error())
	}

	return result
}

type pendingWriteKind string

const (
	pendingSummary         pendingWriteKind = "summary"
	pendingClassifyFailure pendingWriteKind = "classify_failure"
	pendingExtractFailure  pendingWriteKind = "extract_failure"
	pendingFetchFailure    pendingWriteKind = "fetch_failure"
	pendingAIError         pendingWriteKind = "ai_error"
	pendingUnhandled       pendingWriteKind = "unhandled"
)

type pendingURLWrite struct {
	URL         string
	Kind        pendingWriteKind
	Item        *wikisvc.ClassifyItem
	FailureType wikisvc.FailureKind
	ExtraInfo   string
	Error       string
}

func prepareURLAttempt(ctx context.Context, deps *dependencies, urlStr string) (pendingURLWrite, error) {
	slog.Info("Processing wiki URL", "url", urlStr)

	contentType := wikisvc.DetectContentType(urlStr)
	fetchResult := deps.fetcher.FetchContent(ctx, urlStr, contentType)
	if fetchResult == nil {
		return pendingURLWrite{}, &fetchFailureError{failureType: wikisvc.FailureFetch, message: "fetch content: empty result"}
	}
	if fetchResult.Error != "" {
		return pendingURLWrite{}, &fetchFailureError{
			failureType: failureKindForFetchResult(fetchResult),
			message:     "fetch content: " + fetchResult.Error,
		}
	}

	title := fetchResult.Title
	if title == "" {
		title = urlStr
	}

	content := fetchResult.Body

	// Pre-classification content quality: for video content, require enough
	// content for meaningful classification (i.e., actually got a transcript).
	if contentType == wikisvc.ContentVideo && len([]rune(content)) < 600 {
		item := &wikisvc.ClassifyItem{URL: urlStr, Title: title, ContentType: contentType, Summary: &wikisvc.StructuredSummary{Overview: content}}

		return pendingExtractFailureWrite(item, "video content too short (likely no transcript)"), nil
	}

	classResult := deps.classifier.ClassifyURL(ctx, urlStr, title, content)
	if classResult == nil {
		// Distinguish: empty content is a permanent classify failure (content-side issue);
		// non-empty content with nil classifier means AI call failed (transient).
		item := &wikisvc.ClassifyItem{URL: urlStr, Title: title, ContentType: contentType, Summary: &wikisvc.StructuredSummary{Overview: content}}
		if strings.TrimSpace(content) == "" {
			return pendingExtractFailureWrite(item, "extraction failed: empty content"), nil
		}

		return pendingURLWrite{}, &classifyRetryError{message: "classification failed: AI error"}
	}

	item := &wikisvc.ClassifyItem{
		URL:               urlStr,
		Title:             title,
		ContentType:       classResult.ContentType,
		TopicPath:         classResult.TopicPath,
		Type:              classResult.WikiType,
		Summary:           classResult.Summary,
		MetadataBlock:     classResult.MetadataBlock,
		NeedsManualReview: classResult.NeedsManualReview,
	}

	if shouldWriteClassifyFailure(classResult) {
		extraInfo := classifyFailureInfo(classResult)

		return pendingClassifyFailureWrite(item, extraInfo), nil
	}

	return pendingURLWrite{URL: urlStr, Kind: pendingSummary, Item: item}, nil
}

func pendingClassifyFailureWrite(item *wikisvc.ClassifyItem, extraInfo string) pendingURLWrite {
	return pendingURLWrite{
		URL:         item.URL,
		Kind:        pendingClassifyFailure,
		Item:        item,
		FailureType: wikisvc.FailureClassify,
		ExtraInfo:   extraInfo,
	}
}

func pendingExtractFailureWrite(item *wikisvc.ClassifyItem, extraInfo string) pendingURLWrite {
	return pendingURLWrite{
		URL:         item.URL,
		Kind:        pendingExtractFailure,
		Item:        item,
		FailureType: wikisvc.FailureExtract,
		ExtraInfo:   extraInfo,
	}
}

func newPendingAIError(urlStr, message string) pendingURLWrite {
	return pendingURLWrite{URL: urlStr, Kind: pendingAIError, Error: message}
}

func newPendingFetchFailure(urlStr string, failureType wikisvc.FailureKind, extraInfo string) pendingURLWrite {
	return pendingURLWrite{URL: urlStr, Kind: pendingFetchFailure, FailureType: failureType, ExtraInfo: extraInfo}
}

func newPendingUnhandled(urlStr, message string) pendingURLWrite {
	return pendingURLWrite{URL: urlStr, Kind: pendingUnhandled, Error: message}
}
