package cmd

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"log/slog"
	"sort"
	"strings"

	carbon "github.com/dromara/carbon/v2"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/linear2nl/internal"
	"github.com/xbpk3t/docs-alfred/linear2nl/linear"
	"github.com/xbpk3t/docs-alfred/pkg/ai"
)

// renderMorningIssueContent pre-renders context/bottleneck/advice into HTML,
// matching the same style as the evening report's per-issue review.
func renderMorningIssueContent(item *internal.GroupItemView) template.HTML {
	var sb strings.Builder
	renderReviewSection(&sb, "上下文", item.Context)
	renderReviewSection(&sb, "卡点", item.Bottleneck)
	renderReviewSection(&sb, "建议", item.Advice)

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

	// Fetch active issues with full details (description + comments) for AI review.
	details, err := client.GetActiveIssuesWithDetails(ctx)
	if err != nil {
		return fmt.Errorf("query active issues with details: %w", err)
	}

	if len(details) == 0 {
		return sendBriefEmptyEmail(cfg, "Linear 今日任务", "今天没有待办任务", dryRun)
	}

	// Convert linear.IssueDetail → internal.IssueDetail for AI and display.
	issueDetails := toIssueDetails(details)
	issueViews := toIssueViewsFromDetails(details)

	// Truncate to 15 items if too many, with a note.
	var truncatedNote string
	if len(issueViews) > 15 {
		truncatedNote = fmt.Sprintf("还有 %d 个低优先级未显示", len(issueViews)-15)
		issueDetails = issueDetails[:15]
		issueViews = issueViews[:15]
	}

	// Stage 1: Fast classification using metadata only (identifier/title/priority/team/dueDate).
	groups := buildMorningGroups(aiClient, issueViews)

	// Stage 2: Deep analysis for FIXME + MAYBE groups using full description + comments.
	enrichActiveGroups(aiClient, groups, issueDetails)

	// Append truncated note as an extra group if needed.
	if truncatedNote != "" {
		groups = append(groups, internal.GroupView{
			Name:  "Note",
			Emoji: "",
			Issues: []internal.GroupItemView{
				{
					Identifier: "",
					Title:      truncatedNote,
				},
			},
		})
	}

	now := carbon.Now()
	data := internal.MorningData{
		Date:      now.ToDateString(),
		DayOfWeek: now.ToShortWeekString(),
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

	subject := fmt.Sprintf("Linear 今日任务 · %s %s", data.Date, data.DayOfWeek)

	return sendOrWrite(cfg, subject, htmlBody, "morning", dryRun)
}

// groupEmoji returns the emoji for a group. Now unused by the template — emoji removed.
func groupEmoji(name string) string {
	return ""
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
			Emoji:  "",
			Issues: toGroupItems(views, nil),
		},
	}
}

// buildMorningGroups generates the grouped view of issues using AI classification (stage 1).
// Takes metadata-only IssueView for fast FIXME/MAYBE/REMOVE grouping.
// Falls back to a flat uncategorized group if AI is unavailable or returns invalid JSON.
func buildMorningGroups(aiClient *internal.AIProvider, views []internal.IssueView) []internal.GroupView {
	if !aiClient.IsConfigured() || len(views) == 0 {
		slog.Info("AI not configured or no issues; using fallback flat group")

		return fallbackGroup(views)
	}

	raw := aiClient.MorningClassify(views)
	if raw == "" {
		slog.Warn("AI returned empty response; using fallback flat group")

		return fallbackGroup(views)
	}
	slog.Info("AI morning classification response", "len", len(raw))

	result, err := parseMorningReviewJSON(raw)
	if err != nil {
		slog.Warn("failed to parse AI morning classification JSON; using fallback flat group", "error", err)

		return fallbackGroup(views)
	}

	return buildGroupsFromResult(result, views)
}

// buildGroupsFromResult converts parsed AI results into GroupViews with proper ordering
// and adds any uncategorized issues not mentioned in the AI response.
func buildGroupsFromResult(result *internal.MorningReviewJSON, views []internal.IssueView) []internal.GroupView {
	viewMap := lo.KeyBy(views, func(v internal.IssueView) string { return v.Identifier })
	mentioned := make(map[string]bool)

	groups := lo.FilterMap(result.Groups, func(g internal.MorningGroupJSON, _ int) (internal.GroupView, bool) {
		items := buildGroupItems(g, viewMap, mentioned)
		if len(items) == 0 {
			return internal.GroupView{}, false
		}

		return internal.GroupView{
			Name:   g.Name,
			Emoji:  groupEmoji(g.Name),
			Issues: items,
		}, true
	})

	sort.SliceStable(groups, func(i, j int) bool {
		return groupOrder(groups[i].Name) < groupOrder(groups[j].Name)
	})

	// Add uncategorized issues (fetched but not mentioned by AI).
	uncategorized := lo.FilterMap(views, func(v internal.IssueView, _ int) (internal.GroupItemView, bool) {
		if mentioned[v.Identifier] {
			return internal.GroupItemView{}, false
		}

		return toGroupItem(&v), true
	})
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
	return lo.Map(g.Issues, func(iss internal.MorningIssueItem, _ int) internal.GroupItemView {
		mentioned[iss.Identifier] = true
		item := internal.GroupItemView{
			Identifier: iss.Identifier,
			Title:      iss.Title,
			Context:    iss.Context,
			Bottleneck: iss.Bottleneck,
			Advice:     iss.Advice,
		}
		if orig, ok := viewMap[iss.Identifier]; ok {
			item.URL = orig.URL
			item.Priority = orig.Priority
			item.TeamName = orig.TeamName
			item.DueDate = orig.DueDate
		}
		item.Content = renderMorningIssueContent(&item)

		return item
	})
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
	return lo.Map(views, func(v internal.IssueView, _ int) internal.GroupItemView {
		item := toGroupItem(&v)
		if aiData != nil {
			if data, ok := aiData[v.Identifier]; ok {
				item.Context = data.Context
				item.Bottleneck = data.Bottleneck
				item.Advice = data.Advice
			}
		}

		return item
	})
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

// toIssueViewsFromDetails converts linear.IssueDetail to display IssueView.
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

// activeGroupNames are the groups that should receive deep analysis (stage 2).
var activeGroupNames = map[string]bool{"FIXME": true, "MAYBE": true}

// enrichActiveGroups performs stage-2 deep analysis on FIXME + MAYBE groups.
// Sends their full IssueDetail (description + comments) to AI for context/bottleneck/advice.
func enrichActiveGroups(aiClient *internal.AIProvider, groups []internal.GroupView, allDetails []internal.IssueDetail) {
	detailMap := lo.KeyBy(allDetails, func(d internal.IssueDetail) string { return d.Identifier })

	// Collect identifiers from FIXME + MAYBE groups, tracking positions for merging.
	groupIdx := make(map[string]int)
	itemIdx := make(map[string]int)

	activeIdentifiers := lo.FlatMap(groups, func(g internal.GroupView, gi int) []string {
		if !activeGroupNames[g.Name] {
			return nil
		}

		return lo.FilterMap(g.Issues, func(item internal.GroupItemView, ii int) (string, bool) {
			if item.Identifier == "" {
				return "", false
			}
			groupIdx[item.Identifier] = gi
			itemIdx[item.Identifier] = ii

			return item.Identifier, true
		})
	})

	if len(activeIdentifiers) == 0 {
		return
	}

	// Collect IssueDetail for active groups only.
	activeDetails := lo.FilterMap(activeIdentifiers, func(id string, _ int) (internal.IssueDetail, bool) {
		d, ok := detailMap[id]

		return d, ok
	})
	if len(activeDetails) == 0 {
		return
	}

	raw := aiClient.MorningDeepAnalysis(activeDetails)
	if raw == "" {
		slog.Warn("AI deep analysis returned empty response; skipping enrichment")

		return
	}
	slog.Info("AI morning deep analysis response", "len", len(raw))

	result, err := parseMorningAnalysisJSON(raw)
	if err != nil {
		slog.Warn("failed to parse AI morning deep analysis JSON; skipping enrichment", "error", err)

		return
	}

	// Merge analysis results into groups.
	for _, review := range result.Reviews {
		gi, okGI := groupIdx[review.Identifier]
		ii, okII := itemIdx[review.Identifier]
		if !okGI || !okII {
			continue
		}
		item := &groups[gi].Issues[ii]
		item.Context = review.Context
		item.Bottleneck = review.Bottleneck
		item.Advice = review.Advice
		item.Content = renderMorningIssueContent(item)
	}
}

// parseMorningAnalysisJSON attempts to unmarshal the AI response as MorningAnalysisJSON.
func parseMorningAnalysisJSON(raw string) (*internal.MorningAnalysisJSON, error) {
	var result internal.MorningAnalysisJSON
	if err := ai.UnmarshalStrictJSON(raw, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func sendBriefEmptyEmail(cfg *internal.Config, subject, body string, dryRun bool) error {
	now := carbon.Now()
	fullSubject := fmt.Sprintf("%s · %s %s", subject, now.ToDateString(), now.ToShortWeekString())
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
