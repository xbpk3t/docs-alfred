package cmd

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/linear2nl/internal"
	"github.com/xbpk3t/docs-alfred/linear2nl/linear"
	"github.com/xbpk3t/docs-alfred/pkg/ai"
)

// renderMorningIssueContent pre-renders reason/impact/action into HTML,
// matching the same style as the evening report's per-issue review.
func renderMorningIssueContent(item *internal.GroupItemView) template.HTML {
	var sb strings.Builder
	renderReviewSection(&sb, "📌 原因", item.Reason)
	renderReviewSection(&sb, "💡 影响", item.Impact)
	renderReviewSection(&sb, "🎯 行动", item.Action)

	return template.HTML(sb.String()) //nolint:gosec // G203: AI-generated content for trusted template
}

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

	allViews := toIssueViews(issues)

	// Truncate to 15 items if too many, with a note.
	var truncatedNote string
	if len(allViews) > 15 {
		truncatedNote = fmt.Sprintf("还有 %d 个低优先级未显示", len(allViews)-15)
		allViews = allViews[:15]
	}

	groups := buildMorningGroups(aiClient, allViews)

	// Append truncated note as an extra group if needed.
	if truncatedNote != "" {
		groups = append(groups, internal.GroupView{
			Name:  "Note",
			Emoji: "⚠️",
			Issues: []internal.GroupItemView{
				{
					Identifier: "",
					Title:      truncatedNote,
				},
			},
		})
	}

	now := time.Now().In(cst)
	data := internal.MorningData{
		Date:      now.Format("2006-01-02"),
		DayOfWeek: formatWeekday(now),
		Theme:     cfg.Theme,
		Groups:    groups,
	}

	// Render template
	tmpl, err := template.New("morning.gohtml").Funcs(tmplFuncs()).Funcs(template.FuncMap{
		"groupCSSClass": groupCSSClass,
	}).ParseFS(morningTemplates, "templates/morning.gohtml")
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

// groupEmoji maps group names to their display emoji.
func groupEmoji(name string) string {
	switch name {
	case "FIXME":
		return "🔴"
	case "MAYBE":
		return "🟡"
	case "REMOVE":
		return "⚪"
	default:
		return "📋"
	}
}

// groupCSSClass maps group names to CSS class suffixes.
func groupCSSClass(name string) string {
	switch name {
	case "FIXME":
		return "fixme"
	case "MAYBE":
		return "maybe"
	case "REMOVE":
		return "remove"
	default:
		return "uncategorized"
	}
}

// groupOrder returns a sort key for groups: FIXME=0, MAYBE=1, REMOVE=2, others=3.
func groupOrder(name string) int {
	switch name {
	case "FIXME":
		return 0
	case "MAYBE":
		return 1
	case "REMOVE":
		return 2
	default:
		return 3
	}
}

const fallbackGroupName = "Uncategorized"

// fallbackGroup returns a single flat group when AI is unavailable or fails.
func fallbackGroup(views []internal.IssueView) []internal.GroupView {
	return []internal.GroupView{
		{
			Name:   fallbackGroupName,
			Emoji:  "📋",
			Issues: toGroupItems(views, nil),
		},
	}
}

// buildMorningGroups generates the grouped view of issues using AI review.
// Falls back to a flat uncategorized group if AI is unavailable or returns invalid JSON.
func buildMorningGroups(aiClient *internal.AIProvider, views []internal.IssueView) []internal.GroupView {
	if !aiClient.IsConfigured() || len(views) == 0 {
		slog.Info("AI not configured or no issues; using fallback flat group")

		return fallbackGroup(views)
	}

	raw := aiClient.MorningStructuredReview(views)
	if raw == "" {
		slog.Warn("AI returned empty response; using fallback flat group")

		return fallbackGroup(views)
	}
	slog.Info("AI morning review response", "len", len(raw))

	result, err := parseMorningReviewJSON(raw)
	if err != nil {
		slog.Warn("failed to parse AI morning review JSON; using fallback flat group", "error", err)

		return fallbackGroup(views)
	}

	return buildGroupsFromResult(result, views)
}

// buildGroupsFromResult converts parsed AI results into GroupViews with proper ordering
// and adds any uncategorized issues not mentioned in the AI response.
func buildGroupsFromResult(result *internal.MorningReviewJSON, views []internal.IssueView) []internal.GroupView {
	viewMap := lo.KeyBy(views, func(v internal.IssueView) string { return v.Identifier })
	mentioned := make(map[string]bool)
	var groups []internal.GroupView

	for _, g := range result.Groups {
		items := buildGroupItems(g, viewMap, mentioned)
		if len(items) > 0 {
			groups = append(groups, internal.GroupView{
				Name:   g.Name,
				Emoji:  groupEmoji(g.Name),
				Issues: items,
			})
		}
	}

	sort.SliceStable(groups, func(i, j int) bool {
		return groupOrder(groups[i].Name) < groupOrder(groups[j].Name)
	})

	// Add uncategorized issues (fetched but not mentioned by AI).
	var uncategorized []internal.GroupItemView
	for _, v := range views {
		if !mentioned[v.Identifier] {
			uncategorized = append(uncategorized, toGroupItem(&v))
		}
	}
	if len(uncategorized) > 0 {
		groups = append(groups, internal.GroupView{
			Name:   fallbackGroupName,
			Emoji:  "📋",
			Issues: uncategorized,
		})
	}

	return groups
}

// buildGroupItems converts a single AI group's issues into GroupItemViews,
// merging metadata from original views and pre-rendering AI content.
func buildGroupItems(g internal.MorningGroupJSON, viewMap map[string]internal.IssueView, mentioned map[string]bool) []internal.GroupItemView {
	items := make([]internal.GroupItemView, 0, len(g.Issues))
	for _, iss := range g.Issues {
		mentioned[iss.Identifier] = true
		item := internal.GroupItemView{
			Identifier: iss.Identifier,
			Title:      iss.Title,
			Reason:     iss.Reason,
			Impact:     iss.Impact,
			Action:     iss.Action,
		}
		if orig, ok := viewMap[iss.Identifier]; ok {
			item.URL = orig.URL
			item.Priority = orig.Priority
			item.TeamName = orig.TeamName
			item.DueDate = orig.DueDate
		}
		item.Content = renderMorningIssueContent(&item)
		items = append(items, item)
	}

	return items
}

// parseMorningReviewJSON attempts to unmarshal the AI response as MorningReviewJSON.
func parseMorningReviewJSON(raw string) (*internal.MorningReviewJSON, error) {
	var result internal.MorningReviewJSON
	if err := ai.UnmarshalStrictJSON(raw, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// toGroupItems converts IssueViews to GroupItemViews, optionally applying AI data from a lookup map.
func toGroupItems(views []internal.IssueView, aiData map[string]*internal.MorningIssueItem) []internal.GroupItemView {
	items := make([]internal.GroupItemView, 0, len(views))
	for i := range views {
		item := toGroupItem(&views[i])
		if aiData != nil {
			if data, ok := aiData[views[i].Identifier]; ok {
				item.Reason = data.Reason
				item.Impact = data.Impact
				item.Action = data.Action
			}
		}
		items = append(items, item)
	}

	return items
}

// toGroupItem converts a single IssueView to a GroupItemView.
func toGroupItem(v *internal.IssueView) internal.GroupItemView {
	return internal.GroupItemView{
		Identifier: v.Identifier,
		Title:      v.Title,
		Priority:   v.Priority,
		TeamName:   v.TeamName,
		DueDate:    v.DueDate,
		URL:        v.URL,
	}
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
