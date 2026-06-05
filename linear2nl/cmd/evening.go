package cmd

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/linear2nl/internal"
	"github.com/xbpk3t/docs-alfred/linear2nl/linear"
)

//go:embed templates/evening.gohtml
var eveningTemplates embed.FS

func newEveningCmd() *cobra.Command {
	return newReportCmd("evening", "Send evening report with today's accomplishments", runEvening)
}

func runEvening(cfg *internal.Config, dryRun bool) error {
	ctx := context.Background()
	client := linear.NewClient(cfg.Linear.APIKey, cfg.Linear.TeamKeys)
	aiClient := internal.NewAIProvider(cfg.AI)

	todayStart := time.Now().In(cst).Truncate(24 * time.Hour)

	eq, err := queryEveningData(ctx, client, todayStart)
	if err != nil {
		return err
	}

	completed, changes, inProgress, updatedDetails := eq.completed, eq.changes, eq.inProgress, eq.updatedDetails

	if len(completed) == 0 && len(changes) == 0 {
		return sendBriefEveningEmpty(cfg, dryRun)
	}

	relevantDetails := filterActiveDetails(completed, changes, updatedDetails)
	completedViews := toIssueViews(completed)
	changeViews := toStateChangeViews(changes)
	attachPerIssueReviews(aiClient, relevantDetails, completedViews, changeViews)

	now := time.Now().In(cst)
	data := internal.EveningData{
		Date:         now.Format("2006-01-02"),
		DayOfWeek:    formatWeekday(now),
		Theme:        cfg.Theme,
		Completed:    completedViews,
		StateChanges: changeViews,
		Stats: internal.EveningStats{
			Completed:  len(completed),
			InProgress: len(inProgress),
		},
	}

	tmpl, err := template.New("evening.gohtml").Funcs(tmplFuncs()).ParseFS(eveningTemplates, "templates/evening.gohtml")
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	html, err := renderHTML(tmpl, "evening.gohtml", data)
	if err != nil {
		return fmt.Errorf("render template: %w", err)
	}

	subject := fmt.Sprintf("🌙 Linear 今日收获 · %s %s", data.Date, data.DayOfWeek)

	return sendOrWrite(cfg, subject, html, "evening", dryRun)
}

type eveningData struct {
	completed      []linear.Issue
	changes        []linear.StateChange
	inProgress     []linear.Issue
	updatedDetails []linear.IssueDetail
}

func queryEveningData(ctx context.Context, client *linear.Client, todayStart time.Time) (eveningData, error) {
	completed, err := client.GetCompletedTodayIssues(ctx, todayStart)
	if err != nil {
		return eveningData{}, fmt.Errorf("query completed issues: %w", err)
	}
	slog.Info("fetched completed issues", "count", len(completed))

	changes, err := client.GetStateChanges(ctx, todayStart)
	if err != nil {
		slog.Warn("query state changes failed", "error", err)
		changes = nil
	}
	slog.Info("fetched state changes", "count", len(changes))

	inProgress, err := client.GetInProgressIssues(ctx)
	if err != nil {
		slog.Warn("query in-progress issues failed", "error", err)
		inProgress = nil
	}
	slog.Info("fetched in-progress issues", "count", len(inProgress))

	updatedDetails, err := client.GetUpdatedIssuesWithDetails(ctx, todayStart)
	if err != nil {
		slog.Warn("query updated issues with details failed", "error", err)
		updatedDetails = nil
	}
	slog.Info("fetched issue details for AI review", "count", len(updatedDetails))

	return eveningData{
		completed:      completed,
		changes:        changes,
		inProgress:     inProgress,
		updatedDetails: updatedDetails,
	}, nil
}

func filterActiveDetails(completed []linear.Issue, changes []linear.StateChange, updatedDetails []linear.IssueDetail) []linear.IssueDetail {
	activeIDs := make(map[string]bool)
	for i := range completed {
		activeIDs[completed[i].Identifier] = true
	}
	for i := range changes {
		activeIDs[changes[i].IssueIdentifier] = true
	}

	var relevant []linear.IssueDetail
	for i := range updatedDetails {
		if activeIDs[updatedDetails[i].Identifier] {
			relevant = append(relevant, updatedDetails[i])
		}
	}

	return relevant
}

func attachPerIssueReviews(
	aiClient *internal.AIProvider, details []linear.IssueDetail,
	completedViews []internal.IssueView, changeViews []internal.StateChangeView,
) {
	reviewMap := buildPerIssueReviews(aiClient, details)
	for i := range completedViews {
		if r, ok := reviewMap[completedViews[i].Identifier]; ok {
			completedViews[i].Review = template.HTML(r) //nolint:gosec // G203: AI-generated HTML for trusted template
		}
	}
	for i := range changeViews {
		if r, ok := reviewMap[changeViews[i].IssueIdentifier]; ok {
			changeViews[i].Review = template.HTML(r) //nolint:gosec // G203: AI-generated HTML for trusted template
		}
	}
}

// buildPerIssueReviews generates AI review for the given issues and parses
// the response into a per-issue map keyed by issue identifier.
// Returns nil if AI is unavailable or no issues are provided.
func buildPerIssueReviews(aiClient *internal.AIProvider, details []linear.IssueDetail) map[string]string {
	if len(details) == 0 || !aiClient.IsConfigured() {
		return nil
	}

	raw := aiClient.EveningDeepReview(toIssueDetails(details))
	if raw == "" {
		return nil
	}

	return parsePerIssueReview(raw)
}

// parsePerIssueReview splits the AI response by "### IDENTIFIER" headers
// into a map keyed by issue identifier.
func parsePerIssueReview(raw string) map[string]string {
	chunks := regexp.MustCompile(`(?m)^###\s+`).Split(raw, -1)
	result := make(map[string]string, len(chunks)-1)
	for _, chunk := range chunks {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}
		// First line is "IDENTIFIER: title" — extract identifier.
		lines := strings.SplitN(chunk, "\n", 2)
		identifier := strings.TrimSpace(strings.SplitN(lines[0], ":", 2)[0])
		if identifier == "" {
			continue
		}
		body := strings.TrimSpace(chunk)
		if body != "" {
			result[identifier] = markdownToHTML(body)
		}
	}

	return result
}

func toIssueDetails(issues []linear.IssueDetail) []internal.IssueDetail {
	details := make([]internal.IssueDetail, len(issues))
	for i := range issues {
		iss := &issues[i]
		comments := make([]internal.Comment, len(iss.Comments))
		for j, c := range iss.Comments {
			comments[j] = internal.Comment{
				Body:      c.Body,
				UserName:  c.UserName,
				CreatedAt: c.CreatedAt,
			}
		}
		details[i] = internal.IssueDetail{
			Identifier:  iss.Identifier,
			Title:       iss.Title,
			Description: iss.Description,
			StateName:   iss.StateName,
			TeamName:    iss.TeamName,
			Comments:    comments,
			URL:         iss.URL,
		}
	}

	return details
}

func sendBriefEveningEmpty(cfg *internal.Config, dryRun bool) error {
	now := time.Now().In(cst)
	subject := fmt.Sprintf("🌙 Linear 今日收获 · %s %s", now.Format("2006-01-02"), formatWeekday(now))
	html := `<!doctype html><html lang="zh-CN"><body style="font-family:sans-serif;padding:24px;">
		<h1>` + subject + `</h1><p>今天没有完成记录 🎉</p></body></html>`

	return sendOrWrite(cfg, subject, html, "evening-empty", dryRun)
}
