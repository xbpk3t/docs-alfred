package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	carbon "github.com/dromara/carbon/v2"
	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/cmd/linear2nl/internal"
	"github.com/xbpk3t/docs-alfred/internal/linear"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
)

func newExportCmd() *cobra.Command {
	var (
		cfgFile    string
		days       int
		outputPath string
	)

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export Linear issues as JSON or Markdown",
		Long: `Export issues updated within the given time window.

Output includes description, comments (up to 100), and parent relationship.
Default: JSON, last 2 days, written to linear2nl_export_<date>.json.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := internal.LoadConfig(cfgFile)
			if err != nil {
				return err
			}

			since := carbon.Now().SubDays(days).StartOfDay().StdTime()
			slog.Info("fetching issues", "since", since.Format(time.RFC3339), "days", days)

			client := linear.NewClient(cfg.Linear.APIKey, cfg.Linear.TeamKeys)
			details, err := client.GetUpdatedIssuesWithDetails(context.Background(), since)
			if err != nil {
				return err
			}
			slog.Info("fetched issues", "count", len(details))
			if err := fileutil.ValidateOutputPath(outputPath); err != nil {
				return err
			}

			now := carbon.Now()
			dateStr := now.Format("Ymd")

			if strings.HasSuffix(outputPath, ".md") {
				return exportMarkdown(details, outputPath, dateStr)
			}

			return exportJSON(details, outputPath, dateStr)
		},
	}

	cmd.Flags().StringVarP(&cfgFile, "config", "c", "cmd/linear2nl/linear2nl.yml", "config file path")
	cmd.Flags().IntVar(&days, "days", 2, "export issues updated in the last N days")
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "output file path (default: linear2nl_export_<date>.json)")

	return cmd
}

type exportPayload struct {
	Issues []exportIssue `json:"issues"`
}

type exportIssue struct {
	TeamKey          string          `json:"teamKey"`
	Title            string          `json:"title"`
	Description      string          `json:"description,omitempty"`
	StateName        string          `json:"stateName"`
	StateType        string          `json:"stateType"`
	TeamName         string          `json:"teamName"`
	Identifier       string          `json:"identifier"`
	URL              string          `json:"url"`
	CompletedAt      string          `json:"completedAt,omitempty"`
	UpdatedAt        string          `json:"updatedAt"`
	ParentIdentifier string          `json:"parentIdentifier,omitempty"`
	Comments         []exportComment `json:"comments"`
	Priority         float64         `json:"priority"`
}

type exportComment struct {
	Body      string `json:"body"`
	UserName  string `json:"userName"`
	CreatedAt string `json:"createdAt"`
}

func exportJSON(details []linear.IssueDetail, output, dateStr string) error {
	payload := exportPayload{
		Issues: make([]exportIssue, 0, len(details)),
	}
	for i := range details {
		d := &details[i]
		comments := make([]exportComment, 0, len(d.Comments))
		for _, c := range d.Comments {
			comments = append(comments, exportComment{
				Body:      c.Body,
				UserName:  c.UserName,
				CreatedAt: c.CreatedAt,
			})
		}
		payload.Issues = append(payload.Issues, exportIssue{
			Identifier:       d.Identifier,
			Title:            d.Title,
			Description:      d.Description,
			StateName:        d.StateName,
			StateType:        d.StateType,
			TeamName:         d.TeamName,
			TeamKey:          d.TeamKey,
			URL:              d.URL,
			CompletedAt:      d.CompletedAt,
			UpdatedAt:        d.UpdatedAt,
			ParentIdentifier: d.ParentIdentifier,
			Priority:         d.Priority,
			Comments:         comments,
		})
	}

	data, err := fileutil.MarshalJSON(payload)
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}

	filename := output
	if filename == "" {
		filename = fmt.Sprintf("linear2nl_export_%s.json", dateStr)
	}

	return writeOutput(data, filename)
}

func exportMarkdown(details []linear.IssueDetail, output, dateStr string) error {
	var b strings.Builder
	fmt.Fprintf(&b, "# Linear Export (%s)\n\n", dateStr)

	for i := range details {
		d := &details[i]
		fmt.Fprintf(&b, "## %s %s\n\n", d.Identifier, d.Title)
		fmt.Fprintf(&b, "- **State**: %s (%s)\n", d.StateName, d.StateType)
		fmt.Fprintf(&b, "- **Team**: %s (%s)\n", d.TeamName, d.TeamKey)
		fmt.Fprintf(&b, "- **URL**: %s\n", d.URL)
		if d.ParentIdentifier != "" {
			fmt.Fprintf(&b, "- **Parent**: %s\n", d.ParentIdentifier)
		}
		if d.CompletedAt != "" {
			fmt.Fprintf(&b, "- **Completed**: %s\n", d.CompletedAt)
		}
		fmt.Fprintf(&b, "- **Updated**: %s\n", d.UpdatedAt)
		b.WriteString("\n")

		if d.Description != "" {
			b.WriteString("### Description\n\n")
			b.WriteString(d.Description)
			b.WriteString("\n\n")
		}

		if len(d.Comments) > 0 {
			b.WriteString("### Comments\n\n")
			for _, c := range d.Comments {
				fmt.Fprintf(&b, "**%s** (%s)\n\n%s\n\n", c.UserName, c.CreatedAt, c.Body)
			}
		}

		b.WriteString("---\n\n")
	}

	filename := output
	if filename == "" {
		filename = fmt.Sprintf("linear2nl_export_%s.md", dateStr)
	}

	return writeOutput([]byte(b.String()), filename)
}
