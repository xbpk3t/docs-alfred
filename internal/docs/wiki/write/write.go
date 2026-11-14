package write

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/xbpk3t/docs-alfred/internal/docs/wiki/classify"
	"github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
	"log/slog"
	"os"
	"slices"
	"strings"
	"sync"

	carbon "github.com/dromara/carbon/v2"
	"github.com/goccy/go-yaml"
	"github.com/xbpk3t/docs-alfred/internal/gh/index"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
	"github.com/xbpk3t/docs-alfred/pkg/urlutil"
)

// WriteOptions contains options for writing wiki entries. Digest artifacts are
// remote-only: every write goes to R2, so R2Store is required.
type WriteOptions struct {
	R2Store         DigestStore
	ValidTopicPaths map[string]bool
	BatchID         string
	DryRun          bool
	// SkipHistory writes the artifact but does not append a success entry to
	// digest-success.jsonl. Multi-item synthesis entries use it while the line
	// still has retryable child failures, so a later run re-expands the thread.
	SkipHistory bool
}

// R2 has no append/CAS primitive, so read-merge-write must be serialized at
// least inside this process. r2UncatMu and r2LogMu are separate because an
// uncat write also logs a success entry. Overlapping digest processes still
// need an object-store CAS/lease.
var (
	r2UncatMu sync.Mutex
	r2LogMu   sync.Mutex
)

// DigestStore is the R2 surface the write layer needs; digest artifacts are
// remote-only. *litter.R2 implements it via a small adapter in the ingest
// package; tests fake it.
type DigestStore interface {
	// Upload stores content under the given object name and returns its public
	// URL.
	Upload(ctx context.Context, key, content string) (string, error)
	// GetObject reads an object's content; a missing object reads as empty.
	GetObject(ctx context.Context, key string) ([]byte, error)
}

// routeInvalidTopicPath fuzzy-resolves or reroutes invalid topic paths to uncat.
// Returns routed=true when the caller should return immediately.
func routeInvalidTopicPath(item *types.ClassifyItem, opts *WriteOptions) (bool, string, error) {
	if opts == nil || opts.ValidTopicPaths == nil || opts.ValidTopicPaths[item.TopicPath] {
		return false, "", nil
	}
	if resolved, ok := classify.ResolveTopicPathAmong(item.TopicPath, opts.ValidTopicPaths); ok {
		slog.Info("Resolved topic path via fuzzy match",
			"from", item.TopicPath, "to", resolved, "url", item.URL)
		if item.SuggestedTopic == "" {
			item.SuggestedTopic = item.TopicPath
		}
		item.TopicPath = resolved

		return false, "", nil
	}

	slog.Warn("Topic path not in gh.yml, routing to uncat.md",
		"topic", item.TopicPath, "url", item.URL, "reason", types.RouteReasonInvalidTopicPath)
	reviewItem := *item
	reviewItem.NeedsManualReview = true
	reviewItem.Type = types.TypeInbox
	if reviewItem.SuggestedTopic == "" {
		reviewItem.SuggestedTopic = item.TopicPath
	}
	reviewItem.RouteReason = types.RouteReasonInvalidTopicPath
	reviewItem.TopicPath = ""
	out, err := WriteManualReviewEntry(&reviewItem, opts)

	return true, out, err
}

// WriteSummary writes one digest post to R2 as a standalone markdown file with
// YAML frontmatter.
func WriteSummary(item *types.ClassifyItem, opts *WriteOptions) (string, error) {
	if opts == nil || opts.R2Store == nil {
		return "", errors.New("write summary: r2 store required")
	}
	if item.TopicPath == "" {
		// No topic means no digest location — route to classify rejection.
		_, err := WriteFailureEntry(item, types.FailureClassify, types.FailureReasonClassify, "empty topic path", opts)

		return "", err
	}

	// If ValidTopicPaths is set, validate topicPath before writing.
	// Try fuzzy leaf match first (AI often gets parent wrong or strips *** from leaf).
	if routed, outPath, err := routeInvalidTopicPath(item, opts); routed {
		return outPath, err
	}

	batchID := opts.BatchID
	if batchID == "" {
		batchID = "wiki-" + carbon.Now().ToDateString()
	}

	key := digestObjectKey(batchID, item.TopicPath, item.URL)
	if opts.DryRun {
		slog.Info("[DRY RUN] Would write summary to R2", "key", key, "url", item.URL, "topic", item.TopicPath)

		return key, nil
	}

	return writeSummaryR2(item, buildEntry(item), key, opts)
}

// writeSummaryR2 uploads the entry as its own object at
// digest/<batch>/<topic>/<hash>.md and logs the success entry.
func writeSummaryR2(item *types.ClassifyItem, entry, key string, opts *WriteOptions) (string, error) {
	url, err := opts.R2Store.Upload(context.Background(), key, entry)
	if err != nil {
		return "", fmt.Errorf("r2 write summary: %w", err)
	}
	slog.Info("Summary written to R2", "key", key, "url", url, "topic", item.TopicPath)
	if _, err := LogSuccessEntry(item, url, opts); err != nil {
		slog.Warn("Failed to log success entry", "url", item.URL, "error", err)
	}

	return url, nil
}

// digestObjectKey builds the R2 object key for one post's digest:
// digest/<batch>/<topicPath>/<hash>.md. batch is the digest run id
// (wiki-<date>), topicPath is folder/type/topic, and hash is a short sha256 of
// the URL so a post's object is stable and collision-resistant. Each topic
// segment is slugified so CJK/space-heavy topics still produce keys that the
// R2 path validator accepts ([A-Za-z0-9._-] segments).
func digestObjectKey(batch, topicPath, urlStr string) string {
	batch = strings.Trim(batch, "/")
	if batch == "" {
		batch = "unbatched"
	}
	topicPath = strings.Trim(topicPath, "/")
	if topicPath == "" {
		topicPath = "uncat"
	}
	segs := strings.Split(topicPath, "/")
	for i, seg := range segs {
		segs[i] = slugifySegment(seg)
	}
	sum := sha256.Sum256([]byte(urlStr))

	return fmt.Sprintf("digest/%s/%s/%s.md", batch, strings.Join(segs, "/"), hex.EncodeToString(sum[:8]))
}

// slugifySegment maps a topic segment to the R2-safe charset [A-Za-z0-9._-]:
// runs of non-alphanumeric runes become a single "-", and a leading
// non-alphanumeric (the charset requires an alphanumeric first char) is
// replaced by "x". Empty stays empty (handled by the caller via "uncat").
func slugifySegment(seg string) string {
	var b strings.Builder
	dashPending := false
	for _, r := range seg {
		switch {
		case isKeyChar(r):
			if dashPending && b.Len() > 0 {
				b.WriteByte('-')
			}
			dashPending = false
			b.WriteRune(r)
		case r == '.' || r == '_' || r == '-':
			b.WriteRune(r)
		default:
			dashPending = true
		}
	}

	return prefixKeySegment(b.String())
}

// isKeyChar reports whether r is an R2 key alphanumeric.
func isKeyChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

// prefixKeySegment ensures a segment starts with an alphanumeric, prepending
// "x" when it does not (so ".." and "..." stay R2-valid). An empty result
// (the whole segment was non-alphanumeric, e.g. all-CJK) also becomes "x".
func prefixKeySegment(seg string) string {
	if seg != "" && isKeyChar(rune(seg[0])) {
		return seg
	}

	return "x" + seg
}

// LoadValidTopicPaths loads valid topic paths from the remote gh.yml URL when
// ghTopicsURL is set; otherwise falls back to data/gh beside wikiRoot.
// Returns a set of folder/type/topic paths that are valid for writing (formal kinds only).
func LoadValidTopicPaths(wikiRoot, ghTopicsURL string) map[string]bool {
	remote, err := ghindex.RemoteTopicCatalog(
		ghindex.LocalGHConfig{WikiRoot: wikiRoot},
		ghTopicsURL,
	)
	if err != nil {
		slog.Warn("Topic catalog unavailable for write validation", "error", err)

		return nil
	}

	valid := make(map[string]bool, len(remote))
	for _, c := range remote {
		if classify.ValidateTopicPathDepth(c.Path) {
			valid[c.Path] = true
		}
	}

	return valid
}

func buildEntry(item *types.ClassifyItem) string {
	title := item.Title
	if title == "" {
		title = item.URL
	}

	return buildEntryFrontmatter(item, title) + "\n\n" + classify.RenderStructuredSummary(item.Summary) + "\n"
}

// buildEntryFrontmatter renders the original codeblock metadata as YAML
// frontmatter: title first, then URL/topic, then the AI metadata lines, then
// the transcript link.
func buildEntryFrontmatter(item *types.ClassifyItem, title string) string {
	items := yaml.MapSlice{
		{Key: "title", Value: title},
		{Key: "URL", Value: item.URL},
	}
	if item.TopicPath != "" {
		items = append(items, yaml.MapItem{Key: "topic", Value: item.TopicPath})
	}

	return renderFrontmatter(appendEntryMetaItems(items, item))
}

// appendEntryMetaItems appends the frontmatter fields shared by digest posts
// and uncat entries: classifier metadata (or Type) and the transcript link.
func appendEntryMetaItems(items yaml.MapSlice, item *types.ClassifyItem) yaml.MapSlice {
	if item.MetadataBlock != "" {
		items = append(items, metadataItems(item.MetadataBlock)...)
	} else if item.Type != "" {
		items = append(items, yaml.MapItem{Key: "Type", Value: string(item.Type)})
	}
	if item.TranscriptURL != "" {
		items = append(items, yaml.MapItem{Key: "Transcript", Value: item.TranscriptURL})
	}

	return items
}

// metadataItems parses the classifier's "key: value" metadata lines into
// ordered YAML items. Values are marshaled as YAML scalars, so a value that
// needs quoting (a colon, a leading special character) stays valid.
func metadataItems(block string) yaml.MapSlice {
	var items yaml.MapSlice
	for _, line := range strings.Split(block, "\n") {
		key, value, ok := strings.Cut(line, ":")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			continue
		}
		items = append(items, yaml.MapItem{Key: key, Value: strings.TrimSpace(value)})
	}

	return items
}

// renderFrontmatter renders ordered YAML items as a standard --- block.
func renderFrontmatter(items yaml.MapSlice) string {
	fmYAML, err := yaml.Marshal(items)
	if err != nil {
		// Never lose the post body over a metadata marshal failure.
		slog.Warn("marshal entry frontmatter", "error", err)

		return "---\n---"
	}

	return "---\n" + string(fmYAML) + "---"
}

// sectionBodyStart returns the offset just past the "## <heading>" line whose
// text matches heading exactly, or -1 when no such section exists.
//
// The match must be line-exact, not a substring: a heading that merely starts
// with the same text (or the same text inside an entry body) must not be
// mistaken for the section.
func sectionBodyStart(body, heading string) int {
	if i := strings.Index(body, "\n"+heading+"\n"); i >= 0 {
		return i + len(heading) + 2
	}
	if strings.HasPrefix(body, heading+"\n") {
		return len(heading) + 1
	}
	// Heading as the final line without a trailing newline (emptied section).
	if body == heading || strings.HasSuffix(body, "\n"+heading) {
		return len(body)
	}

	return -1
}

// appendEntryToExistingSection inserts entry at the end of the section whose
// "## <sectionHeading>" line exists in existingBody. ok is false when no such
// section exists, in which case existingBody is returned unchanged.
func appendEntryToExistingSection(existingBody, sectionHeading, entry string) (string, bool) {
	sectionStart := sectionBodyStart(existingBody, sectionHeading)
	if sectionStart < 0 {
		return existingBody, false
	}
	nextHeadingRel := strings.Index(existingBody[sectionStart:], "\n## ")
	insertPos := len(existingBody)
	if nextHeadingRel >= 0 {
		insertPos = sectionStart + nextHeadingRel
	}

	prefix := strings.TrimRight(existingBody[:insertPos], "\n")
	suffix := existingBody[insertPos:]
	if strings.TrimSpace(suffix) == "" {
		return fmt.Sprintf("%s\n\n%s\n", prefix, entry), true
	}

	return fmt.Sprintf("%s\n\n%s\n%s", prefix, entry, suffix), true
}

// WriteFailureEntry logs a structured failure entry to the appropriate JSONL log
// under logs/digest-<type>-error.jsonl in the R2 bucket. Each log file collects one type:
//   - fetch:     network-level errors (DNS, timeout) with no opencli fallback
//   - resolve:   HTTP-level errors (403 anti-bot) where opencli also failed
//   - extract:   URL fetched but usable article/media content could not be extracted
//   - classify:  content fetched but AI couldn't assign a topic
//
// The legacy MD file format is replaced by JSONL for machine-parseable diagnostics.
func WriteFailureEntry(
	item *types.ClassifyItem,
	failureType types.FailureKind,
	failureReason types.FailureReason,
	extraInfo string,
	opts *WriteOptions,
) (string, error) {
	if opts == nil {
		return "", errors.New("write options required for failure entry")
	}

	entry := types.DigestEntry{
		URL:            itemURL(item),
		Stage:          digestStageForFailure(failureType),
		Status:         types.DigestFailure,
		FailureKind:    string(failureType),
		FailureReason:  failureReason,
		Error:          extraInfo,
		TopicPath:      item.TopicPath,
		SuggestedTopic: item.SuggestedTopic,
		Reason:         item.RouteReason,
	}
	if entry.Reason == "" && failureType == types.FailureAI {
		entry.Reason = types.RouteReasonAIUnavailable
	}

	path, err := LogDigestEntry(&entry, opts)
	if err != nil {
		return "", fmt.Errorf("log %s failure for %s: %w", failureType, itemURL(item), err)
	}

	slog.Info("Failure entry logged", "path", path, "type", failureType, "url", item.URL)

	return path, nil
}

// LogSuccessEntry logs a successful pipeline outcome to digest-success.jsonl.
func LogSuccessEntry(item *types.ClassifyItem, outputPath string, opts *WriteOptions) (string, error) {
	if opts == nil || opts.SkipHistory {
		return "", nil
	}

	entry := types.DigestEntry{
		URL:            itemURL(item),
		Stage:          types.StageWrite,
		Status:         types.DigestSuccess,
		TopicPath:      item.TopicPath,
		SuggestedTopic: item.SuggestedTopic,
		Reason:         item.RouteReason,
		OutputPath:     outputPath,
	}

	return LogDigestEntry(&entry, opts)
}

// manualReviewKey is the uncat.md object name in the R2 bucket.
const manualReviewKey = "uncat.md"

// WriteManualReviewEntry appends a human-readable entry to the remote uncat.md
// queue for items with NeedsManualReview=true that have good AI summaries.
func WriteManualReviewEntry(item *types.ClassifyItem, opts *WriteOptions) (string, error) {
	if opts == nil || item == nil {
		return "", nil
	}
	if opts.R2Store == nil {
		return "", errors.New("write manual review: r2 store required")
	}

	if item.RouteReason == "" {
		item.RouteReason = types.RouteReasonNeedsManualReview
	}

	if opts.DryRun {
		slog.Info("[DRY RUN] Would append to uncat.md",
			"key", manualReviewKey, "url", item.URL, "reason", item.RouteReason, "suggestedTopic", item.SuggestedTopic)

		return manualReviewKey, nil
	}

	title, metaBlock := buildReviewEntryMeta(item)
	entry := fmt.Sprintf("\n### %s\n\n%s\n\n%s\n", title, metaBlock, classify.RenderStructuredSummary(item.Summary))

	// R2 has no append primitive: serialize the read-merge-write for the queue.
	r2UncatMu.Lock()
	defer r2UncatMu.Unlock()

	existing, err := opts.R2Store.GetObject(context.Background(), manualReviewKey)
	if err != nil {
		return "", fmt.Errorf("r2 write uncat.md: read existing: %w", err)
	}
	if manualReviewContainsURL(string(existing), item.URL) {
		slog.Info("Manual review entry already exists in R2; skipping duplicate",
			"key", manualReviewKey, "url", item.URL)

		return manualReviewKey, nil
	}
	dateHeading := "## " + carbon.Now().ToDateString()
	url, err := opts.R2Store.Upload(context.Background(), manualReviewKey, mergeDateSection(string(existing), dateHeading, entry))
	if err != nil {
		return "", fmt.Errorf("r2 write uncat.md: %w", err)
	}
	slog.Info("Manual review entry appended to R2",
		"key", manualReviewKey, "url", item.URL, "reason", item.RouteReason, "suggestedTopic", item.SuggestedTopic)
	if _, err := LogSuccessEntry(item, url, opts); err != nil {
		slog.Warn("Failed to log success entry for manual review", "url", item.URL, "error", err)
	}

	return url, nil
}

// buildReviewEntryMeta builds the title and frontmatter block for a manual
// review entry. The H3 stays because uncat.md aggregates many entries and the
// heading is the block separator prune uses.
//
//nolint:nonamedreturns
func buildReviewEntryMeta(item *types.ClassifyItem) (title, metaBlock string) {
	title = item.Title
	if title == "" {
		title = item.URL
	}

	items := appendEntryMetaItems(yaml.MapSlice{
		{Key: "title", Value: title},
		{Key: "URL", Value: item.URL},
	}, item)
	if item.RouteReason != "" {
		items = append(items, yaml.MapItem{Key: "reason", Value: item.RouteReason})
	}
	if item.SuggestedTopic != "" {
		items = append(items, yaml.MapItem{Key: "suggestedTopic", Value: item.SuggestedTopic})
	}

	return title, renderFrontmatter(items)
}

// mergeDateSection appends an entry to the matching date section of existing
// content, falling back to appending a fresh section when none exists.
func mergeDateSection(existing, dateHeading, entry string) string {
	if merged, ok := appendEntryToExistingSection(existing, dateHeading, entry); ok {
		return merged
	}
	if strings.TrimSpace(existing) == "" {
		return fmt.Sprintf("%s\n\n%s\n", dateHeading, entry)
	}

	return fmt.Sprintf("%s\n%s\n\n%s\n", strings.TrimRight(existing, "\n"), dateHeading, entry)
}

func itemURL(item *types.ClassifyItem) string {
	if item == nil {
		return ""
	}

	return item.URL
}

func digestStageForFailure(failureType types.FailureKind) types.DigestStage {
	switch failureType {
	case types.FailureFetch, types.FailureResolve:
		return types.StageFetch
	case types.FailureExtract:
		return types.StageExtract
	case types.FailureClassify:
		return types.StageClassify
	default:
		return types.StageClassify
	}
}

// ParseInbox parses inbox.md and returns a list of URL entries.
func ParseInbox(filePath string) ([]InboxEntry, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	var entries []InboxEntry
	seen := make(map[string]bool)

	for i, line := range lines {
		for _, u := range extractInboxLineURLs(line) {
			entries = appendInboxEntry(entries, seen, u, i)
		}
	}

	return entries, nil
}

func extractInboxLineURLs(line string) []string {
	refs := extractInboxLineURLRefs(line)
	urls := make([]string, 0, len(refs))
	for _, ref := range refs {
		urls = append(urls, ref.URL)
	}

	return urls
}

func extractInboxLineURLRefs(line string) []urlutil.URLRef {
	return urlutil.ExtractURLRefs(line, urlutil.ExtractOptions{
		BareURLs:    true,
		HTTPOnly:    true,
		Normalize:   true,
		Deduplicate: false,
	})
}

func appendInboxEntry(entries []InboxEntry, seen map[string]bool, raw string, lineIndex int) []InboxEntry {
	cleaned := cleanInboxURL(raw)
	if cleaned == "" {
		return entries
	}
	key := urlutil.Normalize(cleaned)
	if seen[key] {
		return entries
	}
	seen[key] = true

	return append(entries, InboxEntry{URL: cleaned, LineIndex: lineIndex})
}

// cleanInboxURL strips host-scoped tracking params (utm_*, spm_id_from, ...)
// via the ClearURLs engine, then keeps only absolute HTTP(S) URLs. Fetch,
// digest logs and the wiki `URL:` line all carry this cleaned form; content
// params (?v=, ?bvid=, ...) are guarded and survive.
func cleanInboxURL(raw string) string {
	// StripTracking is total: a stripped "" can only come from raw == "",
	// which CleanHTTPURL handles identically.
	return urlutil.CleanHTTPURL(urlutil.StripTracking(raw))
}

// InboxEntry represents a URL found in inbox.md.
type InboxEntry struct {
	URL       string `json:"url"`
	LineIndex int    `json:"lineIndex"`
}

// FlushInbox removes handled URLs from inbox.md without dropping unhandled URLs
// that share the same line. The line-index snapshot can shift if the file is
// edited while a digest is running, so matching is by normalized URL across the
// whole file; a handled URL must never survive, and an unhandled one must never
// be dropped because its line moved.
func FlushInbox(filePath string, handledURLsByLine map[int][]string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	handled := normalizedURLSet(flattenHandledURLs(handledURLsByLine))
	var remaining []string
	for _, line := range lines {
		if len(handled) == 0 {
			remaining = append(remaining, line)

			continue
		}

		flushedLine := flushInboxLine(line, handled)
		if strings.TrimSpace(flushedLine) != "" {
			remaining = append(remaining, flushedLine)
		}
	}

	var cleaned []string
	for _, l := range remaining {
		if strings.TrimSpace(l) != "" || len(cleaned) > 0 {
			cleaned = append(cleaned, l)
		}
	}

	if len(cleaned) == 0 {
		return fileutil.AtomicWriteFile(filePath, []byte{}, fileutil.FilePermPrivate)
	}

	return fileutil.AtomicWriteFile(filePath, []byte(strings.Join(cleaned, "\n")), fileutil.FilePermPrivate) // #nosec G703
}

func flattenHandledURLs(byLine map[int][]string) []string {
	var urls []string
	for _, lineURLs := range byLine {
		urls = append(urls, lineURLs...)
	}

	return urls
}

func normalizedURLSet(urls []string) map[string]bool {
	return urlutil.NormalizeSet(urls)
}

func flushInboxLine(line string, handled map[string]bool) string {
	refs := extractInboxLineURLRefs(line)
	if len(refs) == 0 {
		return line
	}
	var unhandled int
	flushed := false
	for _, v := range slices.Backward(refs) {
		ref := v
		// Match on the tracking-insensitive key: inbox lines carry the raw URL
		// while handled URLs come from parsed entries, which are stripped.
		if !handled[urlutil.NormalizeForDedup(ref.URL)] {
			unhandled++

			continue
		}
		line = line[:ref.Start] + line[ref.End:]
		flushed = true
	}
	if !flushed {
		return line
	}
	if unhandled == 0 {
		return ""
	}

	return cleanFlushedInboxLine(line)
}

func cleanFlushedInboxLine(line string) string {
	line = strings.TrimSpace(line)
	for strings.Contains(line, "  ") {
		line = strings.ReplaceAll(line, "  ", " ")
	}
	// Fix punctuation artifacts from URL removal.
	if strings.Contains(line, "- ,") || strings.Contains(line, "- .") ||
		strings.Contains(line, "- ;") || strings.Contains(line, "- :") {
		slog.Debug("cleaning punctuation artifacts from flushed inbox line", "original", line)
	}
	line = strings.ReplaceAll(line, "- ,", "-")
	line = strings.ReplaceAll(line, "- .", "-")
	line = strings.ReplaceAll(line, "- ;", "-")
	line = strings.ReplaceAll(line, "- :", "-")

	return strings.TrimSpace(line)
}

// manualReviewContainsURL reports whether an uncat.md body already has an
// entry for rawURL. Entries carry the URL as a `URL:` line in their
// frontmatter (older entries used a fenced metadata block); compare normalized
// forms so tracking variants collapse.
func manualReviewContainsURL(content, rawURL string) bool {
	target := urlutil.NormalizeForDedup(rawURL)
	if target == "" {
		return false
	}
	for _, line := range strings.Split(content, "\n") {
		if key, ok := entryURLFromLine(line); ok && key == target {
			return true
		}
	}

	return false
}

// entryURLFromLine parses the `URL:` line of an uncat.md entry and returns its
// tracking-insensitive dedup key. The value goes through YAML unmarshaling
// because the encoder quotes values that need it (a URL containing '#' gets
// double quotes); a plain trim would keep the quotes and break dedup/prune.
func entryURLFromLine(line string) (string, bool) {
	line = strings.TrimSpace(line)
	raw, ok := strings.CutPrefix(line, "URL:")
	if !ok {
		return "", false
	}

	var urlStr string
	if err := yaml.Unmarshal([]byte(strings.TrimSpace(raw)), &urlStr); err != nil {
		return "", false
	}

	return urlutil.NormalizeForDedup(urlStr), true
}
