package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	carbon "github.com/dromara/carbon/v2"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/linear2nl/internal"
	"github.com/xbpk3t/docs-alfred/linear2nl/linear"
	"github.com/xbpk3t/docs-alfred/pkg/ai"
	"github.com/xbpk3t/docs-alfred/pkg/md"
	"golang.org/x/sync/errgroup"
)

func newEveningCmd() *cobra.Command {
	return newReportCmd("evening", "Send evening report with today's accomplishments", runEvening)
}

func runEvening(cfg *internal.Config, dryRun bool) error {
	ctx := context.Background()
	client := linear.NewClient(cfg.Linear.APIKey, cfg.Linear.TeamKeys)
	aiClient := internal.NewAIProvider(cfg.AI)

	todayStart := carbon.Yesterday().StartOfDay().StdTime()

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

	buildEveningSummary(aiClient, relevantDetails, completedViews, changeViews)

	now := carbon.Yesterday()
	data := internal.EveningData{
		Date:         now.ToDateString(),
		DayOfWeek:    now.ToShortWeekString(),
				Completed:    completedViews,
		StateChanges: changeViews,
		Stats: internal.EveningStats{
			Completed:  len(completed),
			InProgress: len(inProgress),
		},
	}

	htmlBody, err := buildEveningHTML(&data, completed, changes, completedViews, changeViews)
	if err != nil {
		return fmt.Errorf("render document: %w", err)
	}

	subject := fmt.Sprintf("🌙 Linear 今日收获 · %s %s", data.Date, data.DayOfWeek)

	return sendOrWrite(cfg, subject, htmlBody, "evening", dryRun)
}

func buildEveningHTML(data *internal.EveningData, completed []linear.Issue, changes []linear.StateChange, completedViews []internal.IssueView, changeViews []internal.StateChangeView) (string, error) {
	doc := md.NewDocument()
	doc.Add(md.NamedSection(fmt.Sprintf("Linear 今日收获 · %s %s", data.Date, data.DayOfWeek)))

	if len(completed) > 0 {
		headers := []string{"ID", "Title", "Team"}
		var rows [][]string
		for i := range completed {
			rows = append(rows, []string{
				md.Link(completed[i].Identifier, completed[i].URL),
				completed[i].Title,
				completed[i].TeamName,
			})
		}
		doc.Add(md.NamedSection(fmt.Sprintf("✅ 完成 · %d", len(completed)), md.Table(headers, rows)))
	}

	if len(changes) > 0 {
		headers := []string{"ID", "Title", "Status", "Team"}
		var rows [][]string
		for i := range changes {
			rows = append(rows, []string{
				md.Link(changes[i].IssueIdentifier, changes[i].URL),
				changes[i].IssueTitle,
				fmt.Sprintf("%s → %s", changes[i].FromState, changes[i].ToState),
				changes[i].TeamName,
			})
		}
		doc.Add(md.NamedSection(fmt.Sprintf("🔄 状态变更 · %d", len(changes)), md.Table(headers, rows)))
	}

	for i := range completedViews {
		if completedViews[i].Review != "" {
			doc.Add(md.NamedSection(fmt.Sprintf("%s %s", completedViews[i].Identifier, completedViews[i].Title), &rawSection{content: completedViews[i].Review}))
		}
	}
	for i := range changeViews {
		if changeViews[i].Review != "" {
			doc.Add(md.NamedSection(fmt.Sprintf("%s %s", changeViews[i].IssueIdentifier, changeViews[i].IssueTitle), &rawSection{content: changeViews[i].Review}))
		}
	}

	return doc.ToHTML()
}

func buildEveningSummary(aiClient *internal.AIProvider, details []linear.IssueDetail, completedViews []internal.IssueView, changeViews []internal.StateChangeView) {
	r := buildPerIssueReviews(aiClient, details)
	if r == nil {
		return
	}
	for i := range completedViews {
		if review, ok := r.reviews[completedViews[i].Identifier]; ok {
			completedViews[i].Review = review
		}
	}
	for i := range changeViews {
		if review, ok := r.reviews[changeViews[i].IssueIdentifier]; ok {
			changeViews[i].Review = review
		}
	}
}

type eveningQueryData struct {
	completed      []linear.Issue
	changes        []linear.StateChange
	inProgress     []linear.Issue
	updatedDetails []linear.IssueDetail
}

func queryEveningData(ctx context.Context, client *linear.Client, todayStart time.Time) (eveningQueryData, error) {
	g, ctx := errgroup.WithContext(ctx)

	var completed []linear.Issue
	g.Go(func() error {
		var err error
		completed, err = client.GetCompletedTodayIssues(ctx, todayStart)
		if err != nil {
			return fmt.Errorf("query completed issues: %w", err)
		}
		slog.Info("fetched completed issues", "count", len(completed))

		return nil
	})

	var changes []linear.StateChange
	g.Go(func() error {
		var err error
		changes, err = client.GetStateChanges(ctx, todayStart)
		if err != nil {
			slog.Warn("query state changes failed", "error", err)
			changes = nil

			return nil
		}
		slog.Info("fetched state changes", "count", len(changes))

		return nil
	})

	var inProgress []linear.Issue
	g.Go(func() error {
		var err error
		inProgress, err = client.GetInProgressIssues(ctx)
		if err != nil {
			slog.Warn("query in-progress issues failed", "error", err)
			inProgress = nil

			return nil
		}
		slog.Info("fetched in-progress issues", "count", len(inProgress))

		return nil
	})

	var updatedDetails []linear.IssueDetail
	g.Go(func() error {
		var err error
		updatedDetails, err = client.GetUpdatedIssuesWithDetails(ctx, todayStart)
		if err != nil {
			slog.Warn("query issue details failed", "error", err)
			updatedDetails = nil

			return nil
		}
		slog.Info("fetched issue details for AI review", "count", len(updatedDetails))

		return nil
	})

	if err := g.Wait(); err != nil {
		return eveningQueryData{}, err
	}

	return eveningQueryData{
		completed:      completed,
		changes:        changes,
		inProgress:     inProgress,
		updatedDetails: updatedDetails,
	}, nil
}

func filterActiveDetails(completed []linear.Issue, changes []linear.StateChange, updatedDetails []linear.IssueDetail) []linear.IssueDetail {
	activeIDs := lo.SliceToMap(completed, func(i linear.Issue) (string, bool) {
		return i.Identifier, true
	})
	for i := range changes {
		activeIDs[changes[i].IssueIdentifier] = true
	}

	return lo.Filter(updatedDetails, func(d linear.IssueDetail, _ int) bool {
		return activeIDs[d.Identifier]
	})
}

type perIssueReviewResult struct {
	reviews map[string]string
}

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

type AIReviewJSON struct {
	Reviews []AIReviewItemJSON `json:"reviews"`
	Summary []string           `json:"summary"`
}

type AIReviewItemJSON struct {
	Identifier string   `json:"identifier"`
	Title      string   `json:"title"`
	Progress   []string `json:"progress"`
	Knowledge  []string `json:"knowledge"`
	Review     []string `json:"review"`
}

func parsePerIssueReviewJSON(raw string) *perIssueReviewResult {
	result, err := parseAIReviewJSON(raw)
	if err != nil {
		return nil
	}

	reviews := make(map[string]string, len(result.Reviews))
	for _, r := range result.Reviews {
		var sections []md.ReviewSection
		if len(r.Progress) > 0 {
			sections = append(sections, md.ReviewSection{Heading: "决策/进展", Items: r.Progress})
		}
		if len(r.Knowledge) > 0 {
			sections = append(sections, md.ReviewSection{Heading: "知识点", Items: r.Knowledge})
		}
		if len(r.Review) > 0 {
			sections = append(sections, md.ReviewSection{Heading: "Review", Items: r.Review})
		}
		reviews[r.Identifier] = md.AIReviewItem(sections...).Markdown()
	}

	return &perIssueReviewResult{reviews: reviews}
}

func parseAIReviewJSON(raw string) (*AIReviewJSON, error) {
	var result AIReviewJSON
	if err := ai.UnmarshalStrictJSON(raw, &result); err != nil {
		slog.Warn("failed to parse AI review JSON", "error", err)

		return nil, err
	}

	return &result, nil
}

func toIssueDetails(issues []linear.IssueDetail) []internal.IssueDetail {
	return lo.Map(issues, func(iss linear.IssueDetail, _ int) internal.IssueDetail {
		return internal.IssueDetail{
			Identifier:  iss.Identifier,
			Title:       iss.Title,
			Description: iss.Description,
			StateName:   iss.StateName,
			TeamName:    iss.TeamName,
			URL:         iss.URL,
			Priority:    priorityLabel(iss.Priority),
			Comments: lo.Map(iss.Comments, func(c linear.Comment, _ int) internal.Comment {
				return internal.Comment{Body: c.Body, UserName: c.UserName, CreatedAt: c.CreatedAt}
			}),
		}
	})
}

func sendBriefEveningEmpty(cfg *internal.Config, dryRun bool) error {
	now := carbon.Now()
	subject := fmt.Sprintf("🌙 Linear 今日收获 · %s %s", now.ToDateString(), now.ToShortWeekString())
	doc := md.NewDocument()
	doc.Add(md.Paragraph("今天没有完成记录 🎉"))
	htmlBody, _ := doc.ToHTML()

	return sendOrWrite(cfg, subject, htmlBody, "evening-empty", dryRun)
}
