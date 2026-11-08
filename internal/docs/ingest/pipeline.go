package wikiingest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
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

// maxConsecutiveAIFailures is the circuit breaker threshold: this many
// consecutive AI/catalog failures means the gateway or account quota is down,
// so the rest of the batch would only burn fetch work and retries.
const maxConsecutiveAIFailures = 5

// errAIBreaker is returned when a digest run is stopped early by the breaker.
var errAIBreaker = errors.New("wiki digest aborted: consecutive AI failures")

// aiFailureBreaker counts consecutive AI-side failures across workers and
// cancels the run context once the threshold is hit.
type aiFailureBreaker struct {
	cancel      context.CancelFunc
	consecutive int
	mu          sync.Mutex
	tripped     bool
}

func newAIFailureBreaker(cancel context.CancelFunc) *aiFailureBreaker {
	return &aiFailureBreaker{cancel: cancel}
}

// record updates the breaker with one line's results. Skipped URLs make no AI
// call, so they leave the streak untouched; any other non-AI outcome resets it.
func (b *aiFailureBreaker) record(results []URLResult) {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.tripped {
		return
	}
	for i := range results {
		result := &results[i]
		switch {
		case isAIFailure(result):
			b.consecutive++
		case result.Status == StatusSkipped:
			// no AI call; keep the current streak
		default:
			b.consecutive = 0
		}
		if b.consecutive >= maxConsecutiveAIFailures {
			b.tripped = true
			slog.Warn("wiki digest: AI failure breaker tripped; stopping remaining work",
				"consecutive", b.consecutive)
			b.cancel()

			return
		}
	}
}

func (b *aiFailureBreaker) Tripped() bool {
	if b == nil {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.tripped
}

func isAIFailure(result *URLResult) bool {
	return result.FailureType == wikitypes.FailureAI &&
		(result.Status == StatusFailureWritten || result.Status == StatusDryRunFailure) &&
		(result.FailureReason == wikitypes.FailureReasonAI ||
			result.FailureReason == wikitypes.FailureReasonCatalog)
}

func runInboxEntries(
	ctx context.Context,
	deps *dependencies,
	wikiRoot string,
	entries []wikiwrite.InboxEntry,
	inboxCfg inboxConfig,
	dryRun bool,
	breaker *aiFailureBreaker,
) []URLResult {
	var (
		mu      sync.Mutex
		results []URLResult
	)

	g, groupCtx := errgroup.WithContext(ctx)
	g.SetLimit(inboxCfg.concurrency)

	for _, entry := range entries {
		g.Go(func() error {
			// The breaker already stopped the run: do not fetch, classify or log
			// the remaining URLs. The cause is recorded by the breaker, so a nil
			// return here is intentional.
			if groupCtx.Err() != nil {
				return nil //nolint:nilerr
			}
			prepared := prepareInboxEntry(groupCtx, deps, entry, inboxCfg)
			lineResults := writeLineResults(deps, wikiRoot, prepared, dryRun)
			for i := range lineResults {
				lineResults[i].LineIndex = entry.LineIndex
			}
			breaker.record(lineResults)

			mu.Lock()
			results = append(results, lineResults...)
			mu.Unlock()

			return nil
		})
	}
	_ = g.Wait()

	// Writes happen as each line finishes, so sort back into inbox order for a
	// stable CLI output.
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].LineIndex < results[j].LineIndex
	})

	return results
}

// writeLineResults writes one inbox line's pending entries. A multi-entry line
// (synthesis + child posts) writes children first and the synthesis last: if a
// child is still retryable, the synthesis keeps the line (Handled=false) and
// skips success history, so the next run re-expands the thread and retries the
// child instead of orphaning it.
func writeLineResults(deps *dependencies, wikiRoot string, pending []pendingURLWrite, dryRun bool) []URLResult {
	if len(pending) <= 1 {
		return writePendingURLs(deps, wikiRoot, pending, dryRun)
	}

	results := make([]URLResult, 0, len(pending))
	lineComplete := true
	for i := 1; i < len(pending); i++ {
		result := writePendingURL(deps, wikiRoot, &pending[i], dryRun)
		if !result.Handled {
			lineComplete = false
		}
		results = append(results, result)
	}

	parent := pending[0]
	parent.SkipHistory = !lineComplete
	parentResult := writePendingURL(deps, wikiRoot, &parent, dryRun)
	if !lineComplete {
		parentResult.Handled = false
	}
	results = append(results, parentResult)

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
	if count := deps.failures.authFailures(entry.URL); count >= maxAuthFailures {
		slog.Warn("wiki digest: auth retry budget exhausted", "url", entry.URL, "count", count)

		return []pendingURLWrite{newPendingFetchFailure(
			entry.URL,
			wikitypes.FailureExtract,
			wikitypes.FailureReasonRetryExhausted,
			fmt.Sprintf("retry limit reached: %d prior auth failures", count),
		)}
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

	// Archive the transcript remotely (R2) so the entry can link the full text
	// without storing it in the wiki repo. Best-effort: "" means no link.
	transcriptURL := maybeUploadTranscript(urlCtx, deps, entry.URL, fetchResult)

	// Classify using pre-fetched content. Outer retry removed (streaming
	// bypasses CF 524 timeout, so transient errors are rare). Inner retries
	// in classifyOnly handle AI call failures.
	pending := classifyPending(urlCtx, deps, entry.URL, fetchResult, wikiclassify.TopicPin{})
	attachTranscriptURL(&pending, transcriptURL)

	return []pendingURLWrite{pending}
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
		return newPendingFetchFailure(
			urlStr, wikitypes.FailureFetch, wikitypes.FailureReasonHTTPError, "fetch content: empty result",
		), true
	}
	if fetchResult.Error != "" {
		return newPendingFetchFailure(
			urlStr,
			failureKindForFetchResult(fetchResult),
			failureReasonForFetchResult(fetchResult),
			"fetch content: "+fetchResult.Error,
		), true
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
	// Video metadata tables are long enough to pass a length-only check
	// (bilibili video emits ~25 field/value rows). The driver flags results
	// where a transcript or official summary was actually appended.
	hasTranscript := fetchResult.HasTranscript
	if contentType != wikitypes.ContentVideo ||
		(len([]rune(fetchResult.Body)) >= minVideoRunes && hasTranscript) {
		return pendingURLWrite{}, false
	}

	item := classifyItemWithRawBody(urlStr, fetchResult.Title, fetchResult.Body)

	return pendingExtractFailureWrite(
		item, wikitypes.FailureReasonTooShort, "video content too short (likely no transcript)",
	), true
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

	classResult, classifyErr := deps.classifier.ClassifyURLInTopic(ctx, urlStr, title, content, pin)
	if classifyErr != nil {
		switch {
		case errors.Is(classifyErr, wikiclassify.ErrCatalogUnavailable):
			// The model was never called. Keep the inbox line, but record the
			// real cause so a catalog outage is not counted as AI downtime.
			return pendingAIErrorWrite(urlStr, wikitypes.FailureReasonCatalog, classifyErr.Error())
		case errors.Is(classifyErr, wikiclassify.ErrVideoTooShort):
			return pendingExtractFailureWrite(
				classifyItemWithRawBody(urlStr, title, content),
				wikitypes.FailureReasonTooShort,
				classifyErr.Error(),
			)
		default:
			return pendingAIErrorWrite(urlStr, wikitypes.FailureReasonAI, "AI classify unavailable after retries: "+classifyErr.Error())
		}
	}
	if classResult == nil {
		// Empty content is a permanent content-side failure; a nil result with
		// non-empty content means a classifier implementation returned no
		// result, which remains transient.
		if strings.TrimSpace(content) == "" {
			return pendingExtractFailureWrite(
				classifyItemWithRawBody(urlStr, title, content),
				wikitypes.FailureReasonEmpty,
				"extraction failed: empty content",
			)
		}

		return pendingAIErrorWrite(urlStr, wikitypes.FailureReasonAI, "AI classify unavailable after retries: classifier returned nil")
	}

	item := classifyItemFromResult(urlStr, title, classResult)

	if shouldWriteClassifyFailure(classResult) {
		return pendingClassifyFailureWrite(item, classifyFailureInfo(classResult))
	}

	return pendingURLWrite{URL: urlStr, Kind: pendingSummary, Item: item}
}

// pendingAIErrorWrite is the single shape for a transient classification
// failure: the URL stays in the inbox for a later retry.
func pendingAIErrorWrite(urlStr string, reason wikitypes.FailureReason, message string) pendingURLWrite {
	return pendingURLWrite{
		URL:           urlStr,
		Kind:          pendingAIError,
		FailureType:   wikitypes.FailureAI,
		FailureReason: reason,
		Error:         message,
	}
}

// classifyItemWithRawBody preserves fetched content on failure paths without
// writing it to uncat.
func classifyItemWithRawBody(urlStr, title, content string) *wikitypes.ClassifyItem {
	return &wikitypes.ClassifyItem{
		URL:         urlStr,
		Title:       title,
		ContentType: wikifetch.DetectContentType(urlStr),
		Summary:     &wikitypes.StructuredSummary{Overview: content},
	}
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
	URL           string
	Kind          pendingWriteKind
	Item          *wikitypes.ClassifyItem
	FailureType   wikitypes.FailureKind
	FailureReason wikitypes.FailureReason
	ExtraInfo     string
	Error         string
	// SkipHistory writes the artifact without appending a success-history entry.
	SkipHistory bool
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

	// A multi-item page's children carry their own content, so only single-item
	// pages archive a transcript — nothing else would link the upload.
	transcriptURL := maybeUploadTranscript(ctx, deps, urlStr, fetchResult)

	pending := classifyPending(ctx, deps, urlStr, fetchResult, wikiclassify.TopicPin{})
	attachTranscriptURL(&pending, transcriptURL)

	return []pendingURLWrite{pending}
}

// attachTranscriptURL records the uploaded transcript link on the pending
// entry so the write layer renders "Transcript: <url>".
func attachTranscriptURL(pending *pendingURLWrite, transcriptURL string) {
	if transcriptURL != "" && pending.Item != nil {
		pending.Item.TranscriptURL = transcriptURL
	}
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
		URL:           item.URL,
		Kind:          pendingClassifyFailure,
		Item:          item,
		FailureType:   wikitypes.FailureClassify,
		FailureReason: wikitypes.FailureReasonClassify,
		ExtraInfo:     extraInfo,
	}
}

func pendingExtractFailureWrite(
	item *wikitypes.ClassifyItem,
	failureReason wikitypes.FailureReason,
	extraInfo string,
) pendingURLWrite {
	return pendingURLWrite{
		URL:           item.URL,
		Kind:          pendingExtractFailure,
		Item:          item,
		FailureType:   wikitypes.FailureExtract,
		FailureReason: failureReason,
		ExtraInfo:     extraInfo,
	}
}

func newPendingFetchFailure(
	urlStr string,
	failureType wikitypes.FailureKind,
	failureReason wikitypes.FailureReason,
	extraInfo string,
) pendingURLWrite {
	return pendingURLWrite{
		URL:           urlStr,
		Kind:          pendingFetchFailure,
		FailureType:   failureType,
		FailureReason: failureReason,
		ExtraInfo:     extraInfo,
	}
}

func newPendingUnhandled(urlStr, message string) pendingURLWrite {
	return pendingURLWrite{URL: urlStr, Kind: pendingUnhandled, Error: message}
}
