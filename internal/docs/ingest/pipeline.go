package wikiingest

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	wikifetch "github.com/xbpk3t/docs-alfred/internal/docs/wiki/fetch"
	wikitypes "github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
	wikiwrite "github.com/xbpk3t/docs-alfred/internal/docs/wiki/write"
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
	entries []wikiwrite.InboxEntry,
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
	entry wikiwrite.InboxEntry,
	inboxCfg inboxConfig,
) pendingURLWrite {
	urlCtx, cancel := context.WithTimeout(ctx, inboxCfg.perURLTimeout)
	defer cancel()

	// Fetch once — content doesn't change between retries, no point re-fetching.
	slog.Info("Processing wiki URL", "url", entry.URL)

	contentType := wikifetch.DetectContentType(entry.URL)
	fetchResult := deps.fetcher.FetchContent(urlCtx, entry.URL, contentType)
	if fetchResult == nil {
		return newPendingFetchFailure(entry.URL, wikitypes.FailureFetch, "fetch content: empty result")
	}
	if fetchResult.Error != "" {
		return newPendingFetchFailure(entry.URL, failureKindForFetchResult(fetchResult), "fetch content: "+fetchResult.Error)
	}

	// Pre-classification content quality: for video content, require enough
	// content for meaningful classification (i.e., actually got a transcript).
	if contentType == wikitypes.ContentVideo && len([]rune(fetchResult.Body)) < 600 {
		item := &wikitypes.ClassifyItem{URL: entry.URL, Title: fetchResult.Title, ContentType: contentType, Summary: &wikitypes.StructuredSummary{Overview: fetchResult.Body}}

		return pendingExtractFailureWrite(item, "video content too short (likely no transcript)")
	}

	// Classify using pre-fetched content. Outer retry removed (streaming
	// bypasses CF 524 timeout, so transient errors are rare). Inner retries
	// in classifyOnly handle AI call failures.
	result, classifyErr := classifyURLOnly(urlCtx, deps, entry.URL, fetchResult)
	if classifyErr != nil {
		var cerr *classifyRetryError
		if errors.As(classifyErr, &cerr) {
			// Content was fetched but AI classify failed. Log as AI error JSONL
			// (inbox still flushes; human re-adds URL). Do not dump raw into uncat.
			msg := "AI classify unavailable after retries"
			if cerr.message != "" {
				msg = msg + ": " + cerr.message
			}

			return pendingURLWrite{
				URL:   entry.URL,
				Kind:  pendingAIError,
				Error: msg,
			}
		}

		return newPendingUnhandled(entry.URL, classifyErr.Error())
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
	Item        *wikitypes.ClassifyItem
	FailureType wikitypes.FailureKind
	ExtraInfo   string
	Error       string
}

func prepareURLAttempt(ctx context.Context, deps *dependencies, urlStr string) (pendingURLWrite, error) {
	slog.Info("Processing wiki URL", "url", urlStr)

	contentType := wikifetch.DetectContentType(urlStr)
	fetchResult := deps.fetcher.FetchContent(ctx, urlStr, contentType)
	if fetchResult == nil {
		return pendingURLWrite{}, &fetchFailureError{failureType: wikitypes.FailureFetch, message: "fetch content: empty result"}
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
	if contentType == wikitypes.ContentVideo && len([]rune(content)) < 600 {
		item := &wikitypes.ClassifyItem{URL: urlStr, Title: title, ContentType: contentType, Summary: &wikitypes.StructuredSummary{Overview: content}}

		return pendingExtractFailureWrite(item, "video content too short (likely no transcript)"), nil
	}

	classResult := deps.classifier.ClassifyURL(ctx, urlStr, title, content)
	if classResult == nil {
		// Distinguish: empty content is a permanent classify failure (content-side issue);
		// non-empty content with nil classifier means AI call failed (transient).
		item := &wikitypes.ClassifyItem{URL: urlStr, Title: title, ContentType: contentType, Summary: &wikitypes.StructuredSummary{Overview: content}}
		if strings.TrimSpace(content) == "" {
			return pendingExtractFailureWrite(item, "extraction failed: empty content"), nil
		}

		return pendingURLWrite{
			URL:   urlStr,
			Kind:  pendingAIError,
			Error: "AI classify unavailable after retries: classification failed: AI error",
		}, nil
	}

	item := classifyItemFromResult(urlStr, title, classResult)

	if shouldWriteClassifyFailure(classResult) {
		extraInfo := classifyFailureInfo(classResult)

		return pendingClassifyFailureWrite(item, extraInfo), nil
	}

	return pendingURLWrite{URL: urlStr, Kind: pendingSummary, Item: item}, nil
}

// classifyURLOnly runs only the AI classification step using pre-fetched content.
// Used by prepareInboxEntry to avoid re-fetching on retry.
func classifyURLOnly(ctx context.Context, deps *dependencies, urlStr string, fetchResult *wikitypes.ContentFetchResult) (pendingURLWrite, error) {
	title := fetchResult.Title
	if title == "" {
		title = urlStr
	}

	content := fetchResult.Body

	classResult := deps.classifier.ClassifyURL(ctx, urlStr, title, content)
	if classResult == nil {
		item := &wikitypes.ClassifyItem{URL: urlStr, Title: title, ContentType: wikifetch.DetectContentType(urlStr), Summary: &wikitypes.StructuredSummary{Overview: content}}
		if strings.TrimSpace(content) == "" {
			return pendingExtractFailureWrite(item, "extraction failed: empty content"), nil
		}

		return pendingURLWrite{}, &classifyRetryError{message: "classification failed: AI error"}
	}

	item := classifyItemFromResult(urlStr, title, classResult)

	if shouldWriteClassifyFailure(classResult) {
		extraInfo := classifyFailureInfo(classResult)

		return pendingClassifyFailureWrite(item, extraInfo), nil
	}

	return pendingURLWrite{URL: urlStr, Kind: pendingSummary, Item: item}, nil
}

func classifyItemFromResult(urlStr, title string, classResult *wikitypes.ClassifyResult) *wikitypes.ClassifyItem {
	return &wikitypes.ClassifyItem{
		URL:               urlStr,
		Title:             title,
		ContentType:       classResult.ContentType,
		TopicPath:         classResult.TopicPath,
		Type:              classResult.WikiType,
		Summary:           classResult.Summary,
		MetadataBlock:     classResult.MetadataBlock,
		SuggestedTopic:    classResult.SuggestedTopic,
		RouteReason:       classResult.RouteReason,
		Confidence:        classResult.Confidence,
		NeedsManualReview: classResult.NeedsManualReview,
	}
}

func pendingClassifyFailureWrite(item *wikitypes.ClassifyItem, extraInfo string) pendingURLWrite {
	return pendingURLWrite{
		URL:         item.URL,
		Kind:        pendingClassifyFailure,
		Item:        item,
		FailureType: wikitypes.FailureClassify,
		ExtraInfo:   extraInfo,
	}
}

func pendingExtractFailureWrite(item *wikitypes.ClassifyItem, extraInfo string) pendingURLWrite {
	return pendingURLWrite{
		URL:         item.URL,
		Kind:        pendingExtractFailure,
		Item:        item,
		FailureType: wikitypes.FailureExtract,
		ExtraInfo:   extraInfo,
	}
}

func newPendingAIError(urlStr, message string) pendingURLWrite {
	return pendingURLWrite{URL: urlStr, Kind: pendingAIError, Error: message}
}

func newPendingFetchFailure(urlStr string, failureType wikitypes.FailureKind, extraInfo string) pendingURLWrite {
	return pendingURLWrite{URL: urlStr, Kind: pendingFetchFailure, FailureType: failureType, ExtraInfo: extraInfo}
}

func newPendingUnhandled(urlStr, message string) pendingURLWrite {
	return pendingURLWrite{URL: urlStr, Kind: pendingUnhandled, Error: message}
}
