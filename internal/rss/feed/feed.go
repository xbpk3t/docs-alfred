package rss

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	retry "github.com/avast/retry-go/v4"
	carbon "github.com/dromara/carbon/v2"
	"github.com/mmcdole/gofeed"
	"github.com/xbpk3t/docs-alfred/pkg/httputil"
	"golang.org/x/sync/errgroup"
)

// DefaultUserAgent is a realistic browser UA to avoid bot detection (e.g. Substack 403).
// TODO: consider using a random-UA library (e.g. corpix/uarand) if rotation becomes necessary.
const DefaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) " +
	"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

// createFeedParser 创建Feed解析器.
func createFeedParser(cfg *Config) *gofeed.Parser {
	fp := gofeed.NewParser()
	fp.UserAgent = DefaultUserAgent
	fp.Client = httputil.StdHTTPClient(time.Duration(cfg.FeedConfig.Timeout) * time.Second)

	return fp
}

// getMaxAttempts 获取最大重试次数.
func getMaxAttempts(cfg *Config) uint {
	if cfg.FeedConfig.MaxTries < 0 {
		return 0
	}

	return uint(cfg.FeedConfig.MaxTries)
}

// FetchURLWithRetry 重试获取URL内容.
func FetchURLWithRetry(ctx context.Context, rawURL string, cfg *Config) (*gofeed.Feed, *FeedError) {
	return fetchURLWithRetry(ctx, rawURL, cfg, nil)
}

func fetchURLWithRetry(
	ctx context.Context,
	rawURL string,
	cfg *Config,
	hosts *hostFetchController,
) (*gofeed.Feed, *FeedError) {
	if feedErr := validateURL(rawURL); feedErr != nil {
		slog.Error("Invalid URL", slog.String(LogKeyURL, rawURL), slog.Any(LogKeyError, feedErr))

		return nil, feedErr
	}

	host := hostnameOf(rawURL)
	if feedErr := hosts.circuitFeedError(rawURL, host); feedErr != nil {
		return nil, feedErr
	}

	fp := createFeedParser(cfg)
	var attempts uint
	var lastError error
	var feed *gofeed.Feed

	err := retry.Do(
		func() error {
			parsed, attemptErr := fetchOnce(ctx, rawURL, host, fp, hosts)
			if attemptErr != nil {
				lastError = attemptErr
				if !retry.IsRecoverable(attemptErr) {
					if unwrapped := errors.Unwrap(attemptErr); unwrapped != nil {
						lastError = unwrapped
					}
				}

				return attemptErr
			}
			feed = parsed

			return nil
		},
		retry.Context(ctx),
		retry.Attempts(getMaxAttempts(cfg)),
		retry.Delay(DefaultRetryDelay),
		retry.DelayType(retry.BackOffDelay),
		retry.LastErrorOnly(true),
		retry.OnRetry(func(n uint, err error) {
			attempts = n
			lastError = err
			slog.Info("Retry Parse FeedConfig",
				slog.String(LogKeyURL, rawURL),
				slog.Uint64(LogKeyAttempts, uint64(attempts)),
				slog.Any(LogKeyError, err))
		}),
	)
	if err != nil {
		return nil, fetchFailed(rawURL, attempts, lastError, err)
	}

	return feed, nil
}

func fetchOnce(
	ctx context.Context,
	rawURL, host string,
	fp *gofeed.Parser,
	hosts *hostFetchController,
) (*gofeed.Feed, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := hosts.unrecoverableIfCircuitOpen(host); err != nil {
		return nil, err
	}

	parsedFeed, err := fp.ParseURLWithContext(rawURL, ctx)
	if err != nil {
		return nil, handleParseError(rawURL, host, hosts, err)
	}

	return parsedFeed, nil
}

func handleParseError(
	rawURL, host string,
	hosts *hostFetchController,
	err error,
) error {
	slog.Error("Parse FeedConfig Error",
		slog.String(LogKeyURL, rawURL),
		slog.Any(LogKeyError, err))

	if hosts.tripIfNeeded(host, rawURL, err) {
		return retry.Unrecoverable(err)
	}
	if !isFastRetryableFetchError(err) {
		return retry.Unrecoverable(err)
	}

	return err
}

func fetchFailed(rawURL string, attempts uint, lastError, retryErr error) *FeedError {
	if lastError == nil {
		lastError = retryErr
	}
	if !retry.IsRecoverable(retryErr) {
		if unwrapped := errors.Unwrap(retryErr); unwrapped != nil {
			lastError = unwrapped
		}
	}
	slog.Error("Parse FeedConfig Error after retries",
		slog.String(LogKeyURL, rawURL),
		slog.Uint64(LogKeyAttempts, uint64(attempts)),
		slog.Any(LogKeyError, lastError))

	return classifyFetchError(rawURL, lastError)
}

func hostCircuitFeedError(rawURL, reason string) *FeedError {
	return &FeedError{
		URL:       rawURL,
		Message:   fmt.Sprintf("skipped: host circuit open (%s)", reason),
		Kind:      FeedFailureKindHTTPStatus,
		Transient: true,
		Err:       errors.New(reason),
	}
}

func classifyFetchError(rawURL string, err error) *FeedError {
	if err == nil {
		return &FeedError{URL: rawURL, Message: "unknown fetch error", Kind: FeedFailureKindUnknown}
	}
	info := inspectFetchError(err)

	return &FeedError{
		URL:       rawURL,
		Message:   err.Error(),
		Err:       err,
		Kind:      info.Kind,
		Transient: info.Transient,
	}
}

// validateURL 验证URL.
func validateURL(rawURL string) *FeedError {
	if rawURL == "" {
		return &FeedError{
			URL:     rawURL,
			Message: "empty URL",
			Kind:    FeedFailureKindInvalidURL,
		}
	}

	return nil
}

type fetchURLResult struct {
	feed *gofeed.Feed
	err  *FeedError
}

// FetchResult holds a fetched feed with its original URL.
type FetchResult struct {
	Feed *gofeed.Feed
	Err  *FeedError
	URL  string
}

// FetchURLs 批量获取URLs，返回成功和失败的 feeds.
func FetchURLs(ctx context.Context, urls []string, cfg *Config) ([]*gofeed.Feed, []*FeedError) {
	allFeeds, _, failedFeeds := FetchURLsWithMeta(ctx, urls, cfg)

	return allFeeds, failedFeeds
}

// FetchURLsWithMeta 批量获取URLs，同时返回每个请求的结果元信息.
func FetchURLsWithMeta(ctx context.Context, urls []string, cfg *Config) ([]*gofeed.Feed, []FetchResult, []*FeedError) {
	results := make([]fetchURLResult, len(urls))
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(feedFetchConcurrency(cfg))
	hosts := newHostFetchController(cfg.FeedConfig)

	for i, rawURL := range urls {
		g.Go(func() error {
			results[i] = fetchOneURL(ctx, rawURL, cfg, hosts)

			return nil
		})
	}
	_ = g.Wait()

	return collectFetchResults(urls, results)
}

func fetchOneURL(
	ctx context.Context,
	rawURL string,
	cfg *Config,
	hosts *hostFetchController,
) fetchURLResult {
	host := hostnameOf(rawURL)
	if err := hosts.acquire(ctx, host); err != nil {
		return fetchURLResult{err: &FeedError{
			URL:       rawURL,
			Message:   err.Error(),
			Err:       err,
			Kind:      FeedFailureKindContextCancelled,
			Transient: true,
		}}
	}
	defer hosts.release(host)

	feed, feedErr := fetchURLWithRetry(ctx, rawURL, cfg, hosts)

	return fetchURLResult{feed: feed, err: feedErr}
}

func collectFetchResults(urls []string, results []fetchURLResult) ([]*gofeed.Feed, []FetchResult, []*FeedError) {
	allFeeds := make([]*gofeed.Feed, 0, len(urls))
	meta := make([]FetchResult, 0, len(urls))
	failedFeeds := make([]*FeedError, 0)
	for i, result := range results {
		meta = append(meta, FetchResult{URL: urls[i], Feed: result.feed, Err: result.err})
		if result.err != nil {
			slog.Error("Failed to fetch feed",
				slog.String(LogKeyURL, result.err.URL),
				slog.String(LogKeyError, result.err.Message))
			failedFeeds = append(failedFeeds, result.err)
		}
		if result.feed != nil {
			allFeeds = append(allFeeds, result.feed)
		}
	}

	return allFeeds, meta, failedFeeds
}

// FilterFeedsWithTimeRange 根据时间范围过滤feeds.
func FilterFeedsWithTimeRange(created, endDate time.Time, schedule string) bool {
	scheduleTimeRanges := GetScheduleTimeRanges()
	timeRange, exists := scheduleTimeRanges[schedule]
	if !exists {
		slog.Error("Invalid schedule type",
			slog.String("schedule", schedule))

		return false
	}

	createdTime := carbon.CreateFromStdTime(created)

	return createdTime.Gte(carbon.CreateFromStdTime(endDate).SubHours(timeRange).StartOfDay())
}
