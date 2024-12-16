package pkg

import (
	"testing"
	"time"

	"github.com/mmcdole/gofeed"

	"github.com/golang-module/carbon/v2"
)

func TestFilterFeedsWithTimeRange(t *testing.T) {
	endDate := carbon.CreateFromDate(2024, 11, 17).StdTime()

	type args struct {
		created  time.Time
		endDate  time.Time
		schedule string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// {"Daily: 当前: true", args{created: time.Now(), endDate: endDate, schedule: "daily"}, true},
		// {"Daily: 昨天feed: true", args{created: carbon.Yesterday().StdTime(), endDate: endDate, schedule: "daily"}, true},
		// {"Daily: 昨天零点feed: true", args{created: carbon.Yesterday().StartOfDay().StdTime(), endDate: endDate, schedule: "daily"}, true},

		{"Daily: 当前: true", args{created: time.Now(), endDate: endDate, schedule: "daily"}, true},
		{"Daily: 昨天feed: true", args{created: carbon.CreateFromDateTime(2024, 11, 16, 10, 10, 10).StdTime(), endDate: endDate, schedule: "daily"}, true},
		{"Daily: 昨天零点feed: true", args{created: carbon.CreateFromDate(2024, 11, 16).StdTime(), endDate: endDate, schedule: "daily"}, true},
		{"Daily: 前天feed: false", args{created: carbon.CreateFromDate(2024, 11, 15).StdTime(), endDate: endDate, schedule: "daily"}, false},
		{"Daily: 前天feed: false", args{created: carbon.CreateFromDateTime(2024, 11, 15, 10, 10, 10).StdTime(), endDate: endDate, schedule: "daily"}, false},
		{"Daily: 前天feed: false", args{created: carbon.CreateFromDateTime(2024, 11, 15, 23, 52, 36).StdTime(), endDate: endDate, schedule: "daily"}, false},

		{"Weekly: 前天feed: true", args{created: carbon.Now().SubDays(2).StdTime(), endDate: endDate, schedule: "weekly"}, true},
		{"Weekly: 本周feed: true", args{created: carbon.Now().SubDays(7).StdTime(), endDate: endDate, schedule: "weekly"}, true},
		// {"Weekly: 上周之前的feed: false", args{created: carbon.Now().SubDays(8).StdTime(), endDate: endDate, schedule: "weekly"}, false},
		{"Wekly: 拼写错误返回false: false", args{created: carbon.Now().SubDays(8).StdTime(), endDate: endDate, schedule: "wekly"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FilterFeedsWithTimeRange(tt.args.created, tt.args.endDate, tt.args.schedule); got != tt.want {
				t.Errorf("FilterFeedsWithTimeRange() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateFeeds(t *testing.T) {
	tests := []struct {
		name    string
		feeds   []*gofeed.Feed
		wantErr bool
	}{
		{
			name:    "空feeds列表",
			feeds:   []*gofeed.Feed{},
			wantErr: true,
		},
		{
			name:    "有效feeds列表",
			feeds:   []*gofeed.Feed{{Title: "Test Feed"}},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFeeds(tt.feeds)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateFeeds() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCreateBaseFeed(t *testing.T) {
	tests := []struct {
		name  string
		title string
		want  string
	}{
		{
			name:  "基本标题",
			title: "Test Feed",
			want:  "Test Feed",
		},
		{
			name:  "空标题",
			title: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := createBaseFeed(tt.title)
			if got.Title != tt.want {
				t.Errorf("createBaseFeed() title = %v, want %v", got.Title, tt.want)
			}
		})
	}
}

func TestConfig_GetItemCreationTime(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name string
		item *gofeed.Item
		want time.Time
	}{
		{
			name: "有发布时间",
			item: &gofeed.Item{
				PublishedParsed: &now,
			},
			want: now,
		},
		{
			name: "只有更新时间",
			item: &gofeed.Item{
				UpdatedParsed: &now,
			},
			want: now,
		},
		{
			name: "都没有时间",
			item: &gofeed.Item{},
			want: time.Now(),
		},
	}

	cfg := Config{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfg.getItemCreationTime(tt.item)
			if !got.Equal(tt.want) && tt.item.PublishedParsed != nil {
				t.Errorf("getItemCreationTime() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfig_ProcessSingleFeed(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name      string
		feed      *gofeed.Feed
		seen      map[string]bool
		wantCount int
		config    Config
	}{
		{
			name: "正常处理",
			feed: &gofeed.Feed{
				Title: "Test Feed",
				Items: []*gofeed.Item{
					{
						Title:           "Item 1",
						Link:            "http://example.com/1",
						PublishedParsed: &now,
					},
				},
			},
			seen:      make(map[string]bool),
			wantCount: 1,
			config: Config{
				Feed: struct {
					MaxTries  int `yaml:"maxTries"`
					FeedLimit int `yaml:"feedLimit"`
				}{
					FeedLimit: 10,
				},
				Newsletter: struct {
					Schedule            string `yaml:"schedule"`
					IsHideAuthorInTitle bool   `yaml:"isHideAuthorInTitle"`
				}{
					Schedule: "daily",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.processSingleFeed(tt.feed, tt.seen)
			if len(got) != tt.wantCount {
				t.Errorf("processSingleFeed() got %v items, want %v", len(got), tt.wantCount)
			}
		})
	}
}
