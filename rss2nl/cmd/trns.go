package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
	"github.com/xbpk3t/docs-alfred/pkg/rss"
)

// Package-level gofeed parser and HTTP client reused across requests.
var trnsParser = gofeed.NewParser()
var trnsHTTPClient = &http.Client{Timeout: 60 * time.Second}

const defaultTrnsSource = "podcast"

const (
	statusFound  = "found"
	statusCached = "cached"
)

type trnsFlags struct {
	cfgFile  string
	asr      *bool
	publish  *bool
	outDir   string
	language string
	limit    int
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
		Long:      "Fetch transcript/transcription data for a source (e.g. podcast).",
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
	flags.asr = cmd.Flags().Bool("asr", false, "Enable ASR fallback")
	cmd.Flags().StringVar(&flags.language, "language", "", "ASR language")
	flags.publish = cmd.Flags().Bool("publish", false, "Temporary upload")

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

func runTrns(source string, flags *trnsFlags) error {
	cfg, err := rss.NewConfig(flags.cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	outDir := flags.outDir
	if err2 := os.MkdirAll(outDir, fileutil.DirPerm); err2 != nil {
		return fmt.Errorf("mkdir %s: %w", outDir, err2)
	}

	entries := processPodcastFeeds(cfg, outDir, flags)

	indexPath := filepath.Join(outDir, "index.json")
	idxData, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal index: %w", err)
	}
	_ = os.WriteFile(indexPath, idxData, fileutil.FilePermPrivate)

	found, cached, notFound := 0, 0, 0
	for ei := range entries {
		e := entries[ei]
		switch e.Status {
		case statusFound:
			found++
		case statusCached:
			cached++
		default:
			notFound++
		}
	}

	slog.Info("Trns completed", "episodes", len(entries), "found", found, "cached", cached, "notFound", notFound, "index", indexPath)

	if flags.strict && len(entries) == 0 {
		return errors.New("trns: no episodes processed")
	}

	return nil
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

			fp := trnsParser
			fp.Client = trnsHTTPClient

			parsed, err := fp.ParseURL(u.Feed)
			if err != nil {
				slog.Warn("Feed parse failed", "feed", u.Feed, "error", err)
				failedFeeds++

				continue
			}

			rssCount, audioCount, epCount := inspectFeedItems(parsed, limit)
			totalEpisodes += epCount
			slog.Info("Feed inspection", "feed", parsed.Title, "limit", limit, "rss", rssCount, "audio", audioCount)
		}
	}

	slog.Info("Trns check completed", "episodes", totalEpisodes, "failedFeeds", failedFeeds)

	if flags.strict && failedFeeds > 0 {
		return fmt.Errorf("trns check: %d feeds failed", failedFeeds)
	}

	return nil
}

//nolint:nonamedreturns // named returns preferred for readability in counting function
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

func processPodcastFeeds(cfg *rss.Config, outDir string, flags *trnsFlags) []trnsIndexEntry {
	limit := flags.limit
	if limit <= 0 {
		limit = 10
	}

	var entries []trnsIndexEntry

	for _, feed := range cfg.Feeds {
		for _, u := range feed.URLs {
			feedEntries := processFeedURL(u, outDir, limit, flags.refresh)
			entries = append(entries, feedEntries...)
		}
	}

	return entries
}

// processFeedURL parses a single feed URL and returns index entries for its episodes.
func processFeedURL(u rss.Feeds, outDir string, limit int, refresh bool) []trnsIndexEntry {
	if u.Feed == "" || !strings.HasPrefix(u.Feed, "http") {
		return nil
	}

	fp := trnsParser
	fp.Client = trnsHTTPClient

	parsed, err := fp.ParseURL(u.Feed)
	if err != nil {
		slog.Warn("Feed parse failed in process", "feed", u.Feed, "error", err)

		return nil
	}

	var entries []trnsIndexEntry
	for i, item := range parsed.Items {
		if i >= limit {
			break
		}

		entries = append(entries, processEpisodeItem(item, outDir, refresh, parsed.Title, u.Feed))
	}

	return entries
}

// processEpisodeItem handles a single episode item, returning a cached, found, or not_found entry.
func processEpisodeItem(item *gofeed.Item, outDir string, refresh bool, feedTitle, feedURL string) trnsIndexEntry {
	key := episodeCacheKey(item)
	cacheFile := filepath.Join(outDir, key+".txt")

	if !refresh {
		if data, err := os.ReadFile(cacheFile); err == nil && len(data) > 0 {
			return trnsIndexEntry{
				EpisodeTitle:   item.Title,
				EpisodeURL:     item.Link,
				FeedTitle:      feedTitle,
				FeedURL:        feedURL,
				Key:            key,
				Source:         "cache",
				Status:         "cached",
				TranscriptPath: cacheFile,
			}
		}
	}

	content := extractTranscriptContent(item)
	if content != "" {
		_ = os.WriteFile(cacheFile, []byte(content), fileutil.FilePermPrivate)

		return trnsIndexEntry{
			EpisodeTitle:   item.Title,
			EpisodeURL:     item.Link,
			FeedTitle:      feedTitle,
			FeedURL:        feedURL,
			Key:            key,
			Source:         "rss-transcript",
			Status:         "found",
			TranscriptPath: cacheFile,
		}
	}

	return trnsIndexEntry{
		EpisodeTitle: item.Title,
		FeedTitle:    feedTitle,
		FeedURL:      feedURL,
		Key:          key,
		Status:       "not_found",
		Message:      "no transcript found in RSS",
	}
}

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

func extractTranscriptContent(item *gofeed.Item) string {
	for _, ext := range item.Extensions {
		for tag, values := range ext {
			if !strings.Contains(strings.ToLower(tag), "transcript") {
				continue
			}
			for _, v := range values {
				if v.Value != "" {
					return v.Value
				}
				if u, ok := v.Attrs["url"]; ok && u != "" {
					// Optionally fetch the transcript from URL
					_ = u

					return "Transcript URL: " + u
				}
			}
		}
	}
	if item.Description != "" && strings.Contains(strings.ToLower(item.Description), "transcript") {
		return item.Description
	}

	return ""
}

func episodeCacheKey(item *gofeed.Item) string {
	base := item.GUID
	if base == "" {
		base = item.Link
	}
	if base == "" {
		base = item.Title
	}
	base = strings.ReplaceAll(base, "/", "_")
	base = strings.ReplaceAll(base, ":", "_")
	base = strings.ReplaceAll(base, "?", "_")
	base = strings.ReplaceAll(base, "&", "_")
	base = strings.ReplaceAll(base, "=", "_")
	if len(base) > 120 {
		base = base[:120]
	}

	return strings.Trim(base, "_")
}
