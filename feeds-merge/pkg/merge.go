package pkg

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/avast/retry-go"
	"github.com/gorilla/feeds"
	"github.com/mmcdole/gofeed"
)

type Config struct {
	Feed       FeedCfg       `yaml:"feed"`
	Resend     ResendCfg     `yaml:"resend"`
	Newsletter NewsletterCfg `yaml:"newsletter"`
	Categories
}

type FeedCfg struct {
	Timeout   int `yaml:"timeout"`
	MaxTries  int `yaml:"maxTries"`
	FeedLimit int `yaml:"feedLimit"`
}
type ResendCfg struct {
	Token string `yaml:"token"`
}
type NewsletterCfg struct {
	Schedule            string `yaml:"schedule"`
	IsHideAuthorInTitle bool   `yaml:"isHideAuthorInTitle"`
}

type Categories []Category

type Category struct {
	Type string `yaml:"type"`
	URLs []Feed `yaml:"urls"`
}

type Feed struct {
	Feed string `yaml:"feed"`
	Des  string `yaml:"des"`
	URL  string `yaml:"url"`
	Name string `yaml:"name"`
	Date string
}

const (
	Daily  = "daily"
	Weekly = "weekly"
)

// 时间范围映射
var scheduleTimeRanges = map[string]time.Duration{
	Daily:  24 * time.Hour,
	Weekly: 7 * 24 * time.Hour,
}

// func NewConfig() *Config {
// 	fx, err := os.ReadFile(feedFile)
// 	if err != nil {
// 		slog.Error("Read FeedCfg File Error, Check your path:", slog.String("FeedCfg File", feedFile))
// 		return nil
// 	}
// 	var cates []Category
// 	err = yaml.Unmarshal(fx, &cates)
// 	if err != nil {
// 		slog.Error("Unmarshal:", slog.Any("Error", err))
// 		return nil
// 	}
// 	return &Config{
// 		// Timeout:    30,
// 		// FeedLimit:  20,
// 		Categories: cates,
// 	}
// }

func NewConfig(cfgFile, feedFile string) *Config {
	fx, err := os.ReadFile(cfgFile)
	if err != nil {
		return nil
	}

	var cfg Config
	if err := yaml.Unmarshal(fx, &cfg); err != nil {
		return nil
	}

	ff, err := os.ReadFile(feedFile)
	if err != nil {
		return nil
	}
	var cates Categories
	if err := yaml.Unmarshal(ff, &cates); err != nil {
		return nil
	}
	cfg.Categories = cates

	return &cfg
}

// // fetchURL 直接获取
// func (e Config) fetchURL(url string, ch chan<- *gofeed.FeedCfg) {
// 	// core.Infof("Fetching URL: %v\n", url)
// 	fp := gofeed.NewParser()
// 	fp.Client = &http.Client{
// 		Timeout: time.Duration(e.Timeout) * time.Second,
// 	}
// 	feed, err := fp.ParseURL(url)
// 	if err == nil {
// 		ch <- feed
// 	} else {
// 		slog.Info("fetchURL Error:", slog.String("URL", url), slog.Any("Error", err))
// 		ch <- nil
// 	}
// }

// 尝试retry获取
func (e Config) FetchURLWithRetry(url string, ch chan<- *gofeed.Feed) {
	// core.Infof("Fetching URL: %v\n", url)
	fp := gofeed.NewParser()
	fp.Client = &http.Client{
		Timeout: time.Duration(e.Feed.Timeout) * time.Second,
	}

	var attempts uint = 0

	err := retry.Do(
		func() error {
			feed, err := fp.ParseURL(url)
			if err != nil {
				return err // 返回错误以触发重试
			}
			ch <- feed // 如果没有错误，发送 feed 并结束重试
			return nil
		},
		retry.Attempts(uint(e.Feed.MaxTries)), // 设置最大尝试次数，例如超时的5倍
		retry.Delay(time.Second*5),            // 设置初始延迟
		retry.DelayType(retry.BackOffDelay),   // 设置退避策略
		retry.LastErrorOnly(true),             // 只返回最后一个错误
		retry.OnRetry(func(n uint, err error) {
			attempts = n
			slog.Info("Retry Parse FeedCfg:", slog.String("URL", url), slog.Int("Tries", int(attempts)))
		}),
	)
	if err != nil {
		slog.Info("Parse FeedCfg Error:", slog.String("URL", url), slog.Any("Error", err))
		ch <- nil // 发送 nil 表示获取失败
	}

	// feed, err := fp.ParseURL(url)
	// if err == nil {
	// 	ch <- feed
	// } else {
	// 	core.Infof("Parse FeedCfg URL [%s] Error: (%v)\n", url, err)
	// 	ch <- nil
	// }
}

// 批量提取URL为gofeed.FeedCfg
func (e Config) FetchURLs(urls []string) []*gofeed.Feed {
	allFeeds := make([]*gofeed.Feed, 0)
	ch := make(chan *gofeed.Feed)
	for _, url := range urls {
		go e.FetchURLWithRetry(url, ch)
	}
	for range urls {
		feed := <-ch
		if feed != nil {
			allFeeds = append(allFeeds, feed)
		}
	}
	return allFeeds
}

// TODO: there must be a shorter syntax for this
// type byPublished []*gofeed.FeedCfg
//
// func (s byPublished) Len() int {
// 	return len(s)
// }
//
// func (s byPublished) Swap(i, j int) {
// 	s[i], s[j] = s[j], s[i]
// }
//
// func (s byPublished) Less(i, j int) bool {
// 	date1 := s[i].Items[0].PublishedParsed
// 	if date1 == nil {
// 		date1 = s[i].Items[0].UpdatedParsed
// 	}
// 	date2 := s[j].Items[0].PublishedParsed
// 	if date2 == nil {
// 		date2 = s[j].Items[0].UpdatedParsed
// 	}
// 	return date1.Before(*date2)
// }

func (e Config) getAuthor(feed *gofeed.Feed) string {
	// if feed.Author != nil {
	// 	return feed.Author.Name
	// }
	// if feed.Items[0].Author != nil {
	// 	return feed.Items[0].Author.Name
	// }
	if feed.Title != "" {
		return feed.Title
	}
	// slog.Info("Using Default Author for", feed.Link)
	return ""
}

// MergeAllFeeds
// feedTitle type
// allFeeds x
func (e Config) MergeAllFeeds(feedTitle string, allFeeds []*gofeed.Feed) (*feeds.Feed, error) {
	if len(allFeeds) == 0 {
		return nil, errors.New("no feeds Found")
	}

	feed := &feeds.Feed{
		Title: feedTitle,
		// Link:        &feeds.Link{Href: e.FeedLink},
		Description: "Merged feeds from " + feedTitle,
		// Author: &feeds.Author{
		// 	Name: e.Author,
		// },
		Created: time.Now(),
	}
	// sort.Sort(sort.Reverse(byPublished(allFeeds)))
	limitPerFeed := e.Feed.FeedLimit
	seen := make(map[string]bool)
	created := GetToday()
	for _, sourceFeed := range allFeeds {
		for i, item := range sourceFeed.Items {
			if i > limitPerFeed {
				break
			}
			if seen[item.Link] {
				continue
			}

			if item.UpdatedParsed != nil {
				created = *item.UpdatedParsed
			} else if item.PublishedParsed != nil {
				created = *item.PublishedParsed
			}

			timeRange, exists := scheduleTimeRanges[e.Newsletter.Schedule]
			if !exists {
				// 如果不存在对应的时间范围，可以选择跳过或者使用默认值
				continue
			}

			if created.After(GetToday().Add(-timeRange)) {
				feed.Items = append(feed.Items, &feeds.Item{
					Title: item.Title,
					Link:  &feeds.Link{Href: item.Link},
					// Description: item.Description,
					Author:  &feeds.Author{Name: e.getAuthor(sourceFeed)},
					Created: created,
					// Content:     item.Content,
				})
				seen[item.Link] = true
			}
		}
	}
	return feed, nil
}

func GetToday() time.Time {
	timeStr := time.Now().Format("2006-01-02")
	t, _ := time.ParseInLocation("2006-01-02 15:04:05", timeStr+" 00:00:00", time.Local)
	return t
	// return time.Now().Round(24 * time.Hour).Truncate(24 * time.Hour)
}
