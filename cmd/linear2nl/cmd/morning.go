package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	carbon "github.com/dromara/carbon/v2"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/cmd/linear2nl/internal"
	"github.com/xbpk3t/docs-alfred/internal/linear"
	"github.com/xbpk3t/docs-alfred/pkg/md"
)

func newMorningCmd() *cobra.Command {
	return newReportCmd("morning", "Send morning report with today's tasks", runMorning)
}

func runMorning(cfg *internal.Config, dryRun bool) error {
	ctx := context.Background()
	client := linear.NewClient(cfg.Linear.APIKey, cfg.Linear.TeamKeys)
	aiClient := internal.NewAIProvider(cfg.AI)

	details, err := client.GetActiveIssuesWithDetails(ctx)
	if err != nil {
		return fmt.Errorf("query active issues with details: %w", err)
	}

	if len(details) == 0 {
		return sendBriefEmptyEmail(cfg, "Linear 今日任务", "今天没有待办任务", dryRun)
	}

	issueDetails := toIssueDetails(details)
	issueViews := toIssueViewsFromDetails(details)

	// Truncate to 15 items if too many.
	var truncatedNote string
	if len(issueViews) > 15 {
		truncatedNote = fmt.Sprintf("还有 %d 个低优先级未显示", len(issueViews)-15)
		issueDetails = issueDetails[:15]
		issueViews = issueViews[:15]
	}

	// AI plan (single stage).
	plans := buildMorningPlan(aiClient, issueDetails)

	now := carbon.Now()
	dateStr := now.ToDateString()
	dayOfWeek := now.ToShortWeekString()

	// Render document.
	doc := md.NewDocument()
	doc.Add(md.NamedSection(fmt.Sprintf("Linear 今日任务 · %s %s", dateStr, dayOfWeek)))

	headers := []string{"ID", "Title", "Team"}
	var rows [][]string
	for i := range issueViews {
		rows = append(rows, []string{
			md.Link(issueViews[i].Identifier, issueViews[i].URL),
			issueViews[i].Title,
			issueViews[i].TeamName,
		})
	}
	doc.Add(md.NamedSection(fmt.Sprintf("📋 待办 · %d", len(issueViews)), md.Table(headers, rows)))

	for i := range issueViews {
		if plan, ok := plans[issueViews[i].Identifier]; ok {
			doc.Add(md.NamedSection(fmt.Sprintf("%s %s", issueViews[i].Identifier, issueViews[i].Title), &rawSection{content: plan}))
		}
	}

	if truncatedNote != "" {
		doc.Add(md.Paragraph(truncatedNote))
	}

	htmlBody, err := doc.ToHTML()
	if err != nil {
		return fmt.Errorf("render document: %w", err)
	}

	subject := fmt.Sprintf("Linear 今日任务 · %s %s", dateStr, dayOfWeek)

	return sendOrWrite(cfg, subject, htmlBody, "morning", dryRun)
}

// buildMorningPlan generates per-issue plan using AI.
// Returns a map of identifier → rendered markdown.
func buildMorningPlan(aiClient *internal.AIProvider, details []internal.IssueDetail) map[string]string {
	if len(details) == 0 || !aiClient.IsConfigured() {
		return nil
	}

	raw := aiClient.MorningPlan(details)
	if raw == "" {
		slog.Warn("AI returned empty response")

		return nil
	}
	slog.Info("AI raw response preview", "len", len(raw), "raw", raw[:min(len(raw), 2000)])

	return parsePlanJSON(raw)
}

func parsePlanJSON(raw string) map[string]string {
	var result internal.PlanJSON
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		slog.Warn("failed to parse AI plan JSON", "error", err)

		return nil
	}

	plans := make(map[string]string, len(result.Reviews))
	for _, r := range result.Reviews { //nolint:dupl // structurally similar to evening but different types/headings
		var sections []md.ReviewSection
		if len(r.Context) > 0 {
			sections = append(sections, md.ReviewSection{Heading: "上下文", Items: r.Context})
		}
		if len(r.Bottleneck) > 0 {
			sections = append(sections, md.ReviewSection{Heading: "卡点", Items: r.Bottleneck})
		}
		if len(r.Advice) > 0 {
			sections = append(sections, md.ReviewSection{Heading: "建议", Items: r.Advice})
		}
		plans[r.Identifier] = md.AIReviewItem(sections...).Markdown()
	}

	return plans
}

func sendBriefEmptyEmail(cfg *internal.Config, subject, body string, dryRun bool) error {
	now := carbon.Now()
	fullSubject := fmt.Sprintf("%s · %s %s", subject, now.ToDateString(), now.ToShortWeekString())
	doc := md.NewDocument()
	doc.Add(md.Paragraph(body))
	htmlBody, err := doc.ToHTML()
	if err != nil {
		return fmt.Errorf("render morning empty email body: %w", err)
	}

	return sendOrWrite(cfg, fullSubject, htmlBody, "morning-empty", dryRun)
}

func sendOrWrite(cfg *internal.Config, subject, htmlBody, suffix string, dryRun bool) error {
	if dryRun {
		return writeHTML(htmlBody, suffix)
	}

	return sendEmail(cfg, subject, htmlBody)
}

func toIssueViewsFromDetails(details []linear.IssueDetail) []internal.IssueView {
	return lo.Map(details, func(d linear.IssueDetail, _ int) internal.IssueView {
		return internal.IssueView{
			Identifier: d.Identifier,
			Title:      d.Title,
			Priority:   priorityLabel(d.Priority),
			TeamName:   d.TeamName,
			URL:        d.URL,
		}
	})
}

// rawSection is a Section that renders pre-built Markdown content directly.
type rawSection struct {
	content string
}

func (r *rawSection) Markdown() string {
	return r.content
}

func (r *rawSection) Add(_ ...md.Section) {}
