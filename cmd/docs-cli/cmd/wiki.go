package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	workspaceuc "github.com/xbpk3t/docs-alfred/internal/docs/check"
	wikiuc "github.com/xbpk3t/docs-alfred/internal/docs/ingest"
	"github.com/xbpk3t/docs-alfred/pkg/cmdutil"
	"github.com/xbpk3t/docs-alfred/pkg/output"
)

type wikiFlags struct {
	config         string
	wikiRoot       string
	model          string
	auditPaths     []string
	maxContentSize int
	dryRun         bool
	changedOnly    bool
}

const (
	wikiCommandName       = "wiki"
	wikiDigestCommandName = "digest"
	wikiAuditCommandName  = "audit"
	wikiCheckCommandName  = "check"
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
				return errors.New("use `docs-cli wiki add <urls...>`, `docs-cli wiki digest`, or `docs-cli wiki digest-local`")
			}

			return cmd.Help()
		},
	}

	cmd.AddCommand(newWikiAddCmd())
	cmd.AddCommand(newWikiDigestCmd())
	cmd.AddCommand(newWikiDigestLocalCmd())
	cmd.AddCommand(newWikiAuditCmd())
	cmd.AddCommand(newWikiCheckCmd())

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

func newWikiDigestLocalCmd() *cobra.Command {
	var flags struct {
		config   string
		wikiRoot string
		fromDir  string
	}
	cmd := &cobra.Command{
		Use:   "digest-local",
		Short: "Classify and summarize local transcript files into wiki (from --from-dir)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.fromDir == "" {
				return errors.New("--from-dir is required")
			}
			cfg, err := wikiuc.LoadConfig(flags.config, flags.wikiRoot)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			resolveWikiAPIKey(cfg)

			result, err := wikiuc.RunDigestLocal(context.Background(), wikiuc.DigestLocalInput{
				Config:  cfg,
				FromDir: flags.fromDir,
			})
			if err != nil {
				return err
			}

			return writeWikiResult(result, "text")
		},
	}
	cmd.Flags().StringVarP(&flags.config, "config", "c", "", "Config file path")
	cmd.Flags().StringVar(&flags.wikiRoot, "wiki-root", "", "Wiki root directory (overrides config)")
	cmd.Flags().StringVar(&flags.fromDir, "from-dir", "", "Local directory containing BVxxx_title/ transcript folders")
	_ = cmd.MarkFlagRequired("from-dir")

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
			if workspaceuc.HasIssueErrors(result.Issues) {
				return errors.New("wiki check failed")
			}

			return nil
		},
	}
	cmd.Flags().StringVar(&flags.ghRoot, "gh-root", "data/gh", "data/gh path")
	cmd.Flags().StringVar(&flags.wikiRoot, "wiki-root", "wiki", "wiki path")

	return cmd
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
	if flags.maxContentSize > 0 {
		cfg.Wiki.MaxContentSize = flags.maxContentSize
	}
}

func addWikiFlags(cmd *cobra.Command, flags *wikiFlags) {
	cmd.Flags().StringVarP(&flags.config, "config", "c", "", "Config file path")
	cmd.Flags().StringVar(&flags.wikiRoot, "wiki-root", "", "Wiki root directory (overrides config)")
	cmd.Flags().StringVar(&flags.model, "model", "", "AI model override (e.g. deepseek-v3)")
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
		"summary: processed=%v succeeded=%v handledFailures=%v unhandledFailures=%v "+
			"written=%v flushed=%v wouldFlush=%v dryRun=%v\n",
		summary["processed"], summary["succeeded"], summary["handledFailures"], summary["unhandledFailures"],
		summary["written"], summary["flushed"], summary["wouldFlush"], summary["dryRun"])
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
