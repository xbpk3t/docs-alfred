package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/internal/rss/feed"
	"github.com/xbpk3t/docs-alfred/internal/rss/transcript"
	"github.com/xbpk3t/docs-alfred/pkg/ai"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
	"github.com/xbpk3t/docs-alfred/pkg/httputil"
	"github.com/xbpk3t/docs-alfred/pkg/litter"
	"github.com/xbpk3t/docs-alfred/pkg/md"
	"github.com/xbpk3t/docs-alfred/pkg/output"
)

const (
	defaultTrnsSource         = "podcast"
	defaultTrnsSummaryBaseURL = "https://api.lucc.dev/v1"
)

const (
	statusFound  = "found"
	statusCached = "cached"
	statusFailed = "failed"
)

type trnsFlags struct {
	cfgFile  string
	outDir   string
	limit    int
	limitSet bool
	publish  bool
	refresh  bool
	strict   bool
}

type trnsIndexEntry struct {
	EpisodeTitle   string `json:"episodeTitle"`
	EpisodeURL     string `json:"episodeUrl,omitempty"`
	FeedTitle      string `json:"feedTitle"`
	FeedURL        string `json:"feedUrl"`
	Key            string `json:"key"`
	Source         string `json:"source,omitempty"`
	Status         string `json:"status"`
	TranscriptPath string `json:"transcriptPath,omitempty"`
	TranscriptURL  string `json:"transcriptUrl,omitempty"`
	Message        string `json:"message,omitempty"`
}

// NewsletterTrnsReport summarizes best-effort trns processing during send.
type NewsletterTrnsReport struct {
	Eligible       int
	Attempted      int
	Linked         int
	Failed         int
	SkippedNoMedia int
	SkippedByLimit int
}

type newsletterTrnsProcessor func(item *NewsletterItem) (string, error)

func newTrnsCmd() *cobra.Command {
	flags := &trnsFlags{}

	cmd := &cobra.Command{
		Use:       "trns [source]",
		Short:     "Fetch transcript data for a source",
		Long:      "Fetch transcript/transcription data for a source (e.g. podcast). Routes to Xiaoyuzhou API or RSS transcript tags.",
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: []string{defaultTrnsSource},
		RunE: func(cmd *cobra.Command, args []string) error {
			flags.limitSet = cmd.Flags().Changed("limit")
			source := defaultTrnsSource
			if len(args) > 0 {
				source = args[0]
			}

			return runTrns(source, flags)
		},
	}

	cmd.Flags().StringVar(&flags.outDir, "output", fileutil.CachePath("rss2nl/trns"), "Trns cache/output directory")
	cmd.Flags().IntVar(&flags.limit, "limit", 0, "Episodes to process per feed")
	cmd.Flags().BoolVar(&flags.refresh, "refresh", false, "Ignore existing cached trns data")
	cmd.PersistentFlags().StringVar(&flags.cfgFile, "config", "rss2nl.yml", "Config file path")
	cmd.Flags().BoolVar(&flags.publish, "publish", false, "Temporary upload to Litterbox")

	checkCmd := &cobra.Command{
		Use:       "check [source]",
		Short:     "Check transcript availability",
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: []string{defaultTrnsSource},
		RunE: func(cmd *cobra.Command, args []string) error {
			flags.limitSet = cmd.Flags().Changed("limit")
			source := defaultTrnsSource
			if len(args) > 0 {
				source = args[0]
			}

			return runTrnsCheck(source, flags, output.GetFormat(cmd))
		},
	}
	checkCmd.Flags().StringVar(&flags.outDir, "out", fileutil.CachePath("rss2nl/trns"), "Trns cache/output directory")
	checkCmd.Flags().IntVar(&flags.limit, "limit", 0, "Episodes to inspect per feed")
	checkCmd.Flags().BoolVar(&flags.strict, "strict", false, "Exit non-zero when any trns feed fails")

	cmd.AddCommand(checkCmd)

	return cmd
}

// -- Run pipeline --

func runTrns(source string, flags *trnsFlags) error {
	cfg, err := rss.NewConfig(flags.cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	outDir := flags.outDir
	if errMkdir := fileutil.EnsureDir(outDir); errMkdir != nil {
		return fmt.Errorf("mkdir %s: %w", outDir, errMkdir)
	}

	cache := transcript.NewCache(outDir)

	router := buildRouter()

	summarizer := setupSummarizer(cfg)

	uploader := setupUploader(cfg, flags)

	entries := processPodcastFeeds(cfg, outDir, flags, cache, router, summarizer, uploader)

	// Write index
	indexPath := cache.IndexFilePath()
	idxData, err := fileutil.MarshalJSON(entries)
	if err != nil {
		return fmt.Errorf("marshal index: %w", err)
	}
	if err := fileutil.AtomicWriteFile(indexPath, idxData, fileutil.FilePermPrivate); err != nil {
		return fmt.Errorf("write index: %w", err)
	}

	found, cached, failed := computeStats(entries)

	slog.Info("Trns completed",
		"episodes", len(entries),
		"found", found,
		"cached", cached,
		"failed", failed,
		"index", indexPath,
	)

	if flags.strict && failed > 0 {
		return fmt.Errorf("trns: %d episode(s) failed", failed)
	}

	return nil
}

func setupSummarizer(cfg *rss.Config) *transcript.Summarizer {
	if !cfg.TrnsConfig.Summary.Enabled {
		return nil
	}
	aiCfg := ai.DefaultConfig()
	if cfg.TrnsConfig.Summary.Model != "" {
		aiCfg.Model = cfg.TrnsConfig.Summary.Model
	}
	if baseURL := configuredSummaryBaseURL(cfg); baseURL != "" {
		aiCfg.BaseURL = baseURL
	}
	summaryLang := cfg.TrnsConfig.Summary.Language
	if summaryLang == "" {
		summaryLang = "en"
	}

	return transcript.NewSummarizer(aiCfg, summaryLang)
}

func configuredSummaryBaseURL(cfg *rss.Config) string {
	baseURL := strings.TrimSpace(cfg.TrnsConfig.Summary.BaseURL)
	if baseURL == "" || baseURL == defaultTrnsSummaryBaseURL {
		return ""
	}

	return baseURL
}

func setupUploader(cfg *rss.Config, flags *trnsFlags) litter.Uploader {
	if !flags.publish && !cfg.TrnsConfig.TemporaryUpload.Enabled {
		return nil
	}
	exp := cfg.TrnsConfig.TemporaryUpload.ExpirationDuration
	if exp == "" {
		exp = "24h"
	}

	return litter.NewLitterbox(exp)
}

//nolint:nonamedreturns
func computeStats(entries []trnsIndexEntry) (found, cached, failed int) {
	for ei := range entries {
		switch entries[ei].Status {
		case statusFound:
			found++
		case statusCached:
			cached++
		default:
			failed++
		}
	}

	return
}

func effectiveTrnsLimit(cfg *rss.Config, flags *trnsFlags) int {
	if flags != nil && flags.limitSet {
		return normalizeTrnsLimit(flags.limit)
	}

	return normalizeTrnsLimit(cfg.TrnsConfig.DefaultLimit)
}

func normalizeTrnsLimit(limit int) int {
	if limit <= 0 {
		return 0
	}

	return limit
}

func trnsLimitReached(processed, limit int) bool {
	return limit > 0 && processed >= limit
}

func buildRouter() *transcript.Router {
	return &transcript.Router{
		Xiaoyuzhou:    transcript.NewXiaoyuzhouProvider(""),
		RssTranscript: transcript.NewRssTranscriptProvider(),
	}
}

type trnsCheckFeedResult struct {
	Feed     string `json:"feed"`
	Title    string `json:"title"`
	Error    string `json:"error,omitempty"`
	Limit    int    `json:"limit"`
	RSS      int    `json:"rss"`
	Audio    int    `json:"audio"`
	Episodes int    `json:"episodes"`
	Failed   bool   `json:"failed,omitempty"`
}

type trnsCheckResult struct {
	Feeds         []trnsCheckFeedResult `json:"feeds"`
	TotalEpisodes int                   `json:"totalEpisodes"`
	FailedFeeds   int                   `json:"failedFeeds"`
}

func runTrnsCheck(source string, flags *trnsFlags, format string) error {
	cfg, err := rss.NewConfig(flags.cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	limit := effectiveTrnsLimit(cfg, flags)

	result := trnsCheckResult{}

	for _, feed := range cfg.RSS {
		for _, u := range feed.Feeds {
			if u.Feed == "" {
				continue
			}

			fp := gofeed.NewParser()
			fp.Client = httputil.StdHTTPClient(time.Duration(cfg.FeedConfig.Timeout) * time.Second)
			parsed, err := fp.ParseURL(u.Feed)
			if err != nil {
				slog.Warn("Feed parse failed", "feed", u.Feed, "error", err)
				result.FailedFeeds++
				result.Feeds = append(result.Feeds, trnsCheckFeedResult{
					Feed:   u.Feed,
					Failed: true,
					Error:  err.Error(),
				})

				continue
			}

			rssCount, audioCount, epCount := inspectFeedItems(parsed, limit)
			result.TotalEpisodes += epCount
			result.Feeds = append(result.Feeds, trnsCheckFeedResult{
				Feed:     u.Feed,
				Title:    parsed.Title,
				Limit:    limit,
				RSS:      rssCount,
				Audio:    audioCount,
				Episodes: epCount,
			})

			slog.Info("Feed inspection",
				"feed", parsed.Title,
				"limit", limit,
				"rss", rssCount,
				"audio", audioCount,
			)
		}
	}

	if format == output.FormatJSON {
		return output.WriteJSON(result)
	}

	slog.Info("Trns check completed",
		"episodes", result.TotalEpisodes,
		"failedFeeds", result.FailedFeeds,
	)

	if flags.strict && result.FailedFeeds > 0 {
		return fmt.Errorf("trns check: %d feeds failed", result.FailedFeeds)
	}

	return nil
}

//nolint:nonamedreturns // named returns for readability
func inspectFeedItems(parsed *gofeed.Feed, limit int) (rssCount, audioCount, episodeCount int) {
	for _, item := range parsed.Items {
		if trnsLimitReached(episodeCount, limit) {
			break
		}
		episodeCount++

		if hasTranscriptLinks(item) {
			rssCount++
		}
		if item.Enclosures != nil {
			for _, enc := range item.Enclosures {
				t := strings.ToLower(enc.Type)
				if strings.Contains(t, "audio") || strings.Contains(t, "mp3") || strings.Contains(t, "mpeg") {
					audioCount++

					break
				}
			}
		}
	}

	return
}

func processPodcastFeeds(
	cfg *rss.Config,
	outDir string,
	flags *trnsFlags,
	cache *transcript.Cache,
	provider transcript.Provider,
	summarizer *transcript.Summarizer,
	uploader litter.Uploader,
) []trnsIndexEntry {
	limit := effectiveTrnsLimit(cfg, flags)

	var entries []trnsIndexEntry

	for _, feed := range cfg.RSS {
		for _, u := range feed.Feeds {
			if !u.IsMedia {
				continue
			}
			feedEntries := processFeedURL(&u, outDir, limit, flags.refresh, cache, provider)
			entries = append(entries, feedEntries...)
		}
	}

	return entries
}

func processFeedURL(
	u *rss.Feeds,
	outDir string,
	limit int,
	refresh bool,
	cache *transcript.Cache,
	provider transcript.Provider,
) []trnsIndexEntry {
	if u.Feed == "" || !strings.HasPrefix(u.Feed, "http") {
		return nil
	}

	fp := gofeed.NewParser()
	fp.Client = httputil.StdHTTPClient(30 * time.Second)

	parsed, err := fp.ParseURL(u.Feed)
	if err != nil {
		slog.Warn("Feed parse failed in process",
			"feed", u.Feed,
			"error", err,
		)

		return nil
	}

	var entries []trnsIndexEntry
	for _, item := range parsed.Items {
		if trnsLimitReached(len(entries), limit) {
			break
		}

		entry := processEpisode(item, outDir, refresh, cache, provider, parsed.Title, u.Feed)
		entries = append(entries, entry)
	}

	return entries
}

func processEpisode(
	item *gofeed.Item,
	outDir string,
	refresh bool,
	cache *transcript.Cache,
	provider transcript.Provider,
	feedTitle, feedURL string,
) trnsIndexEntry {
	epRef := toEpisodeRef(item, feedTitle, feedURL)
	key := cache.Key(feedURL, epRef.GUID, epRef.URL, epRef.Title)

	// Check cache
	if !refresh {
		if entry, err := cache.Get(key); err == nil && entry != nil {
			return trnsIndexEntry{
				EpisodeTitle:   item.Title,
				EpisodeURL:     item.Link,
				FeedTitle:      feedTitle,
				FeedURL:        feedURL,
				Key:            key,
				Source:         entry.Source,
				Status:         statusCached,
				TranscriptPath: entry.TranscriptPath,
			}
		}
	}

	// Run provider
	ctx := context.Background()
	result, err := provider.Fetch(ctx, &epRef)
	if err != nil || result == nil || result.Content == "" {
		message := "no transcript found"
		if err != nil {
			message += ": " + err.Error()
		}
		slog.Debug("No transcript found",
			"episode", item.Title,
			"error", err,
		)

		return trnsIndexEntry{
			EpisodeTitle: item.Title,
			FeedTitle:    feedTitle,
			FeedURL:      feedURL,
			Key:          key,
			Status:       statusFailed,
			Message:      message,
		}
	}

	// Normalize content type
	contentType := result.ContentType
	if contentType == "" {
		contentType = "plaintext"
	}

	// Cache the result
	cacheEntry := &transcript.CacheEntry{
		EpisodeTitle: item.Title,
		EpisodeURL:   item.Link,
		FeedTitle:    feedTitle,
		FeedURL:      feedURL,
		Source:       result.Source,
		ContentType:  contentType,
	}

	if err := cache.Set(key, cacheEntry, result.Content); err != nil {
		slog.Warn("Failed to cache transcript", "key", key, "error", err)
	}

	return trnsIndexEntry{
		EpisodeTitle:   item.Title,
		EpisodeURL:     item.Link,
		FeedTitle:      feedTitle,
		FeedURL:        feedURL,
		Key:            key,
		Source:         result.Source,
		Status:         statusFound,
		TranscriptPath: cache.CacheFilePath(key),
	}
}

func toEpisodeRef(item *gofeed.Item, feedTitle, feedURL string) transcript.EpisodeRef {
	ref := transcript.EpisodeRef{
		Title:       item.Title,
		URL:         item.Link,
		GUID:        item.GUID,
		Description: item.Description,
		Content:     item.Content,
		FeedTitle:   feedTitle,
		FeedURL:     feedURL,
	}

	// Get enclosure URL
	if len(item.Enclosures) > 0 {
		ref.EnclosureURL = item.Enclosures[0].URL
	}

	// Extract transcript links from RSS extensions (podcast:transcript)
	for ns, extMap := range item.Extensions {
		nsLower := strings.ToLower(ns)
		if !strings.Contains(nsLower, "podcast") && !strings.Contains(nsLower, "transcript") {
			continue
		}
		for tag, exts := range extMap {
			if !strings.Contains(strings.ToLower(tag), "transcript") {
				continue
			}
			for _, e := range exts {
				link := transcript.TranscriptLink{
					URL:  e.Attrs["url"],
					Type: e.Attrs["type"],
				}
				if link.URL != "" {
					ref.TranscriptLinks = append(ref.TranscriptLinks, link)
				}
			}
		}
	}

	return ref
}

// -- Helpers --

func hasTranscriptLinks(item *gofeed.Item) bool {
	for _, ext := range item.Extensions {
		for tag := range ext {
			if strings.Contains(strings.ToLower(tag), "transcript") {
				return true
			}
		}
	}
	if item.Enclosures != nil {
		for _, enc := range item.Enclosures {
			if strings.Contains(strings.ToLower(enc.Type), "transcript") {
				return true
			}
		}
	}

	return false
}

// -- Trns page rendering --

type trnsPageView struct {
	Title        string
	FeedTitle    string
	EpisodeURL   string
	Status       string
	Summary      string
	SummaryError string
	Content      string
}

type itemTrnsContent struct {
	Content string
	Source  string
}

type cachedItemTrns struct {
	content string
	source  string
	ok      bool
}

type itemTrnsSummary struct {
	text    string
	errText string
}

func renderTrnsPage(view *trnsPageView) string {
	doc := md.NewDocument()
	doc.Add(md.NamedSection(view.Title))

	var metaPairs []md.MdPair
	metaPairs = append(metaPairs, md.MdPair{Key: "Feed", Value: view.FeedTitle})
	if view.EpisodeURL != "" {
		metaPairs = append(metaPairs, md.MdPair{Key: "Episode", Value: md.Link(view.EpisodeURL, view.EpisodeURL)})
	}
	metaPairs = append(metaPairs, md.MdPair{Key: "Status", Value: view.Status})
	doc.Add(md.Metadata(metaPairs...))

	if view.Summary != "" {
		doc.Add(md.NamedSection("AI Summary", md.Paragraph(view.Summary)))
	}
	if view.SummaryError != "" {
		doc.Add(md.Notice("AI Summary unavailable", view.SummaryError))
	}

	doc.Add(md.Paragraph(view.Content))

	page, err := doc.ToPage()
	if err != nil {
		slog.Warn("Failed to render trns page", "error", err)

		return fmt.Sprintf("<pre>%s</pre>", view.Content)
	}

	return page
}

// ProcessNewsletterTrns fetches transcripts for podcast newsletter items,
// renders HTML pages, uploads to Litterbox, sets TrnsURL on items, and returns a best-effort report.
func ProcessNewsletterTrns(items []NewsletterItem, cfg *rss.Config, outDir string) NewsletterTrnsReport {
	if !cfg.TrnsConfig.Enabled {
		return NewsletterTrnsReport{}
	}

	expiration := cfg.TrnsConfig.TemporaryUpload.ExpirationDuration
	if !cfg.TrnsConfig.TemporaryUpload.Enabled && expiration == "" {
		return NewsletterTrnsReport{}
	}

	cache := transcript.NewCache(outDir)
	router := buildRouter()

	// Pre-flight: validate xiaoyuzhou credentials before processing.
	if xzProvider, ok := router.Xiaoyuzhou.(*transcript.XiaoyuzhouProvider); ok {
		if err := xzProvider.ValidateCredentials(context.Background()); err != nil {
			slog.Warn("Xiaoyuzhou credential validation failed, skipping trns",
				"error", err,
			)

			return NewsletterTrnsReport{}
		}
	}
	summarizer := setupSummarizer(cfg)
	uploader := litter.NewLitterbox(expiration)
	processor := func(item *NewsletterItem) (string, error) {
		return processItemTrns(item, cfg, cache, router, summarizer, uploader, outDir)
	}

	return processNewsletterTrnsItems(items, cfg.TrnsConfig.DefaultLimit, processor)
}

func processNewsletterTrnsItems(items []NewsletterItem, limit int, process newsletterTrnsProcessor) NewsletterTrnsReport {
	report := NewsletterTrnsReport{}
	limit = normalizeTrnsLimit(limit)
	for i := range items {
		item := &items[i]
		if !newsletterItemHasTrnsInput(item) {
			report.SkippedNoMedia++

			continue
		}
		report.Eligible++
		if trnsLimitReached(report.Attempted, limit) {
			report.SkippedByLimit++

			continue
		}
		report.Attempted++

		trnsURL, err := process(item)
		if err != nil {
			report.Failed++
			slog.Warn("Trns for newsletter item failed", "title", item.Title, "link", item.Link, "error", err)

			continue
		}
		if trnsURL != "" {
			item.TrnsURL = trnsURL
			report.Linked++
		}
	}

	return report
}

func newsletterItemHasTrnsInput(item *NewsletterItem) bool {
	return item != nil && item.IsMedia &&
		(item.EnclosureURL != "" || len(item.PodcastTranscripts) > 0)
}

func processItemTrns(
	item *NewsletterItem,
	cfg *rss.Config,
	cache *transcript.Cache,
	provider transcript.Provider,
	summarizer *transcript.Summarizer,
	uploader litter.Uploader,
	outDir string,
) (string, error) {
	feedTitle := item.FeedTitle
	key := cache.Key(feedTitle, item.ItemHash, item.Link, item.Title)

	trns, err := getNewsletterItemTrns(item, feedTitle, key, cache, provider)
	if err != nil {
		return "", err
	}

	summary := summarizeItemTrns(summarizer, item.Title, trns.Content)
	html := renderTrnsPage(&trnsPageView{
		Title:        item.Title,
		FeedTitle:    feedTitle,
		EpisodeURL:   item.Link,
		Status:       trns.Source,
		Summary:      summary.text,
		SummaryError: summary.errText,
		Content:      trns.Content,
	})

	return uploadItemTrns(uploader, item.ItemHash, html)
}

func getNewsletterItemTrns(
	item *NewsletterItem,
	feedTitle, key string,
	cache *transcript.Cache,
	provider transcript.Provider,
) (*itemTrnsContent, error) {
	cached, err := readCachedItemTrns(cache, key)
	if err != nil {
		return nil, err
	}
	if cached.ok {
		return &itemTrnsContent{Content: cached.content, Source: cached.source}, nil
	}

	return fetchAndCacheItemTrns(item, feedTitle, key, cache, provider)
}

func fetchAndCacheItemTrns(
	item *NewsletterItem,
	feedTitle, key string,
	cache *transcript.Cache,
	provider transcript.Provider,
) (*itemTrnsContent, error) {
	epRef := transcript.EpisodeRef{
		Title:       item.Title,
		URL:         item.Link,
		Description: item.Description,
		Content:     item.Content,
		FeedTitle:   feedTitle,
	}
	if item.EnclosureURL != "" {
		epRef.EnclosureURL = item.EnclosureURL
	}
	epRef.TranscriptLinks = toTranscriptLinks(item.PodcastTranscripts)

	result, fetchErr := provider.Fetch(context.Background(), &epRef)
	if fetchErr != nil || result == nil || result.Content == "" {
		return nil, fmt.Errorf("no transcript found: %w", fetchErr)
	}

	cacheEntry := &transcript.CacheEntry{
		EpisodeTitle: item.Title,
		EpisodeURL:   item.Link,
		FeedTitle:    feedTitle,
		Source:       result.Source,
		ContentType:  result.ContentType,
	}
	if cacheErr := cache.Set(key, cacheEntry, result.Content); cacheErr != nil {
		slog.Warn("Failed to cache trns for newsletter", "key", key, "error", cacheErr)
	}

	return &itemTrnsContent{
		Content: result.Content,
		Source:  result.Source,
	}, nil
}

func toTranscriptLinks(refs []PodcastTranscriptRef) []transcript.TranscriptLink {
	links := make([]transcript.TranscriptLink, 0, len(refs))
	for _, ref := range refs {
		if ref.URL == "" {
			continue
		}
		links = append(links, transcript.TranscriptLink{
			URL:  ref.URL,
			Type: ref.Type,
		})
	}

	return links
}

func summarizeItemTrns(summarizer *transcript.Summarizer, title, content string) itemTrnsSummary {
	if summarizer == nil {
		return itemTrnsSummary{}
	}

	result, err := summarizer.GenerateSummary(context.Background(), title, content)
	if err != nil {
		return itemTrnsSummary{errText: err.Error()}
	}
	if result == nil {
		return itemTrnsSummary{}
	}

	return itemTrnsSummary{text: result.Summary}
}

func uploadItemTrns(uploader litter.Uploader, itemHash, html string) (string, error) {
	if uploader == nil {
		return "", nil
	}

	filename := fmt.Sprintf("trns-%s.html", itemHash[:min(16, len(itemHash))])
	result, err := uploader.Upload(context.Background(), filename, html)
	if err != nil {
		return "", fmt.Errorf("upload trns page: %w", err)
	}

	return result.URL, nil
}

func readCachedItemTrns(cache *transcript.Cache, key string) (cachedItemTrns, error) {
	entry, _ := cache.Get(key)
	if entry == nil {
		return cachedItemTrns{}, nil
	}

	cached, readErr := cache.ReadTranscript(key)
	if readErr != nil {
		return cachedItemTrns{}, fmt.Errorf("cache read failed: %w", readErr)
	}
	if cached == "" {
		return cachedItemTrns{}, errors.New("cache read failed: empty transcript")
	}

	return cachedItemTrns{content: cached, source: entry.Source, ok: true}, nil
}
