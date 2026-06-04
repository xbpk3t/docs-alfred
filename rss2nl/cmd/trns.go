package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/pkg/ai"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
	"github.com/xbpk3t/docs-alfred/pkg/rss"
	"github.com/xbpk3t/docs-alfred/rss2nl/transcript"
)

const defaultTrnsSource = "podcast"

const (
	statusFound  = "found"
	statusCached = "cached"
	statusFailed = "failed"
)

type trnsFlags struct {
	cfgFile  string
	outDir   string
	language string
	limit    int
	asr      bool
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

func newTrnsCmd() *cobra.Command {
	flags := &trnsFlags{}

	cmd := &cobra.Command{
		Use:       "trns [source]",
		Short:     "Fetch transcript data for a source",
		Long:      "Fetch transcript/transcription data for a source (e.g. podcast). Uses RSS, description link, and ASR fallback chain.",
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: []string{defaultTrnsSource},
		RunE: func(cmd *cobra.Command, args []string) error {
			source := defaultTrnsSource
			if len(args) > 0 {
				source = args[0]
			}

			return runTrns(source, flags)
		},
	}

	cmd.Flags().StringVar(&flags.outDir, "out", ".cache/rss2nl/trns", "Trns cache/output directory")
	cmd.Flags().IntVar(&flags.limit, "limit", 0, "Episodes to process per feed")
	cmd.Flags().BoolVar(&flags.refresh, "refresh", false, "Ignore existing cached trns data")
	cmd.PersistentFlags().StringVar(&flags.cfgFile, "config", "rss2nl.yml", "Config file path")
	cmd.Flags().BoolVar(&flags.asr, "asr", false, "Enable ASR fallback")
	cmd.Flags().StringVar(&flags.language, "language", "", "ASR language")
	cmd.Flags().BoolVar(&flags.publish, "publish", false, "Temporary upload to Litterbox")

	checkCmd := &cobra.Command{
		Use:       "check [source]",
		Short:     "Check transcript availability",
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: []string{defaultTrnsSource},
		RunE: func(cmd *cobra.Command, args []string) error {
			source := defaultTrnsSource
			if len(args) > 0 {
				source = args[0]
			}

			return runTrnsCheck(source, flags)
		},
	}
	checkCmd.Flags().StringVar(&flags.outDir, "out", ".cache/rss2nl/trns", "Trns cache/output directory")
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
	if errMkdir := os.MkdirAll(outDir, fileutil.DirPerm); errMkdir != nil {
		return fmt.Errorf("mkdir %s: %w", outDir, errMkdir)
	}

	cache := transcript.NewCache(outDir)

	// Build pipeline: RssTranscriptProvider -> DescriptionLinkProvider -> AudioTranscriptionProvider (optional)
	pipeline := buildPipeline(flags)

	// Parse config for per-feed ASR override
	asrOverride := flags.asr
	if cfg.TrnsConfig.Asr.Enabled {
		asrOverride = true
	}
	language := flags.language
	if language == "" && cfg.TrnsConfig.Asr.Language != "" {
		language = cfg.TrnsConfig.Asr.Language
	}

	summarizer := setupSummarizer(cfg)

	uploader := setupUploader(cfg, flags)

	entries := processPodcastFeeds(cfg, outDir, flags, cache, pipeline, asrOverride, language, summarizer, uploader)

	// Write index
	indexPath := cache.IndexFilePath()
	idxData, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal index: %w", err)
	}
	_ = os.WriteFile(indexPath, idxData, fileutil.FilePermPrivate)

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
	summaryLang := cfg.TrnsConfig.Summary.Language
	if summaryLang == "" {
		summaryLang = "en"
	}

	return transcript.NewSummarizer(aiCfg, summaryLang)
}

func setupUploader(cfg *rss.Config, flags *trnsFlags) *transcript.LitterboxUploader {
	if !flags.publish && !cfg.TrnsConfig.TemporaryUpload.Enabled {
		return nil
	}
	exp := cfg.TrnsConfig.TemporaryUpload.ExpirationDuration
	if exp == "" {
		exp = "24h"
	}

	return transcript.NewLitterboxUploader(exp)
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

func buildPipeline(flags *trnsFlags) *transcript.Pipeline {
	providers := []transcript.Provider{
		transcript.NewRssTranscriptProvider(),
		transcript.NewDescriptionLinkProvider(),
	}
	if flags.asr {
		providers = append(providers, transcript.NewAudioTranscriptionProvider("pt", flags.language))
	}

	return transcript.NewPipeline(providers...)
}

func runTrnsCheck(source string, flags *trnsFlags) error {
	cfg, err := rss.NewConfig(flags.cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	limit := flags.limit
	if limit <= 0 {
		limit = 10
	}

	failedFeeds := 0
	totalEpisodes := 0

	for _, feed := range cfg.Feeds {
		for _, u := range feed.URLs {
			if u.Feed == "" {
				continue
			}

			fp := gofeed.NewParser()
			fp.Client = rss.NewHTTPClient(cfg)
			parsed, err := fp.ParseURL(u.Feed)
			if err != nil {
				slog.Warn("Feed parse failed", "feed", u.Feed, "error", err)
				failedFeeds++

				continue
			}

			rssCount, audioCount, epCount := inspectFeedItems(parsed, limit)
			totalEpisodes += epCount
			slog.Info("Feed inspection",
				"feed", parsed.Title,
				"limit", limit,
				"rss", rssCount,
				"audio", audioCount,
			)
		}
	}

	slog.Info("Trns check completed",
		"episodes", totalEpisodes,
		"failedFeeds", failedFeeds,
	)

	if flags.strict && failedFeeds > 0 {
		return fmt.Errorf("trns check: %d feeds failed", failedFeeds)
	}

	return nil
}

//nolint:nonamedreturns // named returns for readability
func inspectFeedItems(parsed *gofeed.Feed, limit int) (rssCount, audioCount, episodeCount int) {
	for i, item := range parsed.Items {
		if i >= limit {
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
	pipeline *transcript.Pipeline,
	asrOverride bool,
	language string,
	summarizer *transcript.Summarizer,
	uploader *transcript.LitterboxUploader,
) []trnsIndexEntry {
	limit := flags.limit
	if limit <= 0 {
		limit = 10
	}

	var entries []trnsIndexEntry

	for _, feed := range cfg.Feeds {
		for _, u := range feed.URLs {
			feedEntries := processFeedURL(u, outDir, limit, flags.refresh, cache, pipeline)
			entries = append(entries, feedEntries...)
		}
	}

	return entries
}

func processFeedURL(
	u rss.Feeds,
	outDir string,
	limit int,
	refresh bool,
	cache *transcript.Cache,
	pipeline *transcript.Pipeline,
) []trnsIndexEntry {
	if u.Feed == "" || !strings.HasPrefix(u.Feed, "http") {
		return nil
	}

	fp := gofeed.NewParser()
	fp.Client = &http.Client{Timeout: 30 * time.Second}

	parsed, err := fp.ParseURL(u.Feed)
	if err != nil {
		slog.Warn("Feed parse failed in process",
			"feed", u.Feed,
			"error", err,
		)

		return nil
	}

	var entries []trnsIndexEntry
	for i, item := range parsed.Items {
		if i >= limit {
			break
		}

		entry := processEpisode(item, outDir, refresh, cache, pipeline, parsed.Title, u.Feed)
		entries = append(entries, entry)
	}

	return entries
}

func processEpisode(
	item *gofeed.Item,
	outDir string,
	refresh bool,
	cache *transcript.Cache,
	pipeline *transcript.Pipeline,
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

	// Run pipeline
	ctx := context.Background()
	result, source, err := pipeline.Fetch(ctx, &epRef)
	if err != nil || result == nil || result.Content == "" {
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
			Message:      "no transcript found: " + err.Error(),
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
		Source:       source,
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
		Source:         source,
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

