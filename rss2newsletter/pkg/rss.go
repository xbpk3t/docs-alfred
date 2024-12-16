package pkg

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/avast/retry-go"
	"github.com/golang-module/carbon/v2"
	"github.com/gorilla/feeds"
	"github.com/mmcdole/gofeed"
	"gopkg.in/yaml.v3"
)

// 错误定义
const (
	ErrNoFeedsFound    = "no feeds found"
	ErrInvalidSchedule = "invalid schedule type"
)

// 时间相关常量
const (
	Daily  = "daily"
	Weekly = "weekly"

	// 重试相关常量
	DefaultRetryDelay = 5 * time.Second
	DefaultMaxRetries = 3
	DefaultFeedLimit  = 10
)

// 日志字段常量
const (
	LogKeyURL       = "url"
	LogKeyAttempts  = "attempts"
	LogKeyError     = "error"
	LogKeyFeedTitle = "feed_title"
)

// Config 主配置结构
type Config struct {
	Resend     ResendConfig     `yaml:"resend"`
	Newsletter NewsletterConfig `yaml:"newsletter"`
	Feeds      []FeedsDetail    `yaml:"feeds"`
	Feed       FeedConfig       `yaml:"feed"`
}

// ResendConfig Resend相关配置
type ResendConfig struct {
	Token string `yaml:"token"`
}

// NewsletterConfig 新闻通讯配置
type NewsletterConfig struct {
	Schedule            string `yaml:"schedule"`
	IsHideAuthorInTitle bool   `yaml:"isHideAuthorInTitle"`
}

// FeedConfig Feed相关配置
type FeedConfig struct {
	MaxTries  int `yaml:"maxTries"`
	FeedLimit int `yaml:"feedLimit"`
}

// FeedsDetail Feed详情
type FeedsDetail struct {
	Type string  `yaml:"type"`
	Urls []Feeds `yaml:"urls"`
}

// Feeds Feed URL
type Feeds struct {
	Feed string `yaml:"feed"`
}

// FeedError 自定义错误类型
type FeedError struct {
	URL     string
	Message string
	Err     error
}

func (e *FeedError) Error() string {
	return fmt.Sprintf("feed error for %s: %s: %v", e.URL, e.Message, e.Err)
}

// 时间范围映射
var scheduleTimeRanges = map[string]int{
	Daily:  24,
	Weekly: 7 * 24,
}

// FeedFetcher 定义Feed获取接口
type FeedFetcher interface {
	FetchFeed(ctx context.Context, url string) (*gofeed.Feed, error)
}

// FeedProcessor 定义Feed处理接口
type FeedProcessor interface {
	ProcessFeed(feed *gofeed.Feed) ([]*feeds.Item, error)
}

// NewConfig 创建新的配置
func NewConfig(cfgFile string) (*Config, error) {
	fx, err := os.ReadFile(cfgFile)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(fx, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return &cfg, nil
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.Resend.Token == "" {
		return errors.New("resend token is required")
	}

	if !isValidSchedule(c.Newsletter.Schedule) {
		return fmt.Errorf("invalid schedule: %s", c.Newsletter.Schedule)
	}

	return nil
}

func isValidSchedule(schedule string) bool {
	_, exists := scheduleTimeRanges[schedule]
	return exists
}

// Run 运行主程序
func (e *Config) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		cancel()
	}()

	var urls []string
	for _, feed := range e.Feeds {
		for _, url := range feed.Urls {
			urls = append(urls, url.Feed)
		}
	}

	f := e.FetchURLs(ctx, urls)
	_, err := e.MergeAllFeeds("FeedTitle", f)
	return err
}

// FetchURLWithRetry 重试获取URL内容
func (e *Config) FetchURLWithRetry(ctx context.Context, url string, ch chan<- *gofeed.Feed) {
	if err := e.validateURL(url); err != nil {
		slog.Error("Invalid URL", slog.String(LogKeyURL, url), slog.Any(LogKeyError, err))
		ch <- nil
		return
	}

	fp := gofeed.NewParser()
	fp.Client = &http.Client{}

	var attempts uint = 0

	err := retry.Do(
		func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				feed, err := fp.ParseURL(url)
				if err != nil {
					return err
				}
				ch <- feed
				return nil
			}
		},
		retry.Context(ctx),
		retry.Attempts(uint(e.Feed.MaxTries)),
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
		slog.Error("Parse Feed Error",
			slog.String(LogKeyURL, url),
			slog.Any(LogKeyError, err))
		ch <- nil
	}
}

// validateURL 验证URL
func (e *Config) validateURL(url string) error {
	if url == "" {
		return &FeedError{
			URL:     url,
			Message: "empty URL",
		}
	}
	return nil
}

// FetchURLs 批量获取URLs
func (e *Config) FetchURLs(ctx context.Context, urls []string) []*gofeed.Feed {
	allFeeds := make([]*gofeed.Feed, 0)
	ch := make(chan *gofeed.Feed, len(urls))
	var wg sync.WaitGroup

	for _, url := range urls {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			e.FetchURLWithRetry(ctx, url, ch)
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
func (e *Config) MergeAllFeeds(feedTitle string, allFeeds []*gofeed.Feed) (*feeds.Feed, error) {
	if err := validateFeeds(allFeeds); err != nil {
		return nil, err
	}

	feed := createBaseFeed(feedTitle)
	items := e.processFeeds(allFeeds)
	feed.Items = items
	return feed, nil
}

func validateFeeds(feeds []*gofeed.Feed) error {
	if len(feeds) == 0 {
		return errors.New(ErrNoFeedsFound)
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

func (e *Config) processFeeds(allFeeds []*gofeed.Feed) []*feeds.Item {
	seen := make(map[string]bool)
	var mergedItems []*feeds.Item

	for _, sourceFeed := range allFeeds {
		items := e.processSingleFeed(sourceFeed, seen)
		mergedItems = append(mergedItems, items...)
	}

	return mergedItems
}

func (e *Config) processSingleFeed(sourceFeed *gofeed.Feed, seen map[string]bool) []*feeds.Item {
	var items []*feeds.Item

	for i, item := range sourceFeed.Items {
		if i >= e.Feed.FeedLimit {
			break
		}

		if seen[item.Link] {
			continue
		}

		created := e.getItemCreationTime(item)
		if !FilterFeedsWithTimeRange(created, time.Now(), e.Newsletter.Schedule) {
			continue
		}

		feedItem := &feeds.Item{
			Title:   item.Title,
			Link:    &feeds.Link{Href: item.Link},
			Author:  &feeds.Author{Name: e.getAuthor(sourceFeed)},
			Created: created,
		}

		items = append(items, feedItem)
		seen[item.Link] = true
	}

	return items
}

func (e *Config) getItemCreationTime(item *gofeed.Item) time.Time {
	if item.PublishedParsed != nil {
		return *item.PublishedParsed
	}
	if item.UpdatedParsed != nil {
		return *item.UpdatedParsed
	}
	return time.Now()
}

func (e *Config) getAuthor(feed *gofeed.Feed) string {
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
