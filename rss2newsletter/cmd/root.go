package cmd

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"log/slog"
	"os"
	"sync"

	"github.com/golang-module/carbon/v2"
	feeds2 "github.com/gorilla/feeds"
	"github.com/resend/resend-go/v2"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/rss2newsletter/pkg"
	"github.com/xbpk3t/docs-alfred/utils"
	"golang.org/x/sync/errgroup"
)

// 配置文件路径
var cfgFile string

//go:embed templates/newsletter.tpl
var newsletterTpl embed.FS

// EmailConfig 邮件配置
type EmailConfig struct {
	From  string
	To    []string
	Token string
}

// rootCmd 根命令
var rootCmd = &cobra.Command{
	Use:   "rss2newsletter",
	Short: "RSS订阅转换为邮件推送工具",
	RunE: func(cmd *cobra.Command, args []string) error {
		config := pkg.NewConfig(cfgFile)

		// 使用 errgroup 进行并发处理
		g, _ := errgroup.WithContext(context.Background())
		var results []feeds2.RssFeed
		var mu sync.Mutex // 保护 results

		for _, feed := range config.Feeds {
			feed := feed // 避免闭包问题
			g.Go(func() error {
				rssFeed, err := processFeed(feed, config)
				if err != nil {
					return err
				}

				mu.Lock()
				results = append(results, rssFeed)
				mu.Unlock()
				return nil
			})
		}

		if err := g.Wait(); err != nil {
			return err
		}

		// 渲染模板
		content, err := renderNewsletter(results)
		if err != nil {
			return err
		}

		// 发送邮件
		emailCfg := EmailConfig{
			From:  "Acme <onboarding@resend.dev>",
			To:    []string{"jeffcottlu@gmail.com"},
			Token: config.Resend.Token,
		}

		return sendNewsletter(emailCfg, content)
	},
}

// processFeed 处理单个Feed源
func processFeed(feed pkg.FeedsDetail, config *pkg.Config) (feeds2.RssFeed, error) {
	TypeName := feed.Type
	urls := lo.Compact(lo.Map(feed.Urls, func(item pkg.Feeds, _ int) string {
		return item.Feed
	}))

	allFeeds := config.FetchURLs(urls)
	if len(allFeeds) == 0 {
		return feeds2.RssFeed{}, fmt.Errorf("no feed found for type: %s", TypeName)
	}

	combinedFeed, err := config.MergeAllFeeds(TypeName, allFeeds)
	if err != nil {
		return feeds2.RssFeed{}, fmt.Errorf("merge feeds error: %w", err)
	}

	return convertToRssFeed(TypeName, combinedFeed, config.Newsletter.IsHideAuthorInTitle), nil
}

// convertToRssFeed 将Feed转换为RssFeed格式
func convertToRssFeed(typeName string, combinedFeed *feeds2.Feed, hideAuthor bool) feeds2.RssFeed {
	newFeeds := make([]*feeds2.RssItem, len(combinedFeed.Items))
	for i, item := range combinedFeed.Items {
		title := getItemTitle(item, hideAuthor)
		newFeeds[i] = &feeds2.RssItem{
			Title:    title,
			Link:     item.Link.Href,
			Category: typeName,
			PubDate:  utils.FormatDate(item.Created),
		}
	}
	return feeds2.RssFeed{
		Category: typeName,
		Items:    newFeeds,
	}
}

// getItemTitle 生成文章标题
func getItemTitle(item *feeds2.Item, hideAuthor bool) string {
	if !hideAuthor && item.Author.Name != "" {
		return fmt.Sprintf("[%s] %s", item.Author.Name, item.Title)
	}
	return item.Title
}

// renderNewsletter 渲染邮件模板
func renderNewsletter(feeds []feeds2.RssFeed) (string, error) {
	tmpl, err := template.ParseFS(newsletterTpl, "templates/newsletter.tpl")
	if err != nil {
		return "", fmt.Errorf("解析模板失败: %w", err)
	}

	var tplBytes bytes.Buffer
	if err := tmpl.Execute(&tplBytes, feeds); err != nil {
		return "", fmt.Errorf("渲染模板失败: %w", err)
	}

	return tplBytes.String(), nil
}

// sendNewsletter 发送邮件
func sendNewsletter(emailCfg EmailConfig, content string) error {
	ctx := context.Background()
	client := resend.NewClient(emailCfg.Token)

	params := &resend.SendEmailRequest{
		From:    emailCfg.From,
		To:      emailCfg.To,
		Subject: fmt.Sprintf("新内容更新 %s (第%d周)", carbon.Now().ToDateString(), utils.WeekNumOfYear()),
		Html:    content,
	}

	sent, err := client.Emails.SendWithContext(ctx, params)
	if err != nil {
		return fmt.Errorf("发送邮件失败: %w", err)
	}
	slog.Info("邮件发送成功", "id", sent.Id)
	return nil
}

// Execute 执行根命令
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		slog.Error("执行命令失败", "error", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "rss2newsletter.yml", "配置文件路径")
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
