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
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/adrg/frontmatter"
	carbon "github.com/dromara/carbon/v2"
	"github.com/goccy/go-yaml"
	"github.com/xbpk3t/docs-alfred/internal/gh/index"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
	"github.com/xbpk3t/docs-alfred/pkg/urlutil"
)

// SummaryFrontmatter is the YAML frontmatter for summary.md files.
type SummaryFrontmatter struct {
	Title     string `yaml:"title"`
	Date      string `yaml:"date"`
	Source    string `yaml:"source"`
	Type      string `yaml:"type"`
	BatchID   string `yaml:"batch_id"`
	TotalURLs int    `yaml:"total_urls"`
	Succeeded int    `yaml:"succeeded"`
	Failed    int    `yaml:"failed"`
}

// WriteOptions contains options for writing wiki entries.
type WriteOptions struct {
	R2Store         DigestStore
	ValidTopicPaths map[string]bool
	WikiRoot        string
	BatchID         string
	DryRun          bool
	IsAggr          bool
}

var pathLocks = struct {
	locks map[string]*sync.Mutex
	mu    sync.Mutex
}{locks: make(map[string]*sync.Mutex)}

// DigestStore is the R2 surface the write layer needs when digest artifacts go
// remote. *litter.R2 implements it via a small adapter in the ingest package;
// tests fake it.
type DigestStore interface {
	// Upload stores content under the given object name and returns its public
	// URL.
	Upload(ctx context.Context, key, content string) (string, error)
	// GetObject reads an object's content; a missing object reads as empty.
	GetObject(ctx context.Context, key string) ([]byte, error)
}

func lockPath(path string) func() {
	key, err := filepath.Abs(path)
	if err != nil {
		key = filepath.Clean(path)
	}

	pathLocks.mu.Lock()
	mu := pathLocks.locks[key]
	if mu == nil {
		mu = &sync.Mutex{}
		pathLocks.locks[key] = mu
	}
	pathLocks.mu.Unlock()

	mu.Lock()

	return func() {
		mu.Unlock()
		// Clean up the map entry if no other goroutine is waiting.
		pathLocks.mu.Lock()
		if pathLocks.locks[key] == mu {
			delete(pathLocks.locks, key)
		}
		pathLocks.mu.Unlock()
	}
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

// WriteSummary writes a structured summary.md entry with YAML frontmatter.
// Follows TS writer.ts: appendToSummaryFile.
func WriteSummary(item *types.ClassifyItem, opts *WriteOptions) (string, error) {
	if item.TopicPath == "" {
		// Empty TopicPath would write to wiki root — route to classify rejection.
		_, err := WriteFailureEntry(item, types.FailureClassify, "empty topic path", opts)

		return "", err
	}

	// If ValidTopicPaths is set, validate topicPath before writing.
	// Try fuzzy leaf match first (AI often gets parent wrong or strips *** from leaf).
	if routed, outPath, err := routeInvalidTopicPath(item, opts); routed {
		return outPath, err
	}

	today := carbon.Now().ToDateString()
	batchID := opts.BatchID
	if batchID == "" {
		batchID = "wiki-" + today
	}

	// Aggregate mode (wiki.isAggr) collects the day's entries into one
	// <date>-summary.md at the wiki root, grouped by topic path. The default
	// per-topic mode writes <topic>/summary.md, grouped by date.
	summaryPath := filepath.Join(resolveTopicDir(item, opts), types.SummaryFile)
	sectionHeading := "## " + today
	fmTitle := filepath.Base(item.TopicPath)
	if opts.IsAggr {
		summaryPath = filepath.Join(opts.WikiRoot, today+"-summary.md")
		sectionHeading = "## " + item.TopicPath
		fmTitle = batchID
	}

	if opts.DryRun {
		if opts.R2Store != nil {
			slog.Info("[DRY RUN] Would write summary to R2",
				"key", digestObjectKey(batchID, item.TopicPath, item.URL), "url", item.URL, "topic", item.TopicPath)

			return digestObjectKey(batchID, item.TopicPath, item.URL), nil
		}
		slog.Info("[DRY RUN] Would write summary", "path", summaryPath)

		return summaryPath, nil
	}

	entry := buildEntry(item)

	// R2 mode: every post is its own object at digest/<batch>/<topic>/<hash>.md,
	// so the wiki tree stays free of digest artifacts. batch = one digest run.
	if opts.R2Store != nil {
		key := digestObjectKey(batchID, item.TopicPath, item.URL)
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

	unlock := lockPath(summaryPath)
	defer unlock()

	fm, existingBody := loadFrontmatter(summaryPath, item, today, batchID)

	fm.TotalURLs++
	fm.Succeeded++
	fm.BatchID = batchID
	fm.Date = today
	fm.Title = fmTitle
	// A summary.md aggregates many entries, so its frontmatter type reflects its
	// role (digest) — NOT the type of the individual item being digested. The
	// per-entry type (research/blog/etc.) lives in the body entry metadata. Forcing
	// digest here also self-heals files that were written with a per-entry type.
	// Binding is shared with the checker via wikitypes.ArtifactTypeFor so the two
	// sides can't drift.
	fm.Type = string(types.ArtifactTypeFor(types.SummaryFile))

	newBody := appendEntryBody(existingBody, sectionHeading, entry)

	content := renderContent(fm, newBody)

	if err := fileutil.AtomicWriteFile(summaryPath, []byte(content), fileutil.FilePermPrivate); err != nil {
		return "", fmt.Errorf("write summary: %w", err)
	}

	slog.Info("Summary written", "path", summaryPath, "topic", item.TopicPath)

	// Log success to JSONL.
	if _, err := LogSuccessEntry(item, summaryPath, opts); err != nil {
		slog.Warn("Failed to log success entry", "url", item.URL, "error", err)
	}

	return summaryPath, nil
}

func resolveTopicDir(item *types.ClassifyItem, opts *WriteOptions) string {
	return filepath.Join(opts.WikiRoot, item.TopicPath)
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

	// Build codeblock metadata section. URL is always the first field; topic
	// follows it so an entry stays self-locating when it is cut out of an
	// aggregate <date>-summary.md.
	metaBlock := fmt.Sprintf("URL: %s\n", item.URL)
	if item.TopicPath != "" {
		metaBlock += fmt.Sprintf("topic: %s\n", item.TopicPath)
	}
	if item.MetadataBlock != "" {
		metaBlock += item.MetadataBlock
	} else {
		// Fallback: just Type if no structured metadata.
		metaBlock += fmt.Sprintf("Type: %s", item.Type)
	}
	if item.TranscriptURL != "" {
		metaBlock += fmt.Sprintf("\nTranscript: %s", item.TranscriptURL)
	}

	return fmt.Sprintf("### %s\n\n```markdown\n%s\n```\n\n%s\n", title, metaBlock, classify.RenderStructuredSummary(item.Summary))
}

//nolint:nonamedreturns
func loadFrontmatter(summaryPath string, item *types.ClassifyItem, today, batchID string) (fm *SummaryFrontmatter, body string) {
	fm = &SummaryFrontmatter{
		Title:     filepath.Base(item.TopicPath),
		Date:      today,
		Source:    "rss2nl-wiki",
		BatchID:   batchID,
		TotalURLs: 0,
		Succeeded: 0,
		Failed:    0,
	}
	if data, err := os.ReadFile(summaryPath); err == nil {
		if parsed := parseSummaryFrontmatter(string(data)); parsed != nil {
			fm = parsed.fm
			body = parsed.body

			return
		}
		body = string(data)

		return
	}

	return
}

func appendEntryBody(existingBody, sectionHeading, entry string) string {
	if merged, ok := appendEntryToExistingSection(existingBody, sectionHeading, entry); ok {
		return merged
	}
	insertPos := strings.Index(existingBody, "\n## ")
	if insertPos >= 0 {
		return fmt.Sprintf("%s\n%s\n\n%s\n%s", existingBody[:insertPos], sectionHeading, entry, existingBody[insertPos:])
	}

	return fmt.Sprintf("%s%s\n\n%s\n", existingBody, sectionHeading, entry)
}

// sectionBodyStart returns the offset just past the "## <heading>" line whose
// text matches heading exactly, or -1 when no such section exists.
//
// The match must be line-exact, not a substring: topic paths nest (## AI/agent
// is a prefix of ## AI/agent/agent-fwk), so a substring search would append
// entries into the wrong section in aggregate mode.
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

func renderContent(fm *SummaryFrontmatter, body string) string {
	fmYAML, err := yaml.Marshal(fm)
	if err != nil {
		return "---\n---\n\n" + body
	}

	return fmt.Sprintf("---\n%s---\n\n%s", string(fmYAML), body)
}

// WriteFailureEntry logs a structured failure entry to the appropriate JSONL log
// under <wikiRoot>/digest-<type>-error.jsonl. Each log file collects one type:
//   - fetch:     network-level errors (DNS, timeout) with no opencli fallback
//   - resolve:   HTTP-level errors (403 anti-bot) where opencli also failed
//   - extract:   URL fetched but usable article/media content could not be extracted
//   - classify:  content fetched but AI couldn't assign a topic
//
// The legacy MD file format is replaced by JSONL for machine-parseable diagnostics.
func WriteFailureEntry(item *types.ClassifyItem, failureType types.FailureKind, extraInfo string, opts *WriteOptions) (string, error) {
	if opts == nil {
		return "", errors.New("write options required for failure entry")
	}

	entry := types.DigestEntry{
		URL:            itemURL(item),
		Stage:          digestStageForFailure(failureType),
		Status:         types.DigestFailure,
		FailureKind:    string(failureType),
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
	if opts == nil {
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

// WriteManualReviewEntry appends a human-readable entry to wiki/uncat.md
// for items with NeedsManualReview=true that have good AI-generated summaries.
func WriteManualReviewEntry(item *types.ClassifyItem, opts *WriteOptions) (string, error) {
	if opts == nil || item == nil {
		return "", nil
	}

	if item.RouteReason == "" {
		item.RouteReason = types.RouteReasonNeedsManualReview
	}

	path := filepath.Join(opts.WikiRoot, "uncat.md")
	if opts.DryRun {
		slog.Info("[DRY RUN] Would append to uncat.md",
			"path", path, "url", item.URL, "reason", item.RouteReason, "suggestedTopic", item.SuggestedTopic)

		return path, nil
	}

	today := carbon.Now().ToDateString()
	dateHeading := "## " + today

	title, metaBlock := buildReviewEntryMeta(item)
	entry := fmt.Sprintf("\n### %s\n\n```markdown\n%s\n```\n\n%s\n", title, metaBlock, classify.RenderStructuredSummary(item.Summary))

	// R2 mode: the uncat queue lives at uncat.md in the bucket (read-merge-write,
	// same as the digest logs) so nothing lands in the local wiki tree.
	if opts.R2Store != nil {
		key := "uncat.md"
		existing, err := opts.R2Store.GetObject(context.Background(), key)
		if err != nil {
			return "", fmt.Errorf("r2 write uncat.md: read existing: %w", err)
		}
		merged := mergeDateSection(string(existing), dateHeading, entry)
		url, err := opts.R2Store.Upload(context.Background(), key, merged)
		if err != nil {
			return "", fmt.Errorf("r2 write uncat.md: %w", err)
		}
		slog.Info("Manual review entry appended to R2",
			"key", key, "url", item.URL, "reason", item.RouteReason, "suggestedTopic", item.SuggestedTopic)
		if _, err := LogSuccessEntry(item, url, opts); err != nil {
			slog.Warn("Failed to log success entry for manual review", "url", item.URL, "error", err)
		}

		return url, nil
	}

	unlock := lockPath(path)
	defer unlock()

	if err := appendToDateSectionAndWrite(path, dateHeading, entry); err != nil {
		return "", fmt.Errorf("write uncat.md: %w", err)
	}

	slog.Info("Manual review entry appended",
		"path", path, "url", item.URL, "reason", item.RouteReason, "suggestedTopic", item.SuggestedTopic)

	// Log success to JSONL so uncat.md entries count toward the success rate.
	if _, err := LogSuccessEntry(item, path, opts); err != nil {
		slog.Warn("Failed to log success entry for manual review", "url", item.URL, "error", err)
	}

	return path, nil
}

// buildReviewEntryMeta builds the title and metadata block for a manual review entry.
//
//nolint:nonamedreturns
func buildReviewEntryMeta(item *types.ClassifyItem) (title, metaBlock string) {
	title = item.Title
	if title == "" {
		title = item.URL
	}

	// Build codeblock metadata section matching buildEntry format.
	var b strings.Builder
	fmt.Fprintf(&b, "URL: %s\n", item.URL)
	if item.MetadataBlock != "" {
		b.WriteString(item.MetadataBlock)
		if !strings.HasSuffix(item.MetadataBlock, "\n") {
			b.WriteByte('\n')
		}
	} else if item.Type != "" {
		fmt.Fprintf(&b, "Type: %s\n", item.Type)
	}
	if item.RouteReason != "" {
		fmt.Fprintf(&b, "reason: %s\n", item.RouteReason)
	}
	if item.SuggestedTopic != "" {
		fmt.Fprintf(&b, "suggestedTopic: %s\n", item.SuggestedTopic)
	}
	if item.TranscriptURL != "" {
		fmt.Fprintf(&b, "Transcript: %s\n", item.TranscriptURL)
	}
	metaBlock = strings.TrimRight(b.String(), "\n")

	return
}

// mergeDateSection appends an entry to the matching date section of existing
// content, falling back to appending a fresh section when none exists. Shared by
// the local (file) and R2 (object) uncat write paths.
func mergeDateSection(existing, dateHeading, entry string) string {
	if merged, ok := appendEntryToExistingSection(existing, dateHeading, entry); ok {
		return merged
	}
	if strings.TrimSpace(existing) == "" {
		return fmt.Sprintf("%s\n\n%s\n", dateHeading, entry)
	}

	return fmt.Sprintf("%s\n%s\n\n%s\n", strings.TrimRight(existing, "\n"), dateHeading, entry)
}

// appendToDateSectionAndWrite appends an entry to the appropriate date section
// in the uncat.md file and writes the result.
func appendToDateSectionAndWrite(path, dateHeading, entry string) error {
	var existing strings.Builder
	if data, err := os.ReadFile(path); err == nil {
		existing.Write(data)
	}

	return fileutil.AtomicWriteFile(path, []byte(mergeDateSection(existing.String(), dateHeading, entry)), fileutil.FilePermPrivate)
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

// parseSummaryFrontmatterResult holds the result of parsing.
type parseResult struct {
	fm   *SummaryFrontmatter
	body string
}

// parseSummaryFrontmatter parses YAML frontmatter from a summary.md file.
func parseSummaryFrontmatter(raw string) *parseResult {
	if !strings.HasPrefix(raw, "---") {
		return nil
	}

	var fm SummaryFrontmatter
	body, err := frontmatter.Parse(strings.NewReader(raw), &fm)
	if err != nil || len(body) == len(raw) {
		return nil
	}

	return &parseResult{fm: &fm, body: string(body)}
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
// that share the same line.
func FlushInbox(filePath string, handledURLsByLine map[int][]string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	var remaining []string
	for i, line := range lines {
		handled := normalizedURLSet(handledURLsByLine[i])
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
