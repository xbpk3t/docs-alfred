package rss

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	retry "github.com/avast/retry-go/v4"
	carbon "github.com/dromara/carbon/v2"
	"github.com/mmcdole/gofeed"
	"golang.org/x/sync/errgroup"
)

// NewHTTPClient creates an HTTP client with timeout from config.
func NewHTTPClient(cfg *Config) *http.Client {
	return &http.Client{
		Timeout: time.Duration(cfg.FeedConfig.Timeout) * time.Second,
	}
}

// createFeedParser 创建Feed解析器.
func createFeedParser(cfg *Config) *gofeed.Parser {
	fp := gofeed.NewParser()
	fp.Client = &http.Client{
		Timeout: time.Duration(cfg.FeedConfig.Timeout) * time.Second,
	}

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
func FetchURLWithRetry(ctx context.Context, url string, cfg *Config) (*gofeed.Feed, *FeedError) {
	if feedErr := validateURL(url); feedErr != nil {
		slog.Error("Invalid URL", slog.String(LogKeyURL, url), slog.Any(LogKeyError, feedErr))

		return nil, feedErr
	}

	fp := createFeedParser(cfg)

	var attempts uint = 0
	var lastError error
	var feed *gofeed.Feed

	err := retry.Do(
		func() error {
			select {
			case <-ctx.Done():
				lastError = ctx.Err()

				return ctx.Err()
			default:
				parsedFeed, err := fp.ParseURLWithContext(url, ctx)
				if err != nil {
					slog.Error("Parse FeedConfig Error",
						slog.String(LogKeyURL, url),
						slog.Any(LogKeyError, err))
					lastError = err

					return err
				}
				feed = parsedFeed

				return nil
			}
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
				slog.String(LogKeyURL, url),
				slog.Uint64(LogKeyAttempts, uint64(attempts)),
				slog.Any(LogKeyError, err))
		}),
	)
	if err != nil {
		if lastError == nil {
			lastError = err
		}
		slog.Error("Parse FeedConfig Error after retries",
			slog.String(LogKeyURL, url),
			slog.Uint64(LogKeyAttempts, uint64(attempts)),
			slog.Any(LogKeyError, err))

		return nil, &FeedError{
			URL:     url,
			Message: lastError.Error(),
			Err:     lastError,
		}
	}

	return feed, nil
}

// validateURL 验证URL.
func validateURL(url string) *FeedError {
	if url == "" {
		return &FeedError{
			URL:     url,
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

// FetchURLs 批量获取URLs.
func FetchURLs(ctx context.Context, urls []string, cfg *Config) ([]*gofeed.Feed, []*FeedError) {
	results := make([]fetchURLResult, len(urls))
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(DefaultFeedFetchConcurrency)

	for i, url := range urls {
		g.Go(func() error {
			feed, feedErr := FetchURLWithRetry(ctx, url, cfg)
			results[i] = fetchURLResult{feed: feed, err: feedErr}

			return nil
		})
	}

	_ = g.Wait()

	allFeeds := make([]*gofeed.Feed, 0, len(urls))
	failedFeeds := make([]*FeedError, 0)
	for _, result := range results {
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

	return allFeeds, failedFeeds
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
