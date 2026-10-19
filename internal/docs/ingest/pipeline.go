package wikiingest

import (
	"context"
	"log/slog"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	wikiclassify "github.com/xbpk3t/docs-alfred/internal/docs/wiki/classify"
	wikifetch "github.com/xbpk3t/docs-alfred/internal/docs/wiki/fetch"
	wikitypes "github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
	wikiwrite "github.com/xbpk3t/docs-alfred/internal/docs/wiki/write"
)

// processAddURL fetches one URL and writes every entry it produces: a plain
// page yields one, a multi-item page (zhihu question) yields its synthesis plus
// one entry per child post.
func processAddURL(ctx context.Context, deps *dependencies, wikiRoot, urlStr string, dryRun bool) []URLResult {
	return writePendingURLs(deps, wikiRoot, prepareURLAttempt(ctx, deps, urlStr), dryRun)
}

// writePendingURLs writes every pending entry produced by one fetched URL.
func writePendingURLs(deps *dependencies, wikiRoot string, pending []pendingURLWrite, dryRun bool) []URLResult {
	results := make([]URLResult, 0, len(pending))
	for i := range pending {
		results = append(results, writePendingURL(deps, wikiRoot, &pending[i], dryRun))
	}

	return results
}

func runInboxEntries(
	ctx context.Context,
	deps *dependencies,
	wikiRoot string,
	entries []wikiwrite.InboxEntry,
	inboxCfg inboxConfig,
	dryRun bool,
) []URLResult {
	// One inbox line can yield several entries (a zhihu question produces its
	// synthesis plus one entry per answer), so each line maps to a slice.
	pending := make([][]pendingURLWrite, len(entries))
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

	// Every entry of a line carries that line's index, so the line flushes once.
	var results []URLResult
	for i, prepared := range pending {
		for _, result := range writePendingURLs(deps, wikiRoot, prepared, dryRun) {
			result.LineIndex = entries[i].LineIndex
			results = append(results, result)
		}
	}

	return results
}

func prepareInboxEntry(
	ctx context.Context,
	deps *dependencies,
	entry wikiwrite.InboxEntry,
	inboxCfg inboxConfig,
) []pendingURLWrite {
	// Skip URLs already digested in a previous run, or duplicated elsewhere in
	// this same batch: no fetch, no AI, no write. Handled stays true downstream
	// so the inbox line is flushed like any other processed URL.
	if deps.history != nil && deps.history.claimOrSeen(entry.URL) {
		slog.Info("wiki digest: skip already digested or duplicate URL", "url", entry.URL)

		return []pendingURLWrite{{URL: entry.URL, Kind: pendingSkip}}
	}

	urlCtx, cancel := context.WithTimeout(ctx, inboxCfg.perURLTimeout)
	defer cancel()

	// Fetch once — content doesn't change between retries, no point re-fetching.
	slog.Info("Processing wiki URL", "url", entry.URL)

	contentType := wikifetch.DetectContentType(entry.URL)
	fetchResult := deps.fetcher.FetchContent(urlCtx, entry.URL, contentType)
	if pending, failed := fetchFailurePending(entry.URL, fetchResult); failed {
		return []pendingURLWrite{pending}
	}

	if pending, tooShort := videoTooShortPending(entry.URL, contentType, fetchResult); tooShort {
		return []pendingURLWrite{pending}
	}

	if len(fetchResult.ChildURLs) > 0 {
		return expandChildPosts(urlCtx, deps, entry.URL, fetchResult)
	}

	// Classify using pre-fetched content. Outer retry removed (streaming
	// bypasses CF 524 timeout, so transient errors are rare). Inner retries
	// in classifyOnly handle AI call failures.
	return []pendingURLWrite{classifyPending(urlCtx, deps, entry.URL, fetchResult, wikiclassify.TopicPin{})}
}

// pendingFetchFailure maps a fetch outcome onto the failure entry it deserves.
// ok is false when the fetch succeeded. Every fetch site routes through here so
// a nil result, a driver-reported error and its failure kind are interpreted
// identically wherever a URL is fetched.
func fetchFailurePending(
	urlStr string,
	fetchResult *wikitypes.ContentFetchResult,
) (pendingURLWrite, bool) {
	if fetchResult == nil {
		return newPendingFetchFailure(urlStr, wikitypes.FailureFetch, "fetch content: empty result"), true
	}
	if fetchResult.Error != "" {
		return newPendingFetchFailure(urlStr, failureKindForFetchResult(fetchResult), "fetch content: "+fetchResult.Error), true
	}

	return pendingURLWrite{}, false
}

// videoTooShortPending reports the pre-classification content-quality gate:
// video content below the transcript threshold is metadata, not content, so it
// is an extraction failure rather than an uncategorised entry.
func videoTooShortPending(
	urlStr, contentType string,
	fetchResult *wikitypes.ContentFetchResult,
) (pendingURLWrite, bool) {
	if contentType != wikitypes.ContentVideo || len([]rune(fetchResult.Body)) >= minVideoRunes {
		return pendingURLWrite{}, false
	}

	item := &wikitypes.ClassifyItem{
		URL:         urlStr,
		Title:       fetchResult.Title,
		ContentType: contentType,
		Summary:     &wikitypes.StructuredSummary{Overview: fetchResult.Body},
	}

	return pendingExtractFailureWrite(item, "video content too short (likely no transcript)"), true
}

// minVideoRunes is the minimum transcript length for video content to be
// classifiable; below it the fetch returned metadata only.
const minVideoRunes = 600

// expandChildPosts digests a multi-item page as one synthesis entry for the
// container plus one entry per child post. The synthesis is classified first
// because the children inherit its topic; when it lands without a usable topic
// they fall back to choosing their own.
func expandChildPosts(
	ctx context.Context,
	deps *dependencies,
	parentURL string,
	fetchResult *wikitypes.ContentFetchResult,
) []pendingURLWrite {
	synthesis := classifyPending(ctx, deps, parentURL, fetchResult, wikiclassify.TopicPin{})
	pin := topicPinFromPending(&synthesis)

	out := make([]pendingURLWrite, 0, len(fetchResult.ChildURLs)+1)
	out = append(out, synthesis)
	for _, childURL := range fetchResult.ChildURLs {
		// Claim here so a child digested as part of its thread is never
		// re-digested when it shows up on its own later.
		if deps.history != nil && deps.history.claimOrSeen(childURL) {
			slog.Info("wiki digest: skip already digested child post", "url", childURL, "parent", parentURL)

			continue
		}
		out = append(out, prepareChildPost(ctx, deps, childURL, pin))
	}

	return out
}

// prepareChildPost fetches one child post (an answer permalink, whose body the
// container page only previews) and digests it under pin. Fetch and classify
// failures are recorded against the child URL, so one bad child never sinks the
// thread.
func prepareChildPost(
	ctx context.Context,
	deps *dependencies,
	childURL string,
	pin wikiclassify.TopicPin,
) pendingURLWrite {
	contentType := wikifetch.DetectContentType(childURL)
	fetchResult := deps.fetcher.FetchContent(ctx, childURL, contentType)
	if pending, failed := fetchFailurePending(childURL, fetchResult); failed {
		return pending
	}
	if pending, tooShort := videoTooShortPending(childURL, contentType, fetchResult); tooShort {
		return pending
	}

	return classifyPending(ctx, deps, childURL, fetchResult, pin)
}

// topicPinFromPending extracts the archive location a child post inherits. A
// synthesis that landed in uncat carries no topic, so children choose their own.
func topicPinFromPending(p *pendingURLWrite) wikiclassify.TopicPin {
	if p.Kind != pendingSummary || p.Item == nil || p.Item.NeedsManualReview || isUnclassifiedTopic(p.Item) {
		return wikiclassify.TopicPin{}
	}

	return wikiclassify.TopicPin{TopicPath: p.Item.TopicPath, WikiType: p.Item.Type}
}

// classifyPending classifies a fetched page and maps a classify error onto the
// pending kind the writer expects. pin pins the archive location for child
// posts; the zero pin means "choose your own topic".
func classifyPending(
	ctx context.Context,
	deps *dependencies,
	urlStr string,
	fetchResult *wikitypes.ContentFetchResult,
	pin wikiclassify.TopicPin,
) pendingURLWrite {
	title := fetchResult.Title
	if title == "" {
		title = urlStr
	}
	content := fetchResult.Body

	classResult := deps.classifier.ClassifyURLInTopic(ctx, urlStr, title, content, pin)
	if classResult == nil {
		item := &wikitypes.ClassifyItem{
			URL:         urlStr,
			Title:       title,
			ContentType: wikifetch.DetectContentType(urlStr),
			Summary:     &wikitypes.StructuredSummary{Overview: content},
		}
		// Empty content is a permanent content-side failure; non-empty content
		// with a nil classifier means the AI call failed, which is transient.
		if strings.TrimSpace(content) == "" {
			return pendingExtractFailureWrite(item, "extraction failed: empty content")
		}

		// Content was fetched but AI classify failed. Log as AI error JSONL
		// (inbox still flushes; human re-adds URL). Do not dump raw into uncat.
		return pendingURLWrite{
			URL:   urlStr,
			Kind:  pendingAIError,
			Error: "AI classify unavailable after retries: classification failed: AI error",
		}
	}

	item := classifyItemFromResult(urlStr, title, classResult)

	if shouldWriteClassifyFailure(classResult) {
		return pendingClassifyFailureWrite(item, classifyFailureInfo(classResult))
	}

	return pendingURLWrite{URL: urlStr, Kind: pendingSummary, Item: item}
}

type pendingWriteKind string

const (
	pendingSummary         pendingWriteKind = "summary"
	pendingClassifyFailure pendingWriteKind = "classify_failure"
	pendingExtractFailure  pendingWriteKind = "extract_failure"
	pendingFetchFailure    pendingWriteKind = "fetch_failure"
	pendingAIError         pendingWriteKind = "ai_error"
	pendingUnhandled       pendingWriteKind = "unhandled"
	pendingSkip            pendingWriteKind = "skip"
)

type pendingURLWrite struct {
	URL         string
	Kind        pendingWriteKind
	Item        *wikitypes.ClassifyItem
	FailureType wikitypes.FailureKind
	ExtraInfo   string
	Error       string
}

func prepareURLAttempt(ctx context.Context, deps *dependencies, urlStr string) []pendingURLWrite {
	slog.Info("Processing wiki URL", "url", urlStr)

	contentType := wikifetch.DetectContentType(urlStr)
	fetchResult := deps.fetcher.FetchContent(ctx, urlStr, contentType)
	if pending, failed := fetchFailurePending(urlStr, fetchResult); failed {
		return []pendingURLWrite{pending}
	}
	if pending, tooShort := videoTooShortPending(urlStr, contentType, fetchResult); tooShort {
		return []pendingURLWrite{pending}
	}

	if len(fetchResult.ChildURLs) > 0 {
		return expandChildPosts(ctx, deps, urlStr, fetchResult)
	}

	return []pendingURLWrite{classifyPending(ctx, deps, urlStr, fetchResult, wikiclassify.TopicPin{})}
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

func newPendingFetchFailure(urlStr string, failureType wikitypes.FailureKind, extraInfo string) pendingURLWrite {
	return pendingURLWrite{URL: urlStr, Kind: pendingFetchFailure, FailureType: failureType, ExtraInfo: extraInfo}
}

func newPendingUnhandled(urlStr, message string) pendingURLWrite {
	return pendingURLWrite{URL: urlStr, Kind: pendingUnhandled, Error: message}
}
