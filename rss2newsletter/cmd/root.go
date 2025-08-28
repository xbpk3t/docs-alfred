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

	mjml "github.com/Boostport/mjml-go"
	carbon "github.com/dromara/carbon/v2"
	"github.com/gorilla/feeds"
	resend "github.com/resend/resend-go/v2"
	"github.com/samber/lo"
	"github.com/sourcegraph/conc/pool"
	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/pkg/rss"
)

// Config holds the configuration for the application
type Config struct {
	CfgFile string
}

//go:embed templates/newsletter.mjml
var templates embed.FS

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

// TemplateData represents the data passed to the template
type TemplateData struct {
	DashboardData struct {
		FailedFeeds []*rss.FeedError
		FeedDetails []rss.FeedsDetail
	}
	Feeds           []feeds.RssFeed
	WeekNumber      int
	DashboardConfig rss.DashboardConfig
}

// EmailContent represents a single email content
type EmailContent struct {
	Subject string
	Content string
}

type TemplateType string

const (
	DashboardTpl  TemplateType = "Dashboard For Newsletter"
	NewsletterTpl TemplateType = "Newsletter"
)

// newRootCmd creates and returns the root command
func newRootCmd(cfg *Config) *cobra.Command {
	return &cobra.Command{
		Use:   "rss2newsletter",
		Short: "RSS订阅转换为邮件推送工具",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNewsletter(cfg, cmd, args)
		},
	}
}

func runNewsletter(cfg *Config, _ *cobra.Command, _ []string) error {
	config, err := loadConfig(cfg.CfgFile)
	if err != nil {
		return err
	}

	service := NewNewsletterService(config)
	f, err := service.ProcessAllFeeds()
	if err != nil {
		return err
	}

	var contents []EmailContent

	// 生成并添加主要的newsletter内容
	newsletterContent, err := service.RenderNewsletter(f, config.Feeds, service.failedFeeds)
	if err != nil {
		return err
	}
	contents = append(contents, EmailContent{
		Subject: service.generateEmailSubject(NewsletterTpl),
		Content: newsletterContent,
	})

	return service.handleOutput(contents)
}

func loadConfig(cfgFile string) (*rss.Config, error) {
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
		// 避免闭包问题
		p.Go(func(ctx context.Context) (feeds.RssFeed, error) {
			rssFeed, err := s.processSingleFeed(ctx, feed)
			if err != nil {
				slog.Error("Failed to process feed",
					slog.String("type", feed.Type),
					slog.Any("error", err))
				// 记录失败的feed
				s.failedFeeds = append(s.failedFeeds, &rss.FeedError{
					URL:     feed.URLs[0].Feed,
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
	urls := lo.Compact(lo.Map(feed.URLs, func(item rss.Feeds, _ int) string {
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

// renderTemplate renders a specific template with data
func (s *NewsletterService) renderTemplate(templateName string, data any) (string, error) {
	// Create a custom template function map
	funcMap := template.FuncMap{
		"safeHTML": func(s string) template.HTML {
			// Only allow safe HTML for trusted content
			// In production, consider using a proper HTML sanitizer
			return template.HTML(s) // #nosec G203 - We trust our template content
		},
	}

	// Use New to create a template and add custom functions
	tmpl := template.New(templateName).Funcs(funcMap)

	// Parse the template file
	tmpl, err := tmpl.ParseFS(templates, "templates/"+templateName)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var tplBytes bytes.Buffer
	if err := tmpl.Execute(&tplBytes, data); err != nil {
		return "", fmt.Errorf("failed to render template: %w", err)
	}

	// If this is an MJML template, convert it to HTML
	if templateName == "newsletter.mjml" {
		htmlOutput, err := mjml.ToHTML(context.Background(), tplBytes.String(),
			mjml.WithMinify(true),
			mjml.WithBeautify(true),
			mjml.WithValidationLevel("soft"),
		)
		if err != nil {
			return "", fmt.Errorf("failed to convert MJML to HTML: %w", err)
		}
		return htmlOutput, nil
	}

	return tplBytes.String(), nil
}

// RenderNewsletter renders the newsletter template
func (s *NewsletterService) RenderNewsletter(
	feeds []feeds.RssFeed,
	feedList []rss.FeedsDetail,
	failedFeeds []*rss.FeedError,
) (string, error) {
	now := carbon.Now()
	data := TemplateData{
		WeekNumber:      now.WeekOfYear(),
		Feeds:           feeds,
		DashboardConfig: s.config.DashboardConfig,
		DashboardData: struct {
			FailedFeeds []*rss.FeedError
			FeedDetails []rss.FeedsDetail
		}{
			FailedFeeds: failedFeeds,
			FeedDetails: feedList,
		},
	}
	return s.renderTemplate("newsletter.mjml", data)
}

// handleOutput 处理输出（写入文件或发送邮件）
func (s *NewsletterService) handleOutput(contents []EmailContent) error {
	if s.config.EnvConfig.Debug {
		// 写入到本地文件
		for i, content := range contents {
			filename := fmt.Sprintf("newsletter_%d.html", i+1)
			if err := os.WriteFile(filename, []byte(content.Content), 0o600); err != nil {
				return fmt.Errorf("failed to write file %s: %w", filename, err)
			}
			slog.Info("HTML写入成功", "filename", filename)
		}
		return nil
	}

	// 发送邮件
	for _, content := range contents {
		if err := s.SendNewsletter(content.Content, content.Subject); err != nil {
			return fmt.Errorf("failed to send email: %w", err)
		}
	}
	return nil
}

// SendNewsletter 发送邮件
func (s *NewsletterService) SendNewsletter(content string, subject string) error {
	emailCfg := EmailConfig{
		From:  "Acme <onboarding@resend.dev>",
		To:    s.config.ResendConfig.MailTo,
		Token: s.config.ResendConfig.Token,
	}

	ctx := context.Background()
	client := resend.NewClient(emailCfg.Token)

	params := &resend.SendEmailRequest{
		From:    emailCfg.From,
		To:      emailCfg.To,
		Subject: subject,
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
func (s *NewsletterService) generateEmailSubject(tplType TemplateType) string {
	now := carbon.Now()
	return fmt.Sprintf("%s %s (第%d周)", tplType, now.ToDateString(), now.WeekOfYear())
}

// Execute 执行根命令
func Execute() {
	cfg := &Config{}
	rootCmd := newRootCmd(cfg)

	rootCmd.PersistentFlags().StringVar(
		&cfg.CfgFile, "config", "rss2newsletter.yml",
		"config file (default is rss2newsletter.yml)",
	)

	if err := rootCmd.Execute(); err != nil {
		slog.Error("执行命令失败", "error", err)
		os.Exit(1)
	}
}
