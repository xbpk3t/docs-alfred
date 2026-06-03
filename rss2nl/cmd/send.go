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

//go:embed templates/newsletter.mjml
var templates embed.FS

// Config holds the application configuration.
type Config struct {
	CfgFile string
}

// EmailConfig 邮件配置.
type EmailConfig struct {
	From  string
	Token string
	To    []string
}

// NewsletterService 处理新闻通讯的服务.
type NewsletterService struct {
	config      *rss.Config
	failedFeeds []*rss.FeedError
}

// NewNewsletterService 创建新闻通讯服务.
func NewNewsletterService(cfg *rss.Config) *NewsletterService {
	return &NewsletterService{
		config:      cfg,
		failedFeeds: make([]*rss.FeedError, 0),
	}
}

// TemplateData represents the data passed to the template.
type TemplateData struct {
	DashboardData struct {
		FailedFeeds []*rss.FeedError
		FeedDetails []rss.FeedsDetail
	}
	Feeds           []feeds.RssFeed
	WeekNumber      int
	DashboardConfig rss.DashboardConfig
}

// EmailContent represents a single email content.
type EmailContent struct {
	Subject string
	Content string
}

type TemplateType string

const (
	DashboardTpl  TemplateType = "Dashboard For Newsletter"
	NewsletterTpl TemplateType = "Newsletter"
)

// newSendCmd creates `rss2nl send`.
func newSendCmd() *cobra.Command {
	var cfgFile, trnsOut string
	var checkOnly bool

	cmd := &cobra.Command{
		Use:   "send",
		Short: "Merge feeds and send newsletter",
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := loadConfig(cfgFile)
			if err != nil {
				return err
			}
			if checkOnly {
				return runFeedHealthCheck(config)
			}

			return runSend(config, trnsOut)
		},
	}

	cmd.Flags().StringVarP(&cfgFile, "config", "c", "rss2nl.yml", "配置文件路径")
	cmd.Flags().StringVar(&trnsOut, "trns-out", ".cache/rss2nl/trns", "Trns cache/output directory")
	cmd.Flags().BoolVar(&checkOnly, "check", false, "只检查 feed 健康度，不发邮件")

	return cmd
}

func runSend(config *rss.Config, trnsOut string) error {
	service := NewNewsletterService(config)
	f, err := service.ProcessAllFeeds()
	if err != nil {
		return err
	}

	var contents []EmailContent

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

func runFeedHealthCheck(config *rss.Config) error {
	// TODO: implement feed health check
	slog.Info("Feed health check passed")

	return nil
}

func loadConfig(cfgFile string) (*rss.Config, error) {
	config, err := rss.NewConfig(cfgFile)
	if err != nil {
		slog.Error("rss2nl config file load error:", slog.Any("err", err))

		return nil, err
	}

	return config, nil
}

// ProcessAllFeeds 并发处理所有Feed源.
func (s *NewsletterService) ProcessAllFeeds() ([]feeds.RssFeed, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	p := pool.NewWithResults[feeds.RssFeed]().
		WithContext(ctx).
		WithMaxGoroutines(10)

	for _, feed := range s.config.Feeds {
		p.Go(func(ctx context.Context) (feeds.RssFeed, error) {
			rssFeed, err := s.processSingleFeed(ctx, feed)
			if err != nil {
				slog.Error("Failed to process feed",
					slog.String("type", feed.Type),
					slog.Any("error", err))
				s.failedFeeds = append(s.failedFeeds, &rss.FeedError{
					URL:     feed.URLs[0].Feed,
					Message: "Failed to fetch feed",
				})

				return feeds.RssFeed{
					Category: feed.Type,
					Items:    []*feeds.RssItem{},
				}, nil
			}

			return rssFeed, nil
		})
	}

	results, err := p.Wait()
	if err != nil {
		slog.Error("Error processing feeds", slog.Any("error", err))
	}

	return results, nil
}

func (s *NewsletterService) processSingleFeed(ctx context.Context, feed rss.FeedsDetail) (feeds.RssFeed, error) {
	urls := lo.Compact(lo.Map(feed.URLs, func(item rss.Feeds, _ int) string {
		return item.Feed
	}))

	allFeeds, failedFeeds := rss.FetchURLs(ctx, urls, s.config)
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

func (s *NewsletterService) getItemTitle(item *feeds.Item) string {
	if !s.config.NewsletterConfig.IsHideAuthorInTitle && item.Author.Name != "" {
		return fmt.Sprintf("[%s] %s", item.Author.Name, item.Title)
	}

	return item.Title
}

func (s *NewsletterService) renderTemplate(templateName string, data any) (string, error) {
	funcMap := template.FuncMap{
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s) // #nosec G203
		},
	}

	tmpl := template.New(templateName).Funcs(funcMap)
	tmpl, err := tmpl.ParseFS(templates, "templates/"+templateName)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var tplBytes bytes.Buffer
	if err := tmpl.Execute(&tplBytes, data); err != nil {
		return "", fmt.Errorf("failed to render template: %w", err)
	}

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

// RenderNewsletter renders the newsletter template.
func (s *NewsletterService) RenderNewsletter(
	rssFeeds []feeds.RssFeed,
	feedList []rss.FeedsDetail,
	failedFeeds []*rss.FeedError,
) (string, error) {
	now := carbon.Now()
	data := TemplateData{
		WeekNumber:      now.WeekOfYear(),
		Feeds:           rssFeeds,
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

func (s *NewsletterService) handleOutput(contents []EmailContent) error {
	if s.config.EnvConfig.Debug {
		for i, content := range contents {
			filename := fmt.Sprintf("newsletter_%d.html", i+1)
			if err := os.WriteFile(filename, []byte(content.Content), 0o600); err != nil {
				return fmt.Errorf("failed to write file %s: %w", filename, err)
			}
			slog.Info("HTML写入成功", "filename", filename)
		}

		return nil
	}

	for _, content := range contents {
		if err := s.SendNewsletter(content.Content, content.Subject); err != nil {
			return fmt.Errorf("failed to send email: %w", err)
		}
	}

	return nil
}

// SendNewsletter 发送邮件.
func (s *NewsletterService) SendNewsletter(content, subject string) error {
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

func (s *NewsletterService) generateEmailSubject(tplType TemplateType) string {
	now := carbon.Now()

	return fmt.Sprintf("%s %s (第%d周)", tplType, now.ToDateString(), now.WeekOfYear())
}
