package cmd

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"log/slog"
	"os"

	"github.com/dromara/carbon/v2"
	"github.com/gorilla/feeds"
	"github.com/resend/resend-go/v2"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/pkg/errcode"
	"github.com/xbpk3t/docs-alfred/pkg/rss"
	"golang.org/x/sync/errgroup"
)

// 配置文件路径
var cfgFile string

//go:embed templates/newsletter.tpl
var newsletterTpl embed.FS

// EmailConfig 邮件配置
type EmailConfig struct {
	From  string
	Token string
	To    []string
}

// NewsletterService 处理新闻通讯的服务
type NewsletterService struct {
	config *rss.Config
	feed   *rss.Feed
}

// NewNewsletterService 创建新闻通讯服务
func NewNewsletterService(cfg *rss.Config) *NewsletterService {
	return &NewsletterService{
		config: cfg,
		feed:   rss.NewFeed(cfg),
	}
}

// rootCmd 根命令
var rootCmd = &cobra.Command{
	Use:   "rss2newsletter",
	Short: "RSS订阅转换为邮件推送工具",
	RunE:  runNewsletter,
}

func runNewsletter(cmd *cobra.Command, args []string) error {
	config, err := loadConfig()
	if err != nil {
		return err
	}

	service := NewNewsletterService(config)
	f, err := service.ProcessAllFeeds()
	if err != nil {
		return err
	}

	content, err := service.RenderNewsletter(f)
	if err != nil {
		return err
	}

	return service.SendNewsletter(content)
}

func loadConfig() (*rss.Config, error) {
	config, err := rss.NewConfig(cfgFile)
	if err != nil {
		slog.Error("rss2newsletter config file load error:", slog.Any("err", err))
		return nil, err
	}
	return config, nil
}

// ProcessAllFeeds 并发处理所有Feed源
func (s *NewsletterService) ProcessAllFeeds() ([]feeds.RssFeed, error) {
	g, ctx := errgroup.WithContext(context.Background())
	resultChan := make(chan feeds.RssFeed, len(s.config.Feeds))

	// 启动goroutine处理每个feed
	for _, feed := range s.config.Feeds {
		feed := feed // 避免闭包问题
		g.Go(func() error {
			rssFeed, err := s.processSingleFeed(feed)
			if err != nil {
				return err
			}
			select {
			case resultChan <- rssFeed:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	}

	// 等待所有goroutine完成或出错
	go func() {
		if err := g.Wait(); err != nil {
			close(resultChan)
			return
		}
		close(resultChan)
	}()

	// 收集结果
	var results []feeds.RssFeed
	for result := range resultChan {
		results = append(results, result)
	}

	// 检查是否有错误发生
	if err := g.Wait(); err != nil {
		return nil, err
	}

	return results, nil
}

// processSingleFeed 处理单个Feed源
func (s *NewsletterService) processSingleFeed(feed rss.FeedsDetail) (feeds.RssFeed, error) {
	urls := lo.Compact(lo.Map(feed.Urls, func(item rss.Feeds, _ int) string {
		return item.Feed
	}))

	allFeeds := s.feed.FetchURLs(context.TODO(), urls)
	if len(allFeeds) == 0 {
		slog.Info("No feeds fetched for category",
			slog.String("category", feed.Type),
			slog.Int("total_urls", len(urls)))
		return feeds.RssFeed{
			Category: feed.Type,
			Items:    []*feeds.RssItem{},
		}, nil
	}

	combinedFeed, err := s.feed.MergeAllFeeds(feed.Type, allFeeds)
	if err != nil {
		slog.Error("Failed to merge feeds",
			slog.String("category", feed.Type),
			slog.Int("feeds_count", len(allFeeds)),
			slog.Any("error", err))
		return feeds.RssFeed{
			Category: feed.Type,
			Items:    []*feeds.RssItem{},
		}, nil
	}

	return s.convertToRssFeed(feed.Type, combinedFeed), nil
}

// convertToRssFeed 将Feed转换为RssFeed格式
func (s *NewsletterService) convertToRssFeed(typeName string, combinedFeed *feeds.Feed) feeds.RssFeed {
	newFeeds := make([]*feeds.RssItem, len(combinedFeed.Items))
	for i, item := range combinedFeed.Items {
		title := s.getItemTitle(item)
		newFeeds[i] = &feeds.RssItem{
			Title:    title,
			Link:     item.Link.Href,
			Category: typeName,
			PubDate:  carbon.CreateFromStdTime(item.Created).ToDateTimeString(),
		}
	}
	return feeds.RssFeed{
		Category: typeName,
		Items:    newFeeds,
	}
}

// getItemTitle 生成文章标题
func (s *NewsletterService) getItemTitle(item *feeds.Item) string {
	if !s.config.Newsletter.IsHideAuthorInTitle && item.Author.Name != "" {
		return fmt.Sprintf("[%s] %s", item.Author.Name, item.Title)
	}
	return item.Title
}

// RenderNewsletter 渲染邮件模板
func (s *NewsletterService) RenderNewsletter(feeds []feeds.RssFeed) (string, error) {
	tmpl, err := template.ParseFS(newsletterTpl, "templates/newsletter.tpl")
	if err != nil {
		return "", errcode.WithError(errcode.ErrR2NParseTemplateFailed, err)
	}

	var tplBytes bytes.Buffer
	if err := tmpl.Execute(&tplBytes, feeds); err != nil {
		return "", errcode.WithError(errcode.ErrR2NRenderTemplateFailed, err)
	}

	return tplBytes.String(), nil
}

// SendNewsletter 发送邮件
func (s *NewsletterService) SendNewsletter(content string) error {
	emailCfg := EmailConfig{
		From:  "Acme <onboarding@resend.dev>",
		To:    []string{"jeffcottlu@gmail.com"},
		Token: s.config.Resend.Token,
	}

	ctx := context.Background()
	client := resend.NewClient(emailCfg.Token)

	params := &resend.SendEmailRequest{
		From:    emailCfg.From,
		To:      emailCfg.To,
		Subject: s.generateEmailSubject(),
		Html:    content,
	}

	sent, err := client.Emails.SendWithContext(ctx, params)
	if err != nil {
		return errcode.WithError(errcode.ErrR2NSendMailFailed, err)
	}

	slog.Info("邮件发送成功", "id", sent.Id)
	return nil
}

// generateEmailSubject 生成邮件主题
func (s *NewsletterService) generateEmailSubject() string {
	now := carbon.Now()
	return fmt.Sprintf("新内容更新 %s (第%d周)", now.ToDateString(), now.WeekOfYear())
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
}
