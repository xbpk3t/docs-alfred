package rss

import (
	"context"

	"github.com/gorilla/feeds"
	"github.com/mmcdole/gofeed"
)

// FeedFetcher 定义Feed获取接口
type FeedFetcher interface {
	FetchFeed(ctx context.Context, url string) (*gofeed.Feed, error)
}

// FeedProcessor 定义Feed处理接口
type FeedProcessor interface {
	ProcessFeed(feed *gofeed.Feed) ([]*feeds.Item, error)
}
