package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/xbpk3t/docs-alfred/service/wiki"
)

func runURLs(cfg *Config, urls []string) error {
	wikiRoot := resolveWikiRoot(cfg)

	if _, err := os.Stat(wikiRoot); os.IsNotExist(err) {
		return fmt.Errorf("wiki root not found: %s", wikiRoot)
	}

	aiCfg := newAIConfig(cfg)
	classifier := wiki.NewClassifier(aiCfg, wikiRoot, cfg.Wiki.GhTopicsURL)
	fetcher := wiki.NewFetcher()
	ctx := context.Background()

	slog.Info("Processing URLs for wiki", "count", len(urls), "wikiRoot", wikiRoot)

	for _, urlStr := range urls {
		processClassifyURL(ctx, cfg, wikiRoot, classifier, fetcher, urlStr)
	}

	return nil
}

// processClassifyURL processes a single URL for classification (not inbox mode).
func processClassifyURL(
	ctx context.Context, cfg *Config, wikiRoot string,
	classifier *wiki.Classifier, fetcher *wiki.Fetcher, urlStr string,
) {
	slog.Info("Processing URL", "url", urlStr)

	contentType := wiki.DetectContentType(urlStr)
	result := fetcher.FetchContent(ctx, urlStr, contentType)

	if result.Error != "" {
		slog.Error("Failed to fetch URL", "url", urlStr, "error", result.Error)

		return
	}

	title := result.Title
	if title == "" {
		title = urlStr
	}

	classResult := classifier.ClassifyURL(ctx, urlStr, title, result.Body)
	if classResult == nil {
		slog.Warn("Classification unavailable", "url", urlStr)

		return
	}

	item := &wiki.ClassifyItem{
		URL:         urlStr,
		Title:       title,
		ContentType: classResult.ContentType,
		TopicPath:   classResult.TopicPath,
		Type:        classResult.WikiType,
		Summary:     classResult.Summary,
	}

	opts := &wiki.WriteOptions{
		WikiRoot: wikiRoot,
	}

	if classResult.TopicPath == "none" || classResult.TopicPath == "inbox" {
		extraInfo := "AI could not classify the content into any topic.\nSummary: " + classResult.Summary
		if _, err := wiki.WriteFailureEntry(item, wiki.FailureClassify, extraInfo, opts); err != nil {
			slog.Error("Failed to write failure entry", "url", urlStr, "error", err)

			return
		}
		slog.Info("URL written to group-failed (unclassifiable)", "url", urlStr)

		return
	}

	if _, err := wiki.WriteSummary(item, opts); err != nil {
		slog.Error("Failed to write summary", "url", urlStr, "error", err)
	}
}
