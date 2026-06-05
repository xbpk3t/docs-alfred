package cmd

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"os"
	"time"

	"github.com/resend/resend-go/v2"
	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/linear2nl/internal"
	"github.com/xbpk3t/docs-alfred/linear2nl/linear"
	"github.com/yuin/goldmark"
)

var cst = time.FixedZone("CST", 8*3600)

// newReportCmd creates a cobra command for a report subcommand (morning/evening).
// It consolidates the shared flag setup and config-loading logic.
func newReportCmd(use, short string, runFunc func(*internal.Config, bool) error) *cobra.Command {
	var cfgFile string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := internal.LoadConfig(cfgFile)
			if err != nil {
				return err
			}

			return runFunc(cfg, dryRun)
		},
	}

	cmd.Flags().StringVarP(&cfgFile, "config", "c", "linear2nl/linear2nl.yml", "config file path")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print HTML to stdout instead of sending email")

	return cmd
}

// Execute is the entry point for linear2nl.
func Execute() {
	rootCmd := &cobra.Command{
		Use:   "linear2nl",
		Short: "Linear task reports via email",
		Long: `Generate and send Linear task reports via email.

Subcommands:
  morning   Send morning report with today's tasks
  evening   Send evening report with today's accomplishments`,
	}

	rootCmd.AddCommand(newMorningCmd())
	rootCmd.AddCommand(newEveningCmd())
	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})

	if err := rootCmd.Execute(); err != nil {
		slog.Error("command failed", "error", err)
		os.Exit(1)
	}
}

// --- Shared helpers ---

func toIssueViews(issues []linear.Issue) []internal.IssueView {
	views := make([]internal.IssueView, len(issues))
	for i := range issues {
		iss := &issues[i]
		views[i] = internal.IssueView{
			Identifier: iss.Identifier,
			Title:      iss.Title,
			Priority:   priorityLabel(iss.Priority),
			TeamName:   iss.TeamName,
			DueDate:    iss.DueDate,
			URL:        iss.URL,
		}
	}

	return views
}

func toStateChangeViews(changes []linear.StateChange) []internal.StateChangeView {
	views := make([]internal.StateChangeView, len(changes))
	for i := range changes {
		ch := &changes[i]
		views[i] = internal.StateChangeView{
			IssueIdentifier: ch.IssueIdentifier,
			IssueTitle:      ch.IssueTitle,
			FromState:       ch.FromState,
			ToState:         ch.ToState,
			TeamName:        ch.TeamName,
			URL:             ch.URL,
		}
	}

	return views
}

func priorityLabel(p float64) string {
	switch int(p) {
	case 1:
		return "🔥 P0"
	case 2:
		return "🔴 P1"
	case 3:
		return "⚡ P2"
	case 4:
		return "📋 P3"
	default:
		return ""
	}
}

func formatWeekday(t time.Time) string {
	weekdays := []string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}

	return weekdays[t.Weekday()]
}

// markdownToHTML converts a markdown string to HTML using goldmark.
func markdownToHTML(s string) string {
	var buf bytes.Buffer
	if err := goldmark.New().Convert([]byte(s), &buf); err != nil {
		return s
	}

	return buf.String()
}

func tmplFuncs() template.FuncMap {
	return template.FuncMap{
		"markdownToHTML": func(s string) template.HTML {
			return template.HTML(markdownToHTML(s)) //nolint:gosec // G203: goldmark output is trusted HTML
		},
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s) //nolint:gosec // G203: intentional safe HTML passthrough
		},
	}
}

func renderHTML(tmpl *template.Template, name string, data any) (string, error) {
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}

func sendEmail(cfg *internal.Config, subject, htmlBody string) error {
	client := resend.NewClient(cfg.Resend.Token)

	fromName := cfg.Resend.FromName
	if fromName == "" {
		fromName = "Linear Bot"
	}

	params := &resend.SendEmailRequest{
		From:    fromName + " <onboarding@resend.dev>",
		To:      cfg.Resend.MailTo,
		Subject: subject,
		Html:    htmlBody,
	}

	slog.Info("sending email", "to", cfg.Resend.MailTo, "subject", subject)
	sent, err := client.Emails.SendWithContext(context.Background(), params)
	if err != nil {
		return fmt.Errorf("send email: %w", err)
	}

	slog.Info("email sent", "id", sent.Id)

	return nil
}

func writeHTML(htmlBody, suffix string) error {
	filename := fmt.Sprintf("linear2nl_%s.html", suffix)
	if err := os.WriteFile(filename, []byte(htmlBody), 0600); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	slog.Info("HTML written to file", "filename", filename)

	return nil
}
