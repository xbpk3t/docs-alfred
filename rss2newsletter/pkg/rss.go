package pkg

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/golang-module/carbon/v2"

	"gopkg.in/yaml.v3"

	"github.com/avast/retry-go"
	"github.com/gorilla/feeds"
	"github.com/mmcdole/gofeed"
)

type Config struct {
	Resend struct {
		Token string `yaml:"token"`
	} `yaml:"resend"`
	Newsletter struct {
		Schedule            string `yaml:"schedule"`
		IsHideAuthorInTitle bool   `yaml:"isHideAuthorInTitle"`
	} `yaml:"newsletter"`
	Feeds []FeedsDetail `yaml:"feeds"`
	Feed  struct {
		MaxTries  int `yaml:"maxTries"`
		FeedLimit int `yaml:"feedLimit"`
	} `yaml:"feed"` // Timeout   int `yaml:"timeout"`
}

type FeedsDetail struct {
	Type string  `yaml:"type"`
	Urls []Feeds `yaml:"urls"`
}

type Feeds struct {
	Feed string `yaml:"feed"`
}

const (
	Daily  = "daily"
	Weekly = "weekly"
)

// 时间范围映射
var scheduleTimeRanges = map[string]int{
	Daily:  24,
	Weekly: 7 * 24,
}

func NewConfig(cfgFile string) *Config {
	fx, err := os.ReadFile(cfgFile)
	if err != nil {
		return nil
	}

	var cfg Config
	if err := yaml.Unmarshal(fx, &cfg); err != nil {
		return nil
	}

	return &cfg
}

// 尝试retry获取
func (e Config) FetchURLWithRetry(url string, ch chan<- *gofeed.Feed) {
	// core.Infof("Fetching URL: %v\n", url)
	fp := gofeed.NewParser()
	// fp.Client = &http.Client{
	// 	Timeout: time.Duration(e.Feed.Timeout) * time.Second,
	// }

	fp.Client = &http.Client{}

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
			slog.Info("Retry Parse Feed:", slog.String("URL", url), slog.Int("Tries", int(attempts)))
		}),
	)
	if err != nil {
		slog.Info("Parse Feed Error:", slog.String("URL", url), slog.Any("Error", err))
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
	created := carbon.Now().StartOfDay().StdTime()

	for _, sourceFeed := range allFeeds {
		for i, item := range sourceFeed.Items {
			if i > limitPerFeed {
				break
			}
			if seen[item.Link] {
				continue
			}

			if item.PublishedParsed != nil {
				created = *item.PublishedParsed
			} else if item.UpdatedParsed != nil {
				created = *item.UpdatedParsed
			}

			if FilterFeedsWithTimeRange(created, time.Now(), e.Newsletter.Schedule) {
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

func FilterFeedsWithTimeRange(created, endDate time.Time, schedule string) bool {
	timeRange, exists := scheduleTimeRanges[schedule]
	if !exists {
		// 如果不存在对应的时间范围，可以选择跳过或者使用默认值
		slog.Error("schedule错误，未匹配到，请检查拼写或者是否有该schedule")
		return false
	}

	createdTime := carbon.CreateFromStdTime(created)

	// if createdTime.Gte(carbon.Yesterday().StartOfDay()) && createdTime.Lt(carbon.Now().StartOfDay()) {
	// 	return true
	// }

	// lt := createdTime.Lt(carbon.Now().StartOfDay())

	return createdTime.Gte(carbon.CreateFromStdTime(endDate).SubHours(timeRange).StartOfDay())
}
