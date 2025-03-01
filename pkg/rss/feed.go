package rss

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/avast/retry-go"
	"github.com/dromara/carbon/v2"
	"github.com/gorilla/feeds"
	"github.com/mmcdole/gofeed"
)

// Run 运行主程序
func (f *Feed) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		cancel()
	}()

	var urls []string
	for _, feed := range f.Config.Feeds {
		for _, url := range feed.Urls {
			urls = append(urls, url.Feed)
		}
	}

	_, err := f.MergeAllFeeds("FeedTitle", f.FetchURLs(ctx, urls))
	return err
}

// FetchURLWithRetry 重试获取URL内容
func (f *Feed) FetchURLWithRetry(ctx context.Context, url string, ch chan<- *gofeed.Feed) {
	if err := f.validateURL(url); err != nil {
		slog.Error("Invalid URL", slog.String(LogKeyURL, url), slog.Any(LogKeyError, err))
		ch <- nil
		return
	}

	fp := gofeed.NewParser()
	fp.Client = &http.Client{
		Timeout: time.Duration(f.Config.Feed.Timeout) * time.Second,
	}

	var attempts uint = 0

	err := retry.Do(
		func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				feed, err := fp.ParseURL(url)
				if err != nil {
					slog.Error("Parse Feed Error",
						slog.String(LogKeyURL, url),
						slog.Any(LogKeyError, err))
					return err
				}
				ch <- feed
				return nil
			}
		},
		retry.Context(ctx),
		retry.Attempts(uint(f.Config.Feed.MaxTries)),
		retry.Delay(DefaultRetryDelay),
		retry.DelayType(retry.BackOffDelay),
		retry.LastErrorOnly(true),
		retry.OnRetry(func(n uint, err error) {
			attempts = n
			slog.Info("Retry Parse Feed",
				slog.String(LogKeyURL, url),
				slog.Int(LogKeyAttempts, int(attempts)),
				slog.Any(LogKeyError, err))
		}),
	)
	if err != nil {
		slog.Error("Parse Feed Error after retries",
			slog.String(LogKeyURL, url),
			slog.Int(LogKeyAttempts, int(attempts)),
			slog.Any(LogKeyError, err))
		ch <- nil
	}
}

// validateURL 验证URL
func (f *Feed) validateURL(url string) error {
	if url == "" {
		return &FeedError{
			URL:     url,
			Message: "empty URL",
		}
	}
	return nil
}

// FetchURLs 批量获取URLs
func (f *Feed) FetchURLs(ctx context.Context, urls []string) []*gofeed.Feed {
	allFeeds := make([]*gofeed.Feed, 0)
	ch := make(chan *gofeed.Feed, len(urls))
	var wg sync.WaitGroup

	for _, url := range urls {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			f.FetchURLWithRetry(ctx, url, ch)
		}(url)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	for feed := range ch {
		if feed != nil {
			allFeeds = append(allFeeds, feed)
		}
	}

	return allFeeds
}

// MergeAllFeeds 合并所有feeds
func (f *Feed) MergeAllFeeds(feedTitle string, allFeeds []*gofeed.Feed) (*feeds.Feed, error) {
	if err := validateFeeds(allFeeds); err != nil {
		return nil, err
	}

	feed := createBaseFeed(feedTitle)
	items := f.processFeeds(allFeeds)
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

func (f *Feed) processFeeds(allFeeds []*gofeed.Feed) []*feeds.Item {
	seen := make(map[string]bool)
	var mergedItems []*feeds.Item

	for _, sourceFeed := range allFeeds {
		items := f.processSingleFeed(sourceFeed, seen)
		mergedItems = append(mergedItems, items...)
	}

	return mergedItems
}

func (f *Feed) processSingleFeed(sourceFeed *gofeed.Feed, seen map[string]bool) []*feeds.Item {
	var items []*feeds.Item

	for i, item := range sourceFeed.Items {
		if i >= f.Config.Feed.FeedLimit {
			break
		}

		if seen[item.Link] {
			continue
		}

		created := f.getItemCreationTime(item)
		if !FilterFeedsWithTimeRange(created, time.Now(), f.Config.Newsletter.Schedule) {
			continue
		}

		feedItem := &feeds.Item{
			Title:   item.Title,
			Link:    &feeds.Link{Href: item.Link},
			Author:  &feeds.Author{Name: f.getAuthor(sourceFeed)},
			Created: created,
		}

		items = append(items, feedItem)
		seen[item.Link] = true
	}

	return items
}

func (f *Feed) getItemCreationTime(item *gofeed.Item) time.Time {
	if item.PublishedParsed != nil {
		return *item.PublishedParsed
	}
	if item.UpdatedParsed != nil {
		return *item.UpdatedParsed
	}
	return time.Now()
}

func (f *Feed) getAuthor(feed *gofeed.Feed) string {
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
