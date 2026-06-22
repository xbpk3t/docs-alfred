package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/resend/resend-go/v2"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/linear2nl/internal"
	"github.com/xbpk3t/docs-alfred/linear2nl/linear"
	"github.com/xbpk3t/docs-alfred/pkg/carboninit"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
	"github.com/xbpk3t/docs-alfred/pkg/validator"
)

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
	carboninit.Setup()
	validator.Setup()

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
	rootCmd.AddCommand(newExportCmd())
	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})

	if err := rootCmd.Execute(); err != nil {
		slog.Error("command failed", "error", err)
		os.Exit(1)
	}
}

// --- Shared helpers ---

func toIssueViews(issues []linear.Issue) []internal.IssueView {
	return lo.Map(issues, func(iss linear.Issue, _ int) internal.IssueView {
		return internal.IssueView{
			Identifier: iss.Identifier,
			Title:      iss.Title,
			Priority:   priorityLabel(iss.Priority),
			TeamName:   iss.TeamName,
			DueDate:    iss.DueDate,
			URL:        iss.URL,
		}
	})
}

func toStateChangeViews(changes []linear.StateChange) []internal.StateChangeView {
	return lo.Map(changes, func(ch linear.StateChange, _ int) internal.StateChangeView {
		return internal.StateChangeView{
			IssueIdentifier: ch.IssueIdentifier,
			IssueTitle:      ch.IssueTitle,
			FromState:       ch.FromState,
			ToState:         ch.ToState,
			TeamName:        ch.TeamName,
			URL:             ch.URL,
		}
	})
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

func writeOutput(data []byte, filename string) error {
	if err := fileutil.AtomicWriteFile(filename, data, fileutil.FilePermPrivate); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	slog.Info("output written", "filename", filename)

	return nil
}

func writeHTML(htmlBody, suffix string) error {
	return writeOutput([]byte(htmlBody), fmt.Sprintf("linear2nl_%s.html", suffix))
}
