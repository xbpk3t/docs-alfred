package cmd

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"log/slog"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/linear2nl/internal"
	"github.com/xbpk3t/docs-alfred/linear2nl/linear"
)

//go:embed templates/morning.gohtml
var morningTemplates embed.FS

func newMorningCmd() *cobra.Command {
	return newReportCmd("morning", "Send morning report with today's tasks", runMorning)
}

func runMorning(cfg *internal.Config, dryRun bool) error {
	ctx := context.Background()
	client := linear.NewClient(cfg.Linear.APIKey, cfg.Linear.TeamKeys)
	aiClient := internal.NewAIProvider(cfg.AI)

	issues, err := queryMorningIssues(ctx, client, cfg)
	if err != nil {
		return err
	}

	if len(issues) == 0 {
		return sendBriefEmptyEmail(cfg, "🌅 Linear 今日任务", "今天没有待办任务 🎉", dryRun)
	}

	cat := categorizeIssues(issues)
	inProgress, todo := cat.inProgress, cat.todo

	var truncatedNote string
	if len(todo) > 15 {
		truncatedNote = fmt.Sprintf("还有 %d 个低优先级未显示", len(todo)-15)
		todo = todo[:15]
	}

	inProgressViews := toIssueViews(inProgress)
	todoViews := toIssueViews(todo)

	allViews := make([]internal.IssueView, 0, len(inProgressViews)+len(todoViews))
	allViews = append(allViews, inProgressViews...)
	allViews = append(allViews, todoViews...)

	aiSummary := buildMorningAISummary(aiClient, allViews, truncatedNote)

	now := time.Now().In(cst)
	data := internal.MorningData{
		Date:       now.Format("2006-01-02"),
		DayOfWeek:  formatWeekday(now),
		Theme:      cfg.Theme,
		InProgress: inProgressViews,
		Todo:       todoViews,
		AISummary:  aiSummary,
		Stats: internal.MorningStats{
			InProgress: len(inProgress),
			Todo:       len(todo),
			DueToday:   dueTodayCount(allViews),
		},
	}

	// Render template
	tmpl, err := template.New("morning.gohtml").Funcs(tmplFuncs()).ParseFS(morningTemplates, "templates/morning.gohtml")
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}
	htmlBody, err := renderHTML(tmpl, "morning.gohtml", data)
	if err != nil {
		return fmt.Errorf("render template: %w", err)
	}

	subject := fmt.Sprintf("🌅 Linear 今日任务 · %s %s", data.Date, data.DayOfWeek)

	return sendOrWrite(cfg, subject, htmlBody, "morning", dryRun)
}

func queryMorningIssues(ctx context.Context, client *linear.Client, cfg *internal.Config) ([]linear.Issue, error) {
	var issues []linear.Issue
	var err error

	if cfg.Morning.Strategy == "focused" {
		today := time.Now().In(cst).Format("2006-01-02")
		slog.Info("using focused strategy", "date", today)
		issues, err = client.GetFocusedIssues(ctx, today)
	} else {
		slog.Info("using all_assigned strategy")
		issues, err = client.GetActiveIssues(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("query issues: %w", err)
	}

	slog.Info("fetched issues", "count", len(issues))

	return issues, nil
}

type categorizedIssues struct {
	inProgress []linear.Issue
	todo       []linear.Issue
}

func categorizeIssues(issues []linear.Issue) categorizedIssues {
	var cat categorizedIssues
	for i := range issues {
		if issues[i].StateType == "started" {
			cat.inProgress = append(cat.inProgress, issues[i])
		} else {
			cat.todo = append(cat.todo, issues[i])
		}
	}

	return cat
}

func buildMorningAISummary(aiClient *internal.AIProvider, allViews []internal.IssueView, truncatedNote string) string {
	summary := aiClient.MorningSummary(allViews)
	if summary == "" && aiClient.IsConfigured() {
		summary = "AI 总结暂不可用"
	}
	if truncatedNote != "" {
		if summary != "" {
			summary += "\n\n⚠️ " + truncatedNote
		} else {
			summary = "⚠️ " + truncatedNote
		}
	}

	return summary
}

func dueTodayCount(issues []internal.IssueView) int {
	today := time.Now().In(cst).Format("2006-01-02")
	count := 0
	for _, iss := range issues {
		if strings.HasPrefix(iss.DueDate, today) {
			count++
		}
	}

	return count
}

func sendBriefEmptyEmail(cfg *internal.Config, subject, body string, dryRun bool) error {
	now := time.Now().In(cst)
	fullSubject := fmt.Sprintf("%s · %s %s", subject, now.Format("2006-01-02"), formatWeekday(now))
	html := fmt.Sprintf(`<!doctype html><html lang="zh-CN"><body style="font-family:sans-serif;padding:24px;">
		<h1>%s</h1><p>%s</p></body></html>`, fullSubject, body)

	return sendOrWrite(cfg, fullSubject, html, "morning-empty", dryRun)
}

func sendOrWrite(cfg *internal.Config, subject, htmlBody, suffix string, dryRun bool) error {
	if dryRun {
		return writeHTML(htmlBody, suffix)
	}

	return sendEmail(cfg, subject, htmlBody)
}
