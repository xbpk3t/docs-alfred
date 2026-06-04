package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/xbpk3t/docs-alfred/service/wiki"
)

func runURLs(cfg *Config, urls []string) error {
	wikiRoot := resolveWikiRoot(cfg)

	if _, err := os.Stat(wikiRoot); os.IsNotExist(err) {
		return fmt.Errorf("wiki root not found: %s", wikiRoot)
	}

	aiCfg := newAIConfig(cfg)
	classifier := wiki.NewClassifier(aiCfg, wikiRoot, cfg.Wiki.GhTopicsPath)
	fetcher := wiki.NewFetcher()
	ctx := context.Background()

	slog.Info("Processing URLs for wiki", "count", len(urls), "wikiRoot", wikiRoot)

	for _, urlStr := range urls {
		processClassifyURL(ctx, cfg, wikiRoot, classifier, fetcher, urlStr)
	}

	return nil
}

func processClassifyURL(
	ctx context.Context, cfg *Config, wikiRoot string,
	classifier *wiki.Classifier, fetcher *wiki.Fetcher, urlStr string,
) {
	slog.Info("Processing URL", "url", urlStr)

	contentType := wiki.DetectContentType(strings.ToLower(urlStr))
	result := fetcher.FetchContent(ctx, urlStr, contentType)

	if result.Error != "" {
		slog.Warn("Content fetch failed, skipping URL", "url", urlStr, "error", result.Error)

		return
	}

	title := result.Title
	if title == "" {
		title = urlStr
	}

	classResult := classifier.ClassifyURL(ctx, urlStr, title, result.Body)
	if classResult == nil {
		slog.Warn("Classification unavailable for URL", "url", urlStr)

		return
	}

	slog.Info("Classified",
		"url", urlStr,
		"topic", classResult.TopicPath,
		"type", classResult.WikiType,
		"contentType", classResult.ContentType,
	)

	item := &wiki.ClassifyItem{
		URL:         urlStr,
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
		path, err := wiki.WritePending(item, opts)
		if err != nil {
			slog.Warn("Failed to write pending", "url", urlStr, "error", err)

			return
		}
		slog.Info("Written to pending", "path", path)
	} else {
		path, err := wiki.WriteSummary(item, opts)
		if err != nil {
			slog.Warn("Failed to write summary", "url", urlStr, "error", err)

			return
		}
		slog.Info("Written to summary", "path", path, "topic", classResult.TopicPath)
	}
}
