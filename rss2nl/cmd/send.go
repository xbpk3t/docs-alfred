package cmd

import (
	"bytes"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"html/template"
	"log/slog"
	"os"
	"strings"
	"time"

	carbon "github.com/dromara/carbon/v2"
	"github.com/mmcdole/gofeed"
	resend "github.com/resend/resend-go/v2"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
	"github.com/xbpk3t/docs-alfred/service/rss"
	"golang.org/x/sync/errgroup"
)

//go:embed templates/*.gohtml
var templates embed.FS

// EmailConfig 邮件配置.
type EmailConfig struct {
	From  string
	Token string
	To    []string
}

// NewsletterItem represents a rich feed item with additional fields.
type NewsletterItem struct {
	Title              string
	Link               string
	PubDate            string
	Description        string
	Content            string
	EnclosureURL       string
	EnclosureType      string
	FeedTitle          string
	ItemHash           string
	TrnsURL            string
	PodcastTranscripts []PodcastTranscriptRef
}

// PodcastTranscriptRef represents a reference to a podcast transcript.
type PodcastTranscriptRef struct {
	URL  string
	Type string // plaintext, vtt, srt, json
}

// NewsletterCategory holds items grouped by category with extra metadata.
type NewsletterCategory struct {
	Category    string
	Items       []NewsletterItem
	FailedFeeds []*rss.FeedError
}

// TemplateData represents the data passed to the template.
type TemplateData struct {
	Title         string
	SourceHuntURL string
	DashboardData struct {
		FailedFeeds   []*rss.FeedError
		FailureReport *rss.FeedFailureReport
		FeedDetails   []rss.FeedsDetail
	}
	Feeds           []NewsletterCategory
	DashboardConfig rss.DashboardConfig
	WeekNumber      int
}

// EmailContent represents a single email content.
type EmailContent struct {
	Subject string
	Content string
}

type TemplateType string

const (
	DashboardTpl    TemplateType = "Dashboard For Newsletter"
	NewsletterTpl   TemplateType = "Newsletter"
	podcastCategory              = "podcast"
)

// NewsletterService 处理新闻通讯的服务.
type NewsletterService struct {
	config      *rss.Config
	trnsOut     string
	failedFeeds []*rss.FeedError
}

// NewNewsletterService 创建新闻通讯服务.
func NewNewsletterService(cfg *rss.Config, trnsOut string) *NewsletterService {
	return &NewsletterService{
		config:      cfg,
		trnsOut:     trnsOut,
		failedFeeds: make([]*rss.FeedError, 0),
	}
}

// -- Sub-command setup --

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
	cmd.Flags().StringVar(&trnsOut, "trns-out", fileutil.CachePath("rss2nl/trns"), "Trns cache/output directory")
	cmd.Flags().BoolVar(&checkOnly, "check", false, "只检查 feed 健康度，不发邮件")

	return cmd
}

// -- Run --

func runSend(config *rss.Config, trnsOut string) error {
	service := NewNewsletterService(config, trnsOut)
	categories, err := service.ProcessAllFeeds()
	if err != nil {
		return err
	}

	// Process transcripts for podcast items and set trns URLs
	if config.TrnsConfig.Enabled {
		for i := range categories {
			if !shouldProcessNewsletterTrns(categories[i].Category) {
				continue
			}
			report := ProcessNewsletterTrns(categories[i].Items, config, trnsOut)
			slog.Info("Newsletter trns completed",
				"category", categories[i].Category,
				"eligible", report.Eligible,
				"attempted", report.Attempted,
				"linked", report.Linked,
				"failed", report.Failed,
				"skippedNoMedia", report.SkippedNoMedia,
				"skippedByLimit", report.SkippedByLimit,
			)
		}
	}

	// Inject hunt source discovery link
	sourceHuntURL := os.Getenv("SOURCE_DISCOVERY_URL")

	contents, err := service.RenderNewsletter(categories, config.Feeds, service.failedFeeds, sourceHuntURL)
	if err != nil {
		return err
	}

	return service.handleOutput(contents)
}

func runFeedHealthCheck(config *rss.Config) error {
	slog.Info("=== Feed Health Check ===")

	healthyCount := 0
	staleCount := 0
	failedCount := 0
	staleThreshold := 90 * 24 * time.Hour // 3 months

	for _, feedGroup := range config.Feeds {
		for _, u := range feedGroup.URLs {
			if u.Feed == "" {
				continue
			}

			switch checkFeed(u, config, staleThreshold) {
			case healthOK:
				healthyCount++
			case healthStale:
				staleCount++
			case healthFailed:
				failedCount++
			}
		}
	}

	slog.Info("Health check complete",
		"healthy", healthyCount,
		"stale", staleCount,
		"failed", failedCount,
	)

	if failedCount > 0 {
		return fmt.Errorf("%d feed(s) failed to fetch", failedCount)
	}

	return nil
}

func loadConfig(cfgFile string) (*rss.Config, error) {
	config, err := rss.NewConfig(cfgFile)
	if err != nil {
		slog.Error("rss2nl config file load error:", slog.Any("err", err))

		return nil, err
	}

	// Fall back to RESEND_TOKEN env var if token not set in config file.
	// CI sets RESEND_TOKEN as a secret; local dev can set it in .env or shell.
	if config.ResendConfig.Token == "" {
		config.ResendConfig.Token = os.Getenv("RESEND_TOKEN")
	}

	if err := config.ValidateForSend(); err != nil {
		return nil, err
	}

	return config, nil
}

// -- Feed processing --

// ProcessAllFeeds 并发处理所有Feed源.
func (s *NewsletterService) ProcessAllFeeds() ([]NewsletterCategory, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(10)

	results := make([]NewsletterCategory, len(s.config.Feeds))
	for i, feed := range s.config.Feeds {
		g.Go(func() error {
			category, err := s.processSingleFeed(ctx, feed)
			if err != nil {
				slog.Error("Failed to process feed",
					slog.String("type", feed.Type),
					slog.Any("error", err))
				category.FailedFeeds = append(category.FailedFeeds, &rss.FeedError{
					URL:     firstFeedURL(feed),
					Message: "Failed to fetch feed",
					Err:     err,
				})
			}

			results[i] = category

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		slog.Error("Error processing feeds", slog.Any("error", err))

		return nil, err
	}

	s.failedFeeds = s.failedFeeds[:0]
	for _, category := range results {
		s.failedFeeds = append(s.failedFeeds, category.FailedFeeds...)
	}

	return results, nil
}

func (s *NewsletterService) processSingleFeed(ctx context.Context, feedGroup rss.FeedsDetail) (NewsletterCategory, error) {
	category := NewsletterCategory{Category: feedGroup.Type}
	urls := lo.Compact(lo.Map(feedGroup.URLs, func(item rss.Feeds, _ int) string {
		return item.Feed
	}))

	allFeeds, failedFeeds := rss.FetchURLs(ctx, urls, s.config)
	category.FailedFeeds = failedFeeds

	if len(allFeeds) == 0 {
		slog.Info("No feeds fetched for category",
			slog.String("category", feedGroup.Type),
			slog.Int("total_urls", len(urls)))

		return category, nil
	}

	items, err := s.mergeFeedItems(feedGroup.Type, allFeeds)
	if err != nil {
		slog.Error("Failed to merge feeds",
			slog.String("category", feedGroup.Type),
			slog.Int("feeds_count", len(allFeeds)),
			slog.Any("error", err))

		return category, nil
	}

	category.Items = items

	return category, nil
}

// mergeFeedItems merges all feed results into a deduplicated list of NewsletterItems.
func (s *NewsletterService) mergeFeedItems(typeName string, allFeeds []*gofeed.Feed) ([]NewsletterItem, error) {
	seenLinks := make(map[string]bool)
	seenHashes := make(map[string]bool)
	var items []NewsletterItem

	for _, sourceFeed := range allFeeds {
		for i, item := range sourceFeed.Items {
			if i >= s.config.FeedConfig.FeedLimit {
				break
			}

			// Dedup by link
			if seenLinks[item.Link] {
				continue
			}

			created := getItemCreationTime(item)
			if !rss.FilterFeedsWithTimeRange(created, time.Now(), s.config.NewsletterConfig.Schedule) {
				continue
			}

			// Generate sha256 identity
			feedURL := ""
			if sourceFeed.Link != "" {
				feedURL = sourceFeed.Link
			}
			itemHash := itemIdentity(feedURL, item)

			// Dedup by hash
			if seenHashes[itemHash] {
				continue
			}

			seenLinks[item.Link] = true
			seenHashes[itemHash] = true

			ni := s.makeNewsletterItem(item, sourceFeed, typeName, itemHash)
			items = append(items, ni)
		}
	}

	return items, nil
}

// makeNewsletterItem converts a gofeed.Item to a NewsletterItem.
func (s *NewsletterService) makeNewsletterItem(item *gofeed.Item, sourceFeed *gofeed.Feed, typeName, itemHash string) NewsletterItem {
	ni := NewsletterItem{
		Title:     s.getItemTitle(item),
		Link:      item.Link,
		PubDate:   carbon.CreateFromStdTime(getItemCreationTime(item)).ToDateTimeString(),
		FeedTitle: feedDisplayName(sourceFeed),
		ItemHash:  itemHash,
	}

	// Description / content
	if item.Description != "" {
		ni.Description = item.Description
	}
	if item.Content != "" {
		ni.Content = item.Content
	} else {
		ni.Content = item.Description
	}

	// Enclosure
	if len(item.Enclosures) > 0 {
		ni.EnclosureURL = item.Enclosures[0].URL
		ni.EnclosureType = item.Enclosures[0].Type
	}

	// Podcast transcripts from RSS extensions
	ni.PodcastTranscripts = extractTranscriptRefs(item)

	return ni
}

func shouldProcessNewsletterTrns(category string) bool {
	return strings.EqualFold(strings.TrimSpace(category), podcastCategory)
}

func feedDisplayName(feed *gofeed.Feed) string {
	if feed == nil {
		return ""
	}
	if feed.Title != "" {
		return feed.Title
	}

	return feed.Link
}

// extractTranscriptRefs extracts podcast:transcript references from item extensions.
func extractTranscriptRefs(item *gofeed.Item) []PodcastTranscriptRef {
	var refs []PodcastTranscriptRef
	for ns, exts := range item.Extensions {
		nsLower := strings.ToLower(ns)
		if !strings.Contains(nsLower, "podcast") && !strings.Contains(nsLower, "transcript") {
			continue
		}
		for tag, values := range exts {
			if !strings.Contains(strings.ToLower(tag), "transcript") {
				continue
			}
			for _, v := range values {
				ref := PodcastTranscriptRef{Type: "unknown"}
				if u, ok := v.Attrs["url"]; ok {
					ref.URL = u
				}
				if t, ok := v.Attrs["type"]; ok {
					ref.Type = t
				}
				if ref.URL != "" {
					refs = append(refs, ref)
				}
			}
		}
	}

	return refs
}

// itemIdentity generates a deterministic sha256 hash for an item.
func itemIdentity(feedURL string, item *gofeed.Item) string {
	idSource := item.GUID
	if idSource == "" {
		idSource = item.Link
	}
	if idSource == "" {
		idSource = item.Title
	}

	h := sha256.New()
	h.Write([]byte(feedURL))
	h.Write([]byte(idSource))

	return hex.EncodeToString(h.Sum(nil))
}

func getItemCreationTime(item *gofeed.Item) time.Time {
	if item.PublishedParsed != nil {
		return *item.PublishedParsed
	}
	if item.UpdatedParsed != nil {
		return *item.UpdatedParsed
	}

	return time.Now()
}

func (s *NewsletterService) getItemTitle(item *gofeed.Item) string {
	hasAuthor := item.Author != nil && item.Author.Name != ""
	if !s.config.NewsletterConfig.IsHideAuthorInTitle && hasAuthor {
		return fmt.Sprintf("[%s] %s", item.Author.Name, item.Title)
	}

	return item.Title
}

// -- Template rendering --

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

	return tplBytes.String(), nil
}

// RenderNewsletter renders the newsletter template.
func (s *NewsletterService) RenderNewsletter(
	categories []NewsletterCategory,
	feedList []rss.FeedsDetail,
	failedFeeds []*rss.FeedError,
	sourceHuntURL string,
) ([]EmailContent, error) {
	now := carbon.Now()
	subject := s.generateEmailSubject(NewsletterTpl)
	failureReport, reportErr := rss.BuildFeedFailureReport(
		failedFeeds,
		s.config.DashboardConfig.FetchFailureReport,
		time.Now(),
	)
	if reportErr != nil {
		slog.Warn("Failed to update feed failure report state", "error", reportErr)
	}
	data := TemplateData{
		Title:           subject,
		WeekNumber:      now.WeekOfYear(),
		Feeds:           categories,
		DashboardConfig: s.config.DashboardConfig,
		SourceHuntURL:   sourceHuntURL,
		DashboardData: struct {
			FailedFeeds   []*rss.FeedError
			FailureReport *rss.FeedFailureReport
			FeedDetails   []rss.FeedsDetail
		}{
			FailedFeeds:   failedFeeds,
			FailureReport: failureReport,
			FeedDetails:   feedList,
		},
	}

	newsletterContent, err := s.renderTemplate("newsletter.gohtml", data)
	if err != nil {
		return nil, err
	}

	contents := []EmailContent{
		{
			Subject: subject,
			Content: newsletterContent,
		},
	}

	return contents, nil
}

func firstFeedURL(feed rss.FeedsDetail) string {
	for _, u := range feed.URLs {
		if u.Feed != "" {
			return u.Feed
		}
	}

	return ""
}

func (s *NewsletterService) handleOutput(contents []EmailContent) error {
	if s.config.EnvConfig.Debug {
		for i, content := range contents {
			filename := fmt.Sprintf("newsletter_%d.html", i+1)
			if err := fileutil.AtomicWriteFile(filename, []byte(content.Content), fileutil.FilePermPrivate); err != nil {
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

type feedHealthStatus int

const (
	healthOK feedHealthStatus = iota
	healthStale
	healthFailed
)

func checkFeed(u rss.Feeds, config *rss.Config, staleThreshold time.Duration) feedHealthStatus {
	fp := gofeed.NewParser()
	fp.UserAgent = rss.DefaultUserAgent
	fp.Client = rss.NewHTTPClient(config)

	parsed, err := fp.ParseURL(u.Feed)
	if err != nil {
		slog.Warn("FAILED", "feed", u.Feed, "error", err)

		return healthFailed
	}

	latest := time.Now()
	if len(parsed.Items) > 0 && parsed.Items[0].PublishedParsed != nil {
		latest = *parsed.Items[0].PublishedParsed
	} else if len(parsed.Items) > 0 && parsed.Items[0].UpdatedParsed != nil {
		latest = *parsed.Items[0].UpdatedParsed
	}

	age := time.Since(latest)
	items := len(parsed.Items)
	status := "OK"
	var result feedHealthStatus
	if age > staleThreshold {
		status = "STALE"
		result = healthStale
	} else {
		result = healthOK
	}

	slog.Info("HEALTH",
		"feed", u.Feed,
		"title", parsed.Title,
		"items", items,
		"latest", latest.Format("2006-01-02"),
		"age_days", int(age.Hours()/24),
		"status", status,
	)

	return result
}
