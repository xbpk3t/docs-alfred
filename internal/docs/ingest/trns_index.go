package wikiingest

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"

	"golang.org/x/sync/errgroup"

	wikiclassify "github.com/xbpk3t/docs-alfred/internal/docs/wiki/classify"
	wikitypes "github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
)

// trnsIndexPrefix is the key prefix rss2nl publishes index batches under
// (index-<timestamp>.json objects in the R2 bucket, one per run).
const trnsIndexPrefix = "index-"

// trnsIndexEntry maps one podcast episode to its uploaded transcript page.
// Written by rss2nl send; consumed by the digest so episodes are enqueued
// without manual inbox edits.
type trnsIndexEntry struct {
	URL           string `json:"url"`
	TranscriptURL string `json:"transcriptUrl"`
	Title         string `json:"title,omitempty"`
	FeedTitle     string `json:"feedTitle,omitempty"`
}

// runTrnsIndexEntries reads every index batch from R2 and digests new episodes.
// The entry URL stays the episode URL; the transcript page is only the content
// source, so dedup and the written entry both key on the episode.
func runTrnsIndexEntries(
	ctx context.Context,
	deps *dependencies,
	wikiRoot string,
	inboxCfg inboxConfig,
	dryRun bool,
) []URLResult {
	if deps.r2 == nil {
		return nil
	}
	entries, err := loadTrnsIndex(ctx, deps.r2)
	if err != nil {
		slog.Warn("wiki digest: trns index unavailable", "error", err)

		return nil
	}
	if len(entries) == 0 {
		return nil
	}

	pending := make([]pendingURLWrite, len(entries))
	var mu sync.Mutex

	g, groupCtx := errgroup.WithContext(ctx)
	g.SetLimit(inboxCfg.concurrency)

	for i, entry := range entries {
		g.Go(func() error {
			prepared := prepareTrnsIndexEntry(groupCtx, deps, entry, inboxCfg)

			mu.Lock()
			pending[i] = prepared
			mu.Unlock()

			return nil
		})
	}
	_ = g.Wait()

	var results []URLResult
	for i := range pending {
		result := writePendingURL(deps, wikiRoot, &pending[i], dryRun)
		// Not an inbox line — a negative index keeps it out of the flush set.
		result.LineIndex = -1
		results = append(results, result)
	}

	return results
}

// loadTrnsIndex lists and merges every index batch. Batches are append-only;
// digest history dedups already-processed episodes, so re-reading old batches
// is harmless.
func loadTrnsIndex(ctx context.Context, store mediaStore) ([]trnsIndexEntry, error) {
	keys, err := store.ListKeys(ctx, trnsIndexPrefix)
	if err != nil {
		return nil, err
	}
	var entries []trnsIndexEntry
	for _, key := range keys {
		data, err := store.GetObject(ctx, key)
		if err != nil {
			slog.Warn("wiki digest: read trns index failed", "key", key, "error", err)

			continue
		}
		var batch []trnsIndexEntry
		if err := json.Unmarshal(data, &batch); err != nil {
			slog.Warn("wiki digest: bad trns index batch", "key", key, "error", err)

			continue
		}
		entries = append(entries, batch...)
	}

	return entries, nil
}

// prepareTrnsIndexEntry digests one indexed episode: content comes from the
// uploaded transcript page, identity stays the episode URL.
func prepareTrnsIndexEntry(
	ctx context.Context,
	deps *dependencies,
	entry trnsIndexEntry,
	inboxCfg inboxConfig,
) pendingURLWrite {
	if entry.URL == "" || entry.TranscriptURL == "" {
		return pendingURLWrite{URL: entry.URL, Kind: pendingSkip}
	}
	if deps.history != nil && deps.history.claimOrSeen(entry.URL) {
		slog.Info("wiki digest: skip already digested trns episode", "url", entry.URL)

		return pendingURLWrite{URL: entry.URL, Kind: pendingSkip}
	}

	urlCtx, cancel := context.WithTimeout(ctx, inboxCfg.perURLTimeout)
	defer cancel()

	slog.Info("Processing indexed trns episode", "url", entry.URL, "transcript", entry.TranscriptURL)

	// The transcript page is a plain web page; fetch it through the standard
	// driver so the HTML is converted the same way as any other URL.
	fetchResult := deps.fetcher.FetchContent(urlCtx, entry.TranscriptURL, wikitypes.ContentText)
	if pending, failed := fetchFailurePending(entry.URL, fetchResult); failed {
		return pending
	}
	if fetchResult.Title == "" {
		fetchResult.Title = entry.Title
	}

	pending := classifyPending(urlCtx, deps, entry.URL, fetchResult, wikiclassify.TopicPin{})
	if pending.Item != nil {
		pending.Item.TranscriptURL = entry.TranscriptURL
	}

	return pending
}
