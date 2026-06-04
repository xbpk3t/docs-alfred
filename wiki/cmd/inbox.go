package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/xbpk3t/docs-alfred/service/wiki"
)

func runInbox(cfg *Config) error {
	wikiRoot := resolveWikiRoot(cfg)

	if _, err := os.Stat(wikiRoot); os.IsNotExist(err) {
		return fmt.Errorf("wiki root not found: %s", wikiRoot)
	}

	inboxPath := filepath.Join(wikiRoot, "inbox.md")
	if _, err := os.Stat(inboxPath); os.IsNotExist(err) {
		return fmt.Errorf("inbox file not found: %s", inboxPath)
	}

	aiCfg := newAIConfig(cfg)
	classifier := wiki.NewClassifier(aiCfg, wikiRoot, cfg.Wiki.GhTopicsPath)
	fetcher := wiki.NewFetcher()
	ctx := context.Background()

	slog.Info("Processing wiki inbox", "path", inboxPath)

	entries, err := wiki.ParseInbox(inboxPath)
	if err != nil {
		return fmt.Errorf("parse inbox: %w", err)
	}

	if len(entries) == 0 {
		slog.Info("No URLs found in inbox")

		return nil
	}

	slog.Info("Found URLs in inbox", "count", len(entries))

	processed := make(map[int]bool)

	for i, entry := range entries {
		if processInboxEntry(ctx, cfg, wikiRoot, classifier, fetcher, &entry, processed, i) {
			continue
		}
	}

	if len(processed) > 0 {
		if err := wiki.FlushInbox(inboxPath, processed); err != nil {
			return fmt.Errorf("flush inbox: %w", err)
		}
		slog.Info("Flushed processed lines from inbox", "count", len(processed))
	}

	return nil
}

func processInboxEntry(
	ctx context.Context, cfg *Config, wikiRoot string,
	classifier *wiki.Classifier, fetcher *wiki.Fetcher,
	entry *wiki.InboxEntry, processed map[int]bool, index int,
) bool {
	slog.Info("Processing inbox URL", "index", index+1, "url", entry.URL)

	contentType := wiki.DetectContentType(entry.URL)
	result := fetcher.FetchContent(ctx, entry.URL, contentType)

	if result.Error != "" {
		slog.Warn("Content fetch failed, keeping in inbox", "url", entry.URL, "error", result.Error)

		return true
	}

	title := result.Title
	if title == "" {
		title = entry.URL
	}

	classResult := classifier.ClassifyURL(ctx, entry.URL, title, result.Body)
	if classResult == nil {
		slog.Warn("Classification failed, keeping in inbox", "url", entry.URL)

		return true
	}

	item := &wiki.ClassifyItem{
		URL:         entry.URL,
		Title:       title,
		ContentType: classResult.ContentType,
		TopicPath:   classResult.TopicPath,
		Type:        classResult.WikiType,
		Summary:     classResult.Summary,
	}

	opts := &wiki.WriteOptions{
		WikiRoot:    wikiRoot,
		PendingPath: cfg.Wiki.PendingPath,
	}

	if classResult.TopicPath == "none" || classResult.TopicPath == "inbox" || classResult.WikiType == wiki.TypeInbox {
		if _, err := wiki.WritePending(item, opts); err != nil {
			slog.Warn("Failed to write pending", "url", entry.URL, "error", err)

			return true
		}
	} else {
		if _, err := wiki.WriteSummary(item, opts); err != nil {
			slog.Warn("Failed to write summary", "url", entry.URL, "error", err)

			return true
		}
	}

	processed[entry.LineIndex] = true

	return false
}
