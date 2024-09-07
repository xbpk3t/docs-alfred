package pkg

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/avast/retry-go"
	"gopkg.in/yaml.v3"

	"github.com/gorilla/feeds"
	"github.com/mmcdole/gofeed"
)

type Config struct {
	Path       string
	Author     string
	FeedLink   string
	Categories []Categories `yaml:"categories"`
	Timeout    int
	FeedLimit  int // 每个feed只获取最近n条item
}

type Categories struct {
	Type  string `yaml:"type"`
	Feeds []Feed
}

type Feed struct {
	Feed string `yaml:"feed"`
	Des  string `yaml:"des"`
	URL  string `yaml:"url"`
	Name string `yaml:"name"`
}

var (
	once sync.Once
	Conf *Config
)

func NewConfig() *Config {
	once.Do(func() {
		// ti := EnvStrToInt("INPUT_CLIENT_TIMEOUT", 30)
		// feedLimit := EnvStrToInt("INPUT_FEED_LIMIT", 300)
		path := ReadEnv("INPUT_FEEDS_PATH", ".github/workspace/feeds.yml")
		fx, err := os.ReadFile(path)
		if err != nil {
			return
		}
		var cates []Categories
		err = yaml.Unmarshal(fx, &cates)
		if err != nil {
			slog.Info("Unmarshal:", slog.Any("Error", err))
			return
		}
		Conf = &Config{
			Path:       path,
			Timeout:    30, // 最多retry 6次，所以设置为30（第一次retry 5s，则第6次 = 5 * 2^5 = 160s，合计315s）
			FeedLimit:  20,
			Author:     ReadEnv("INPUT_AUTHOR_NAME", "github-actions"),
			FeedLink:   ReadEnv("INPUT_FEED_LINK", ""),
			Categories: cates,
		}
	})
	return Conf
}

func ReadEnv(envKey, def string) string {
	if val := os.Getenv(envKey); val != "" {
		return val
	}
	return def
}

func EnvStrToInt(envKey string, def int) int {
	val := os.Getenv(envKey)
	ti, err := strconv.Atoi(val)
	if err != nil {
		// core.Infof("set Config [%s] error: %v", envKey, err)
		return def
	}
	return ti
}

// // fetchURL 直接获取
// func (e Config) fetchURL(url string, ch chan<- *gofeed.Feed) {
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
		Timeout: time.Duration(e.Timeout) * time.Second,
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
		retry.Attempts(uint(e.Timeout/5)),   // 设置最大尝试次数，例如超时的5倍
		retry.Delay(time.Second*5),          // 设置初始延迟
		retry.DelayType(retry.BackOffDelay), // 设置退避策略
		retry.LastErrorOnly(true),           // 只返回最后一个错误
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
	// 	core.Infof("Parse Feed URL [%s] Error: (%v)\n", url, err)
	// 	ch <- nil
	// }
}

// 批量提取URL为gofeed.Feed
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
// type byPublished []*gofeed.Feed
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
	return e.Author
}

func (e Config) MergeAllFeeds(feedTitle string, allFeeds []*gofeed.Feed) (*feeds.Feed, error) {
	if len(allFeeds) == 0 {
		return nil, errors.New("no Feeds Found")
	}

	feed := &feeds.Feed{
		Title:       feedTitle,
		Link:        &feeds.Link{Href: e.FeedLink},
		Description: "Merged feeds from " + feedTitle,
		Author: &feeds.Author{
			Name: e.Author,
		},
		Created: time.Now(),
	}
	// sort.Sort(sort.Reverse(byPublished(allFeeds)))
	limitPerFeed := e.FeedLimit
	seen := make(map[string]bool)
	for _, sourceFeed := range allFeeds {
		for i, item := range sourceFeed.Items {
			if i > limitPerFeed {
				break
			}
			if seen[item.Link] {
				continue
			}
			// created := item.PublishedParsed
			// if created == nil {
			// 	created = item.UpdatedParsed
			// }

			created := GetToday()
			if item.UpdatedParsed != nil {
				created = *item.UpdatedParsed
			}
			if item.PublishedParsed != nil {
				created = *item.PublishedParsed
			}
			if item.UpdatedParsed == nil && item.PublishedParsed == nil {
				created = time.Now()
			}

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
	return feed, nil
}

func GetToday() time.Time {
	timeStr := time.Now().Format("2006-01-02")
	t, _ := time.ParseInLocation("2006-01-02 15:04:05", timeStr+" 00:00:00", time.Local)
	return t
}
