package rss

import (
	"fmt"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
)

// DashboardData represents all dashboard information
type DashboardData struct {
	FailedFeeds []FailedFeedInfo `json:"failedFeeds,omitempty"`
	FeedDetails []FeedDetailInfo `json:"feedDetails,omitempty"`
}

// FailedFeedInfo represents information about a failed feed
type FailedFeedInfo struct {
	URL   string `json:"url"`
	Error string `json:"error"`
}

// FeedDetailInfo represents detailed information about a feed
type FeedDetailInfo struct {
	Type  string `json:"type"`
	URL   string `json:"url"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// renderTable 通用表格渲染函数
func renderTable(t table.Writer) string {
	// 设置HTML选项
	t.SetStyle(table.Style{
		Name: "CustomHTML",
		Box: table.BoxStyle{
			BottomLeft:      "└",
			BottomRight:     "┘",
			BottomSeparator: "┴",
			Left:            "│",
			LeftSeparator:   "├",
			MiddleSeparator: "┼",
			PaddingLeft:     " ",
			PaddingRight:    " ",
			Right:           "│",
			RightSeparator:  "┤",
			TopLeft:         "┌",
			TopRight:        "┐",
			TopSeparator:    "┬",
			UnfinishedRow:   " ~~~",
		},
		Options: table.Options{
			DrawBorder:      true,
			SeparateColumns: true,
			SeparateFooter:  true,
			SeparateHeader:  true,
			SeparateRows:    false,
		},
		HTML: table.HTMLOptions{
			CSSClass:    "dashboard-table",
			EmptyColumn: "&nbsp;",
			EscapeText:  false, // 允许HTML标签
			Newline:     "<br/>",
		},
	})

	return t.RenderHTML()
}

// ShowFetchFailedFeeds displays all failed feeds in a table format
func ShowFetchFailedFeeds(failedFeeds []*FeedError) string {
	if len(failedFeeds) == 0 {
		return ""
	}

	t := table.NewWriter()
	t.AppendHeader(table.Row{"Feed URL", "Error"})

	// 设置列配置
	t.SetColumnConfigs([]table.ColumnConfig{
		{
			Name:  "Feed URL",
			Align: text.AlignLeft,
			Transformer: text.Transformer(func(val interface{}) string {
				url := fmt.Sprintf("%v", val)
				return fmt.Sprintf(`<a href="%s" target="_blank">%s</a>`, url, url)
			}),
		},
		{
			Name:  "Error",
			Align: text.AlignLeft,
		},
	})

	for _, feed := range failedFeeds {
		errorMsg := feed.Message
		if feed.Err != nil {
			errorMsg = feed.Err.Error()
		}
		t.AppendRow(table.Row{
			feed.URL,
			errorMsg,
		})
	}

	return renderTable(t)
}

// ShowFeedDetail displays detailed information about each feed
func ShowFeedDetail(feeds []FeedsDetail) string {
	if len(feeds) == 0 {
		return ""
	}

	var content strings.Builder

	// 遍历每个feed类型，为每个类型创建独立的表格
	for _, feed := range feeds {
		count := len(feed.Urls)

		// 添加类型和数量信息
		content.WriteString(fmt.Sprintf(`<div class="feed-type-header">Type: %s &nbsp;&nbsp;&nbsp; Count: %d</div>`, feed.Type, count))

		// 创建该类型的表格
		t := table.NewWriter()
		t.AppendHeader(table.Row{"Feed"})

		// 设置列配置
		t.SetColumnConfigs([]table.ColumnConfig{
			{
				Name:  "Feed",
				Align: text.AlignLeft,
				Transformer: text.Transformer(func(val interface{}) string {
					url := val.(feedInfo).url
					name := val.(feedInfo).name
					if name == "" {
						return fmt.Sprintf(`<a href="%s" target="_blank">%s</a>`, url, url)
					}
					return fmt.Sprintf(`<a href="%s" target="_blank">%s</a>`, url, name)
				}),
			},
		})

		// 添加该类型下的所有feed
		for _, url := range feed.Urls {
			t.AppendRow(table.Row{
				feedInfo{
					url:  url.URL,
					name: url.Des,
				},
			})
		}

		// 渲染表格并添加到内容中
		content.WriteString(renderTable(t))
		content.WriteString("\n")
	}

	return content.String()
}

// feedInfo 用于传递feed信息到transformer
type feedInfo struct {
	url  string
	name string
}

// GenerateDashboardHTML generates dashboard content using go-pretty tables
func GenerateDashboardHTML(config *Config, failedFeeds []*FeedError) string {
	var content strings.Builder

	// Generate failed feeds table if enabled
	if config.DashboardConfig.IsShowFetchFailedFeeds && len(failedFeeds) > 0 {
		content.WriteString(`<div class="dashboard-section">`)
		content.WriteString(`<details>`)
		content.WriteString(`<summary>Failed Feeds</summary>`)
		failedFeedsTable := ShowFetchFailedFeeds(failedFeeds)
		content.WriteString(failedFeedsTable)
		content.WriteString(`</details>`)
		content.WriteString(`</div>`)
	}

	// Generate feed details table if enabled
	if config.DashboardConfig.IsShowFeedDetail {
		content.WriteString(`<div class="dashboard-section">`)
		content.WriteString(`<details>`)
		content.WriteString(`<summary>Feed Details</summary>`)
		feedDetailsTable := ShowFeedDetail(config.Feeds)
		content.WriteString(feedDetailsTable)
		content.WriteString(`</details>`)
		content.WriteString(`</div>`)
	}

	return content.String()
}
