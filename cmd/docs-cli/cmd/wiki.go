package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	workspaceuc "github.com/xbpk3t/docs-alfred/internal/docs/check"
	wikiuc "github.com/xbpk3t/docs-alfred/internal/docs/ingest"
	wikiaudit "github.com/xbpk3t/docs-alfred/internal/docs/wiki/audit"
	wikicompact "github.com/xbpk3t/docs-alfred/internal/docs/wiki/compact"
	"github.com/xbpk3t/docs-alfred/pkg/ai"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
	"github.com/xbpk3t/docs-alfred/pkg/cmdutil"
	"github.com/xbpk3t/docs-alfred/pkg/mail"
	"github.com/xbpk3t/docs-alfred/pkg/output"
)

type wikiFlags struct {
	config         string
	wikiRoot       string
	model          string
	ghTopicsURL    string
	auditPaths     []string
	maxContentSize int
	dryRun         bool
	changedOnly    bool
	force          bool
}

const (
	wikiCommandName        = "wiki"
	wikiDigestCommandName  = "digest"
	wikiAuditCommandName   = "audit"
	wikiCheckCommandName   = "check"
	wikiStatsCommandName   = "stats"
	wikiCompactCommandName = "compact"
)

func newWikiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   wikiCommandName,
		Short: "Classify and summarize URLs into wiki knowledge base",
		Long: `Classify and summarize URLs into wiki knowledge base.

Uses AI to classify URLs by content type (video/audio/text), topic path,
and entry type (repo_eval/deep_dive/inbox). Writes structured entries.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return errors.New("use `docs-cli wiki add <urls...>` or `docs-cli wiki digest`")
			}

			return cmd.Help()
		},
	}

	cmd.AddCommand(newWikiAddCmd())
	cmd.AddCommand(newWikiDigestCmd())
	cmd.AddCommand(newWikiAuditCmd())
	cmd.AddCommand(newWikiCheckCmd())
	cmd.AddCommand(newWikiStatsCmd())
	cmd.AddCommand(newWikiCompactCmd())

	return cmd
}

func newWikiAddCmd() *cobra.Command {
	var flags wikiFlags
	cmd := &cobra.Command{
		Use:   "add <urls...>",
		Short: "Classify and summarize explicit URLs into wiki",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := wikiuc.LoadConfig(flags.config, flags.wikiRoot)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			resolveWikiAPIKey(cfg)
			applyWikiFlagOverrides(cfg, &flags)

			result, err := wikiuc.RunAddURLs(context.Background(), wikiuc.AddInput{
				Config: cfg,
				URLs:   args,
				DryRun: flags.dryRun,
				Force:  flags.force,
			})
			if err != nil {
				return err
			}

			return writeWikiResult(result, output.GetFormat(cmd))
		},
	}
	cmd.Flags().BoolVar(&flags.force, "force", false, "Re-digest explicitly given already digested URLs (default skips them)")
	addWikiFlags(cmd, &flags)

	return cmd
}

func newWikiDigestCmd() *cobra.Command {
	var flags wikiFlags
	cmd := &cobra.Command{
		Use:   wikiDigestCommandName,
		Short: "Digest wiki/inbox.md URLs and flush handled lines",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := wikiuc.LoadConfig(flags.config, flags.wikiRoot)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			resolveWikiAPIKey(cfg)
			applyWikiFlagOverrides(cfg, &flags)

			result, err := wikiuc.RunDigest(context.Background(), wikiuc.DigestInput{
				Config: cfg,
				DryRun: flags.dryRun,
			})
			if err != nil {
				return err
			}

			return writeWikiResult(result, output.GetFormat(cmd))
		},
	}
	addWikiFlags(cmd, &flags)

	return cmd
}

func newWikiAuditCmd() *cobra.Command {
	var flags wikiFlags
	cmd := &cobra.Command{
		Use:   wikiAuditCommandName,
		Short: "Audit wiki entries for extraction and URL issues",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := wikiuc.LoadConfig(flags.config, flags.wikiRoot)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			resolveWikiAPIKey(cfg)

			result, err := wikiuc.RunAudit(context.Background(), wikiuc.AuditInput{
				Config:      cfg,
				RunCmd:      cmdutil.RunWithOutput,
				ChangedOnly: flags.changedOnly,
				Paths:       flags.auditPaths,
			})
			if err != nil {
				return err
			}

			return writeWikiAuditResult(result, output.GetFormat(cmd))
		},
	}
	cmd.Flags().StringVarP(&flags.config, "config", "c", "", "Config file path")
	cmd.Flags().StringVar(&flags.wikiRoot, "wiki-root", "", "Wiki root directory (overrides config)")
	cmd.Flags().BoolVar(&flags.changedOnly, "changed-only", false, "Audit changed wiki markdown files only")
	cmd.Flags().StringSliceVar(&flags.auditPaths, "paths", nil, "Audit only these wiki files or directories")

	return cmd
}

func newWikiCheckCmd() *cobra.Command {
	var flags struct {
		ghRoot   string
		wikiRoot string
	}
	cmd := &cobra.Command{
		Use:   wikiCheckCommandName,
		Short: "Check wiki/data/gh folder structure consistency",
		Long:  `Check that wiki/ and data/gh/ have matching folder structures.`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := workspaceuc.RunWikiCheck(workspaceuc.WikiCheckInput{
				GhRoot:   flags.ghRoot,
				WikiRoot: flags.wikiRoot,
			})
			if err != nil {
				return err
			}
			textDetails := fmt.Sprintf("summary: expected=%d actual=%d missing=%d extra=%d\n",
				len(result.ExpectedWikiDirs), len(result.ActualWikiDirs),
				len(result.MissingWikiDirs), len(result.ExtraWikiDirs))
			if err := writeCheckCommandOutput(output.GetFormat(cmd), &checkCommandOutput{
				Name:    "wiki check",
				Issues:  result.Issues,
				Summary: result.Summary(),
			}, textDetails); err != nil {
				return err
			}
			if checkutil.HasErrors(result.Issues) {
				return errors.New("wiki check failed")
			}

			return nil
		},
	}
	cmd.Flags().StringVar(&flags.ghRoot, "gh-root", "data/gh", "data/gh path")
	cmd.Flags().StringVar(&flags.wikiRoot, "wiki-root", "wiki", "wiki path")

	return cmd
}

type wikiCompactFlags struct {
	config           string
	wikiRoot         string
	model            string
	topHot           int
	bulkLogThreshold int
	minDeltaChars    int
	minDeltaLines    int
	sendMail         bool
	createIssue      bool
	dryRun           bool
	skipAI           bool
}

func newWikiStatsCmd() *cobra.Command {
	var flags struct {
		config   string
		wikiRoot string
		format   string
		top      int
	}
	cmd := &cobra.Command{
		Use:   wikiStatsCommandName,
		Short: "Wiki statistical overview (overview + research top N)",
		Long: `Scan the wiki tree and print ordered record sections ("section" + "data" rows).

Sections:
  overview        files / md / size
  research top<N> topics with the most type=research md files, descending

Output formats:
  tty   (default) — terminal render (go-pretty box tables)
  md              — Markdown source tables (GFM)
  json            — raw data contract (ordered keys, no rendering)
`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := wikiuc.LoadConfig(flags.config, flags.wikiRoot)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			root := wikiuc.ResolveWikiRoot(cfg)

			stats, err := wikiaudit.RunStats(wikiaudit.StatsOptions{
				WikiRoot: root,
				TopN:     flags.top,
				// ExcludeNames omitted → DefaultExcludeNames (operation artifacts).
			})
			if err != nil {
				return err
			}

			switch flags.format {
			case "json":
				b, err := stats.JSON()
				if err != nil {
					return err
				}
				_, err = fmt.Fprintln(cmd.OutOrStdout(), string(b))
				return err
			case "md":
				_, err := fmt.Fprintln(cmd.OutOrStdout(), stats.Markdown())
				return err
			case "tty", "":
				_, err := fmt.Fprintln(cmd.OutOrStdout(), stats.Terminal())
				return err
			default:
				return fmt.Errorf("unsupported format %q (tty|md|json)", flags.format)
			}
		},
	}
	cmd.Flags().StringVarP(&flags.config, "config", "c", "", "Config file path (wiki.yml)")
	cmd.Flags().StringVar(&flags.wikiRoot, "wiki-root", "", "Wiki root directory (overrides config)")
	cmd.Flags().StringVar(&flags.format, "format", "tty", "Output format (tty|md|json) — json is the raw data contract")
	cmd.Flags().IntVar(&flags.top, "top", 10, "Top N topics for the research section")

	return cmd
}

func newWikiCompactCmd() *cobra.Command {
	var flags wikiCompactFlags
	cmd := &cobra.Command{
		Use:   wikiCompactCommandName,
		Short: "Scheduled compact notice: hot log topics → AI → optional Resend + Linear",
		Long: `Identify hot wiki topics (substantive committed log.md edits in the schedule window),
ask AI whether a type:blog compact is warranted, and optionally deliver notices via Resend and/or a new Linear issue.
Topics must clear the heat gate (≥2 distinct edit days ∧ ≥2 commits, OR ≥2000 Δchars) to reach AI; AI is asked
to judge "is there a publishable blog" (default no), and duplicate/cooled topics are hard-excluded. When nothing
clears (all no / all rejected), the run is skipped with zero side effects.

Schedule is week-based, controlled by compact.schedule in the config (default 1 = weekly, 2 = every other week). A run fires only on the schedule day (Saturday) of an eligible week; any other day is skipped with zero side effects — actions may trigger daily and the CLI decides whether to run.

This command never writes blog or log.md. Compact still means you write type:blog manually.

Default is dry print (no side effects). Pass --send-mail (RESEND_TOKEN + compact.send.resend.mailTo) and/or --create-issue (LINEAR_API_KEY + compact.send.linear.teamKey).
Brand from compact.title (From, mail subject prefix, issue title). Each run always creates a new Linear issue ({title} [YYYY-MM-DD]); no dedup against open issues.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWikiCompact(cmd, &flags)
		},
	}
	cmd.Flags().StringVarP(&flags.config, "config", "c", "", "Config file path (wiki.yml)")
	cmd.Flags().StringVar(&flags.wikiRoot, "wiki-root", "", "Wiki root directory (overrides config)")
	cmd.Flags().IntVar(&flags.topHot, "top-hot", 10, "Max gate-passed hot topics to send to AI")
	cmd.Flags().IntVar(&flags.bulkLogThreshold, "bulk-log-threshold", 10, "Ignore commits touching this many log.md paths")
	cmd.Flags().IntVar(&flags.minDeltaChars, "min-delta-chars", 40, "Min non-whitespace char delta for substantive edit")
	cmd.Flags().IntVar(&flags.minDeltaLines, "min-delta-lines", 2, "Min non-empty line ± for substantive edit")
	cmd.Flags().BoolVar(&flags.sendMail, "send-mail", false, "Send Resend email")
	cmd.Flags().BoolVar(&flags.createIssue, "create-issue", false, "Create a new Linear issue with the compact report")
	cmd.Flags().BoolVar(&flags.dryRun, "dry-run", false, "Print result; do not send mail or create issue")
	cmd.Flags().BoolVar(&flags.skipAI, "skip-ai", false, "Skip AI (hot list only; for offline debug)")
	cmd.Flags().StringVar(&flags.model, "model", "", "AI model override")

	return cmd
}

func runWikiCompact(cmd *cobra.Command, flags *wikiCompactFlags) error {
	cfg, err := wikiuc.LoadConfig(flags.config, flags.wikiRoot)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	resolveWikiAPIKey(cfg)
	if flags.model != "" {
		cfg.AI.Model = flags.model
	}

	opts, err := buildCompactOptions(cfg, flags)
	if err != nil {
		return err
	}

	result, err := wikicompact.RunCompact(context.Background(), opts)
	// Always print bodies/status even when delivery partially failed.
	if printErr := printCompactResult(cmd.OutOrStdout(), result, flags); printErr != nil {
		if err != nil {
			return errors.Join(err, printErr)
		}
		return printErr
	}
	if err != nil {
		return err
	}
	if result != nil && result.SoftError != nil {
		return result.SoftError
	}
	return nil
}

func buildCompactOptions(cfg *wikiuc.Config, flags *wikiCompactFlags) (*wikicompact.CompactOptions, error) {
	token, mailTo := resolveCompactMail(cfg)
	if err := validateCompactMailFlags(flags, token, mailTo); err != nil {
		return nil, err
	}

	linearCfg := resolveCompactLinear(cfg)
	if err := validateCompactLinearFlags(flags, &linearCfg); err != nil {
		return nil, err
	}

	aiCfg := ai.ConfigWithOverrides(cfg.AI.APIKey, cfg.AI.BaseURL, cfg.AI.Model)
	if cfg.AI.Temperature > 0 {
		aiCfg.Temperature = cfg.AI.Temperature
	}

	return &wikicompact.CompactOptions{
		WikiRoot:         cfg.Wiki.WikiRoot,
		WindowFn: func(now time.Time) (wikicompact.Window, bool, string) {
			return wikicompact.ScheduleWindow(cfg.Compact.Schedule, now)
		},
		TopHot:           flags.topHot,
		BulkLogThreshold: flags.bulkLogThreshold,
		MinDeltaChars:    flags.minDeltaChars,
		MinDeltaLines:    flags.minDeltaLines,
		SendMail:         flags.sendMail,
		CreateIssue:      flags.createIssue,
		DryRun:           flags.dryRun,
		SkipAI:           flags.skipAI,
		Title:            wikicompact.CompactBrand(cfg.Compact.Title),
		AI:               aiCfg,
		Mail: wikicompact.MailConfig{
			Token:  token,
			MailTo: mailTo,
		},
		Linear: linearCfg,
	}, nil
}

func validateCompactMailFlags(flags *wikiCompactFlags, token string, mailTo []string) error {
	if !flags.sendMail || flags.dryRun {
		return nil
	}
	if token == "" {
		return errors.New("RESEND_TOKEN is required with --send-mail")
	}
	if len(mailTo) == 0 {
		return errors.New("resend mailTo is required (wiki.yml compact.send.resend.mailTo or RESEND_MAIL_TO)")
	}
	return nil
}

func validateCompactLinearFlags(flags *wikiCompactFlags, linearCfg *wikicompact.LinearConfig) error {
	if !flags.createIssue || flags.dryRun {
		return nil
	}
	if linearCfg == nil || linearCfg.APIKey == "" {
		return errors.New("LINEAR_API_KEY is required with --create-issue")
	}
	if linearCfg.TeamKey == "" {
		return errors.New("linear teamKey is required (wiki.yml compact.send.linear.teamKey, default LUC)")
	}
	return nil
}

func resolveCompactMail(cfg *wikiuc.Config) (token string, mailTo []string) {
	mailTo = cfg.Compact.Send.Resend.MailTo
	if envTo := os.Getenv("RESEND_MAIL_TO"); envTo != "" {
		mailTo = mail.ParseAddresses(envTo)
	}
	return os.Getenv("RESEND_TOKEN"), mailTo
}

// resolveCompactLinear maps yaml/env into LinearConfig raw fields, then applies
// product defaults once via NormalizeLinearConfig (single source of defaults).
func resolveCompactLinear(cfg *wikiuc.Config) wikicompact.LinearConfig {
	lc := cfg.Compact.Send.Linear
	out := wikicompact.LinearConfig{
		APIKey:    os.Getenv("LINEAR_API_KEY"),
		TeamKey:   lc.TeamKey,
		StateName: lc.StateName,
		Assignee:  lc.Assignee,
		Priority:  lc.Priority,
	}
	wikicompact.NormalizeLinearConfig(&out)
	return out
}

func printCompactResult(w io.Writer, result *wikicompact.CompactResult, flags *wikiCompactFlags) error {
	if result == nil {
		return nil
	}
	if result.Skipped {
		_, err := fmt.Fprintf(w, "wiki compact skipped: %s\n", result.SkipReason)
		if err != nil {
			return err
		}
		// Heat transparency: when the run produced an empty list, print the
		// computation table so the user sees WHY each topic was rejected.
		if result.TextBody != "" {
			_, err = fmt.Fprint(w, "\n"+result.TextBody+"\n")
		}
		return err
	}
	if _, err := fmt.Fprint(w, result.TextBody); err != nil {
		return err
	}
	for _, line := range compactDeliveryStatusLines(result, flags) {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return nil
}

func compactDeliveryStatusLines(result *wikicompact.CompactResult, flags *wikiCompactFlags) []string {
	var lines []string

	switch {
	case result.MailSent:
		lines = append(lines, "mail: sent")
	case flags.sendMail && flags.dryRun:
		lines = append(lines, "mail: dry-run (not sent)")
	case !flags.sendMail:
		lines = append(lines, "mail: skipped (pass --send-mail to deliver)")
	}

	switch {
	case result.IssueCreated:
		line := "linear: created"
		if result.IssueIdentifier != "" {
			line += " " + result.IssueIdentifier
		}
		if result.IssueURL != "" {
			line += " " + result.IssueURL
		}
		lines = append(lines, line)
	case flags.createIssue && flags.dryRun:
		line := "linear: dry-run (not created)"
		if result.IssueTitle != "" {
			line += " title=" + result.IssueTitle
		}
		lines = append(lines, line)
	case flags.createIssue:
		lines = append(lines, "linear: not created (see error)")
	default:
		lines = append(lines, "linear: skipped (pass --create-issue to deliver)")
	}

	return lines
}

// resolveWikiAPIKey populates cfg.AI.APIKey from environment variables when unset.
func resolveWikiAPIKey(cfg *wikiuc.Config) {
	if cfg.AI.APIKey != "" {
		return
	}
	if v := os.Getenv("OPENAI_API_KEY"); v != "" {
		cfg.AI.APIKey = v
	} else if v := os.Getenv("LLM_AxonHub"); v != "" {
		cfg.AI.APIKey = v
	}
}

// applyWikiFlagOverrides applies CLI flag overrides to the loaded config.
func applyWikiFlagOverrides(cfg *wikiuc.Config, flags *wikiFlags) {
	if flags.model != "" {
		cfg.AI.Model = flags.model
	}
	if flags.ghTopicsURL != "" {
		cfg.Wiki.GHTopicsURL = flags.ghTopicsURL
	}
	if flags.maxContentSize > 0 {
		cfg.Wiki.MaxContentSize = flags.maxContentSize
	}
}

func addWikiFlags(cmd *cobra.Command, flags *wikiFlags) {
	cmd.Flags().StringVarP(&flags.config, "config", "c", "", "Config file path")
	cmd.Flags().StringVar(&flags.wikiRoot, "wiki-root", "", "Wiki root directory (overrides config)")
	cmd.Flags().StringVar(&flags.model, "model", "", "AI model override (e.g. deepseek-v3)")
	cmd.Flags().StringVar(&flags.ghTopicsURL, "gh-topics-url", "", "Remote gh.yml URL for topic candidates (default https://docs.lucc.dev/gh.yml)")
	cmd.Flags().IntVar(&flags.maxContentSize, "max-content-size", 0, "Max content chars sent to AI (default 20000)")
	cmd.Flags().BoolVar(&flags.dryRun, "dry-run", false, "Run fetch/classify without writing files or flushing inbox")
}

func writeWikiResult(result *wikiuc.Result, format string) error {
	out := &CommandOutput{
		Name:    result.Name,
		OK:      result.OK(),
		Summary: result.Summary(),
		Actions: result.Actions(),
		Results: result.URLResults,
	}

	if err := writeCommandOutput(format, out, formatWikiTextResult(result)); err != nil {
		return err
	}
	if !result.OK() {
		return fmt.Errorf("%s failed", result.Name)
	}

	return nil
}

func writeWikiAuditResult(result *wikiuc.AuditResult, format string) error {
	if err := writeCheckCommandOutput(format, &checkCommandOutput{
		Name:    result.Name,
		Issues:  result.Issues,
		Summary: result.Summary(),
	}, formatWikiAuditTextResult(result)); err != nil {
		return err
	}
	if !result.OK() {
		return fmt.Errorf("%s failed", result.Name)
	}

	return nil
}

func formatWikiTextResult(result *wikiuc.Result) string {
	summary := result.Summary()
	var out strings.Builder
	status := "passed"
	if !result.OK() {
		status = "failed"
	}
	fmt.Fprintf(&out, "%s %s\n", result.Name, status)
	fmt.Fprintf(&out,
		"summary: processed=%v succeeded=%v handledFailures=%v unhandledFailures=%v skipped=%v "+
			"written=%v flushed=%v wouldFlush=%v dryRun=%v\n",
		summary["processed"], summary["succeeded"], summary["handledFailures"], summary["unhandledFailures"],
		summary["skipped"], summary["written"], summary["flushed"], summary["wouldFlush"], summary["dryRun"])
	for i := range result.URLResults {
		item := &result.URLResults[i]
		fmt.Fprintf(&out, "%s %s", item.Status, item.URL)
		if item.OutputPath != "" {
			fmt.Fprintf(&out, " -> %s", item.OutputPath)
		}
		if item.TopicPath != "" {
			fmt.Fprintf(&out, " topic=%s", item.TopicPath)
		}
		if item.FailureType != "" {
			fmt.Fprintf(&out, " failure=%s", item.FailureType)
		}
		if item.Error != "" {
			fmt.Fprintf(&out, " error=%s", item.Error)
		}
		fmt.Fprintln(&out)
	}

	return out.String()
}

func formatWikiAuditTextResult(result *wikiuc.AuditResult) string {
	summary := result.Summary()
	status := "passed"
	if !result.OK() {
		status = "failed"
	}

	return fmt.Sprintf("%s %s\nsummary: issues=%v errors=%v warnings=%v\n",
		result.Name, status, summary["issues"], summary["errors"], summary["warnings"])
}
