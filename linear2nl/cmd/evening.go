package cmd

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/linear2nl/internal"
	"github.com/xbpk3t/docs-alfred/linear2nl/linear"
	"github.com/yuin/goldmark"
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
	summaryHTML := attachPerIssueReviews(aiClient, relevantDetails, completedViews, changeViews)

	now := time.Now().In(cst)
	data := internal.EveningData{
		Date:         now.Format("2006-01-02"),
		DayOfWeek:    formatWeekday(now),
		Theme:        cfg.Theme,
		AIReview:     summaryHTML,
		Completed:    completedViews,
		StateChanges: changeViews,
		Stats: internal.EveningStats{
			Completed:  len(completed),
			InProgress: len(inProgress),
		},
	}

	// Render template
	tmpl, err := template.New("evening.gohtml").Funcs(tmplFuncs()).ParseFS(eveningTemplates, "templates/evening.gohtml")
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}
	htmlBody, err := renderHTML(tmpl, "evening.gohtml", data)
	if err != nil {
		return fmt.Errorf("render template: %w", err)
	}

	subject := fmt.Sprintf("🌙 Linear 今日收获 · %s %s", data.Date, data.DayOfWeek)

	return sendOrWrite(cfg, subject, htmlBody, "evening", dryRun)
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

// perIssueReviewResult wraps the two return values from parsing AI review JSON.
type perIssueReviewResult struct {
	reviews     map[string]string
	summaryHTML string
}

func attachPerIssueReviews(
	aiClient *internal.AIProvider, details []linear.IssueDetail,
	completedViews []internal.IssueView, changeViews []internal.StateChangeView,
) string {
	r := buildPerIssueReviews(aiClient, details)
	if r == nil {
		return ""
	}
	for i := range completedViews {
		if review, ok := r.reviews[completedViews[i].Identifier]; ok {
			completedViews[i].Review = template.HTML(review) //nolint:gosec // G203: AI-generated HTML for trusted template
		}
	}
	for i := range changeViews {
		if review, ok := r.reviews[changeViews[i].IssueIdentifier]; ok {
			changeViews[i].Review = template.HTML(review) //nolint:gosec // G203: AI-generated HTML for trusted template
		}
	}

	return r.summaryHTML
}

// buildPerIssueReviews generates AI review for the given issues and parses
// the response into a per-issue map keyed by issue identifier and the summary HTML.
// Returns nil if AI is unavailable or no issues are provided.
func buildPerIssueReviews(aiClient *internal.AIProvider, details []linear.IssueDetail) *perIssueReviewResult {
	if len(details) == 0 || !aiClient.IsConfigured() {
		return nil
	}

	raw := aiClient.EveningDeepReview(toIssueDetails(details))
	if raw == "" {
		slog.Warn("AI returned empty response")

		return nil
	}
	slog.Info("AI raw response preview", "len", len(raw), "raw", raw[:min(len(raw), 2000)])

	return parsePerIssueReviewJSON(raw)
}

// AIReviewJSON is the expected JSON structure from the AI evening deep review.
type AIReviewJSON struct {
	Reviews []AIReviewItemJSON `json:"reviews"`
	Summary []string           `json:"summary"`
}

// AIReviewItemJSON is a single issue review item in the JSON response.
type AIReviewItemJSON struct {
	Identifier string   `json:"identifier"`
	Title      string   `json:"title"`
	Progress   []string `json:"progress"`
	Knowledge  []string `json:"knowledge"`
	Review     []string `json:"review"`
}

// parsePerIssueReviewJSON parses the AI response as JSON and returns
// a perIssueReviewResult with the per-issue review map and summary HTML.
func parsePerIssueReviewJSON(raw string) *perIssueReviewResult {
	result, err := parseAIReviewJSON(raw)
	if err != nil {
		return nil
	}

	reviews := make(map[string]string, len(result.Reviews))
	for _, r := range result.Reviews {
		var sb strings.Builder
		renderReviewSection(&sb, "决策/进展", r.Progress)
		renderReviewSection(&sb, "💡 知识点", r.Knowledge)
		renderReviewSection(&sb, "📊 Review", r.Review)
		reviews[r.Identifier] = sb.String()
	}

	// Render summary
	var summaryHTML strings.Builder
	summaryHTML.WriteString(`<div class="ai-summary">`)
	for _, s := range result.Summary {
		summaryHTML.WriteString(markdownParagraph(s))
	}
	summaryHTML.WriteString(`</div>`)

	return &perIssueReviewResult{reviews: reviews, summaryHTML: summaryHTML.String()}
}

// parseAIReviewJSON attempts to unmarshal the raw string as AIReviewJSON.
// Falls back to extracting the outermost {...} if wrapped in code fences.
func parseAIReviewJSON(raw string) (*AIReviewJSON, error) {
	result, err := tryParseJSON(raw)
	if err == nil {
		return result, nil
	}
	slog.Warn("failed to parse JSON directly, trying brace extraction", "error", err)
	extracted := extractJSONBraces(raw)
	if extracted == "" {
		slog.Warn("brace extraction failed to find JSON object")

		return nil, err
	}
	result, err = tryParseJSON(extracted)
	if err != nil {
		slog.Warn("brace extraction also failed to parse", "error", err)

		return nil, err
	}

	return result, nil
}

// renderReviewSection appends a section heading and its paragraph items to sb.
func renderReviewSection(sb *strings.Builder, heading string, items []string) {
	if len(items) == 0 {
		return
	}
	sb.WriteString("<h3>")
	sb.WriteString(heading)
	sb.WriteString("</h3>")
	for _, item := range items {
		sb.WriteString(markdownParagraph(item))
	}
}

// tryParseJSON attempts to unmarshal the raw string as AIReviewJSON.
func tryParseJSON(raw string) (*AIReviewJSON, error) {
	var result AIReviewJSON
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// extractJSONBraces finds the outermost balanced {...} block in the string.
// This handles cases where the AI wraps JSON in markdown code fences or other text.
func extractJSONBraces(s string) string {
	start := strings.Index(s, "{")
	if start < 0 {
		return ""
	}
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}

	return ""
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

// markdownParagraph converts a paragraph of markdown text to HTML.
// Uses goldmark and strips the document wrapper, keeping <p> tags and inline formatting.
func markdownParagraph(s string) string {
	if s == "" {
		return ""
	}
	var buf bytes.Buffer
	if err := goldmark.New().Convert([]byte(s), &buf); err != nil {
		return "<p>" + template.HTMLEscapeString(s) + "</p>"
	}
	full := buf.String()
	// goldmark wraps output in <html><head></head><body>...</body></html>
	const bodyOpen = "<body>"
	const bodyClose = "</body>"
	start := strings.Index(full, bodyOpen)
	end := strings.LastIndex(full, bodyClose)
	if start >= 0 && end > start {
		return full[start+len(bodyOpen) : end]
	}

	return full
}

func sendBriefEveningEmpty(cfg *internal.Config, dryRun bool) error {
	now := time.Now().In(cst)
	subject := fmt.Sprintf("🌙 Linear 今日收获 · %s %s", now.Format("2006-01-02"), formatWeekday(now))
	html := `<!doctype html><html lang="zh-CN"><body style="font-family:sans-serif;padding:24px;">
		<h1>` + subject + `</h1><p>今天没有完成记录 🎉</p></body></html>`

	return sendOrWrite(cfg, subject, html, "evening-empty", dryRun)
}
