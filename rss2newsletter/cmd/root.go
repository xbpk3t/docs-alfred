package cmd

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"log/slog"
	"os"
	"time"

	"github.com/dromara/carbon/v2"
	"github.com/gorilla/feeds"
	"github.com/resend/resend-go/v2"
	"github.com/samber/lo"
	"github.com/sourcegraph/conc/pool"
	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/pkg/rss"
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
	config      *rss.Config
	failedFeeds []*rss.FeedError
}

// NewNewsletterService 创建新闻通讯服务
func NewNewsletterService(cfg *rss.Config) *NewsletterService {
	return &NewsletterService{
		config:      cfg,
		failedFeeds: make([]*rss.FeedError, 0),
	}
}

// GetFailedFeeds returns the list of failed feeds
func (s *NewsletterService) GetFailedFeeds() []*rss.FeedError {
	return s.failedFeeds
}

// TemplateData represents the data passed to the template
type TemplateData struct {
	DashboardHTML string
	Feeds         []feeds.RssFeed
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

	// 使用handleDashboard生成dashboard HTML
	dashboardHTML := rss.GenerateDashboardHTML(config, service.GetFailedFeeds())

	content, err := service.RenderNewsletter(f, dashboardHTML)
	if err != nil {
		return err
	}

	if config.EnvConfig.Debug {
		slog.Info("HTML写入成功")
		return os.WriteFile("newsletter.html", []byte(content), os.ModePerm)
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
	// 创建一个带超时的context
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// 创建一个结果池，用于收集处理结果
	p := pool.NewWithResults[feeds.RssFeed]().
		WithContext(ctx).
		WithMaxGoroutines(10) // 限制最大并发数

	// 提交所有任务到池中
	for _, feed := range s.config.Feeds {
		feed := feed // 避免闭包问题
		p.Go(func(ctx context.Context) (feeds.RssFeed, error) {
			rssFeed, err := s.processSingleFeed(ctx, feed)
			if err != nil {
				slog.Error("Failed to process feed",
					slog.String("type", feed.Type),
					slog.Any("error", err))
				// 记录失败的feed
				s.failedFeeds = append(s.failedFeeds, &rss.FeedError{
					URL:     feed.Urls[0].Feed,
					Message: "Failed to fetch feed",
				})
				// 返回一个空的feed而不是错误，这样其他feed可以继续处理
				return feeds.RssFeed{
					Category: feed.Type,
					Items:    []*feeds.RssItem{},
				}, nil
			}
			return rssFeed, nil
		})
	}

	// 等待所有任务完成并收集结果
	results, err := p.Wait()
	if err != nil {
		slog.Error("Error processing feeds", slog.Any("error", err))
	}

	return results, nil
}

// processSingleFeed 处理单个Feed源
func (s *NewsletterService) processSingleFeed(ctx context.Context, feed rss.FeedsDetail) (feeds.RssFeed, error) {
	urls := lo.Compact(lo.Map(feed.Urls, func(item rss.Feeds, _ int) string {
		return item.Feed
	}))

	allFeeds, failedFeeds := rss.FetchURLs(ctx, urls, s.config)
	// 记录所有失败的feed
	s.failedFeeds = append(s.failedFeeds, failedFeeds...)

	if len(allFeeds) == 0 {
		slog.Info("No feeds fetched for category",
			slog.String("category", feed.Type),
			slog.Int("total_urls", len(urls)))
		return feeds.RssFeed{
			Category: feed.Type,
			Items:    []*feeds.RssItem{},
		}, nil
	}

	combinedFeed, err := rss.MergeAllFeeds(feed.Type, allFeeds, s.config)
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
	if !s.config.NewsletterConfig.IsHideAuthorInTitle && item.Author.Name != "" {
		return fmt.Sprintf("[%s] %s", item.Author.Name, item.Title)
	}
	return item.Title
}

// RenderNewsletter 渲染邮件模板
func (s *NewsletterService) RenderNewsletter(feeds []feeds.RssFeed, dashboardHTML string) (string, error) {
	// 创建自定义的模板函数映射
	funcMap := template.FuncMap{
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
	}

	// 使用 New 创建模板，并添加自定义函数
	tmpl := template.New("newsletter.tpl").Funcs(funcMap)

	// 解析模板文件
	tmpl, err := tmpl.ParseFS(newsletterTpl, "templates/newsletter.tpl")
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	data := TemplateData{
		Feeds:         feeds,
		DashboardHTML: dashboardHTML,
	}

	var tplBytes bytes.Buffer
	if err := tmpl.Execute(&tplBytes, data); err != nil {
		return "", fmt.Errorf("failed to render template: %w", err)
	}

	return tplBytes.String(), nil
}

// SendNewsletter 发送邮件
func (s *NewsletterService) SendNewsletter(content string) error {
	emailCfg := EmailConfig{
		From:  "Acme <onboarding@resend.dev>",
		To:    []string{"jeffcottlu@gmail.com"},
		Token: s.config.ResendConfig.Token,
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
		return fmt.Errorf("failed to send email: %w", err)
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
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "rss2newsletter.yml", "config file (default is rss2newsletter.yml)")
}
