package rss

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/avast/retry-go"
	"github.com/dromara/carbon/v2"
	"github.com/gorilla/feeds"
	"github.com/mmcdole/gofeed"
)

// FetchURLWithRetry 重试获取URL内容
func FetchURLWithRetry(ctx context.Context, url string, ch chan<- *gofeed.Feed, cfg *Config) {
	if err := validateURL(url); err != nil {
		slog.Error("Invalid URL", slog.String(LogKeyURL, url), slog.Any(LogKeyError, err))
		ch <- nil
		return
	}

	fp := gofeed.NewParser()
	fp.Client = &http.Client{
		Timeout: time.Duration(cfg.FeedConfig.Timeout) * time.Second,
	}

	var attempts uint = 0
	var lastError error

	err := retry.Do(
		func() error {
			select {
			case <-ctx.Done():
				lastError = ctx.Err()
				return ctx.Err()
			default:
				feed, err := fp.ParseURL(url)
				if err != nil {
					slog.Error("Parse FeedConfig Error",
						slog.String(LogKeyURL, url),
						slog.Any(LogKeyError, err))
					lastError = err
					return err
				}
				ch <- feed
				return nil
			}
		},
		retry.Context(ctx),
		retry.Attempts(uint(cfg.FeedConfig.MaxTries)),
		retry.Delay(DefaultRetryDelay),
		retry.DelayType(retry.BackOffDelay),
		retry.LastErrorOnly(true),
		retry.OnRetry(func(n uint, err error) {
			attempts = n
			lastError = err
			slog.Info("Retry Parse FeedConfig",
				slog.String(LogKeyURL, url),
				slog.Int(LogKeyAttempts, int(attempts)),
				slog.Any(LogKeyError, err))
		}),
	)
	if err != nil {
		slog.Error("Parse FeedConfig Error after retries",
			slog.String(LogKeyURL, url),
			slog.Int(LogKeyAttempts, int(attempts)),
			slog.Any(LogKeyError, err))
		ch <- &gofeed.Feed{
			FeedType: "error",
			Title:    url,
			Custom:   map[string]string{"error": lastError.Error()},
		}
	}
}

// validateURL 验证URL
func validateURL(url string) error {
	if url == "" {
		return &FeedError{
			URL:     url,
			Message: "empty URL",
			Err:     nil,
		}
	}
	return nil
}

// FetchURLs 批量获取URLs
func FetchURLs(ctx context.Context, urls []string, cfg *Config) ([]*gofeed.Feed, []*FeedError) {
	allFeeds := make([]*gofeed.Feed, 0)
	failedFeeds := make([]*FeedError, 0)
	ch := make(chan *gofeed.Feed, len(urls))
	var wg sync.WaitGroup

	for _, url := range urls {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			FetchURLWithRetry(ctx, url, ch, cfg)
		}(url)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	for feed := range ch {
		if feed != nil {
			if feed.FeedType == "error" {
				// 记录失败的feed并使用原始错误信息
				slog.Error("Failed to fetch feed",
					slog.String("url", feed.Title),
					slog.String("error", feed.Custom["error"]))
				// 添加到失败列表
				failedFeeds = append(failedFeeds, &FeedError{
					URL:     feed.Title,
					Message: feed.Custom["error"],
					Err:     nil,
				})
			} else {
				allFeeds = append(allFeeds, feed)
			}
		}
	}

	return allFeeds, failedFeeds
}

// MergeAllFeeds 合并所有feeds
func MergeAllFeeds(feedTitle string, allFeeds []*gofeed.Feed, cfg *Config) (*feeds.Feed, error) {
	if err := validateFeeds(allFeeds); err != nil {
		return nil, err
	}

	feed := createBaseFeed(feedTitle)
	items := processFeeds(allFeeds, cfg)
	feed.Items = items
	return feed, nil
}

func validateFeeds(feeds []*gofeed.Feed) error {
	if len(feeds) == 0 {
		slog.Info("No feeds found, skipping")
		return nil
	}
	return nil
}

func createBaseFeed(title string) *feeds.Feed {
	return &feeds.Feed{
		Title:       title,
		Description: "Merged feeds from " + title,
		Created:     time.Now(),
	}
}

func processFeeds(allFeeds []*gofeed.Feed, cfg *Config) []*feeds.Item {
	seen := make(map[string]bool)
	var mergedItems []*feeds.Item

	for _, sourceFeed := range allFeeds {
		items := processSingleFeed(sourceFeed, seen, cfg)
		mergedItems = append(mergedItems, items...)
	}

	return mergedItems
}

func processSingleFeed(sourceFeed *gofeed.Feed, seen map[string]bool, cfg *Config) []*feeds.Item {
	var items []*feeds.Item

	for i, item := range sourceFeed.Items {
		if i >= cfg.FeedConfig.FeedLimit {
			break
		}

		if seen[item.Link] {
			continue
		}

		created := getItemCreationTime(item)
		if !FilterFeedsWithTimeRange(created, time.Now(), cfg.NewsletterConfig.Schedule) {
			continue
		}

		feedItem := &feeds.Item{
			Title:   item.Title,
			Link:    &feeds.Link{Href: item.Link},
			Author:  &feeds.Author{Name: getAuthor(sourceFeed)},
			Created: created,
		}

		items = append(items, feedItem)
		seen[item.Link] = true
	}

	return items
}

func getItemCreationTime(item *gofeed.Item) time.Time {
	if item.PublishedParsed != nil {
		return *item.PublishedParsed
	}
	if item.UpdatedParsed != nil {
		return *item.UpdatedParsed
	}
	return time.Now()
}

func getAuthor(feed *gofeed.Feed) string {
	if feed.Title != "" {
		return feed.Title
	}
	return ""
}

// FilterFeedsWithTimeRange 根据时间范围过滤feeds
func FilterFeedsWithTimeRange(created, endDate time.Time, schedule string) bool {
	timeRange, exists := scheduleTimeRanges[schedule]
	if !exists {
		slog.Error("Invalid schedule type",
			slog.String("schedule", schedule))
		return false
	}

	createdTime := carbon.CreateFromStdTime(created)
	return createdTime.Gte(carbon.CreateFromStdTime(endDate).SubHours(timeRange).StartOfDay())
}
