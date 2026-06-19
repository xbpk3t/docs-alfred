package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/ccx/internal"
)

func newSessionExportCmd() *cobra.Command {
	var flags struct {
		config    string
		wikiRoot  string
		outputDir string
		dryRun    bool
		verbose   bool
	}

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export session to wiki",
		Long: `Export the current Claude Code session to wiki markdown.

This command:
1. Walks the session chain to collect all linked sessions
2. Calls cc2md for each session to get markdown
3. Merges all markdown into single file
4. AI classifies content to determine topic path
5. Writes to wiki/<topic>/YYYY-MM-DD-semantic-title.md`,
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := loadExportConfig(flags.config, exportConfigOverrides{
				WikiRoot: flags.wikiRoot,
			})
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			input := internal.ExportInput{
				DryRun:    flags.dryRun,
				Verbose:   flags.verbose,
				WikiRoot:  cfg.WikiRoot,
				OutputDir: flags.outputDir,
				AIConfig:  buildAIConfig(cfg),
			}

			result, err := internal.ExportSession(input)
			if err != nil {
				return fmt.Errorf("export session: %w", err)
			}

			return writeExportResult(result)
		},
	}

	cmd.Flags().StringVar(&flags.config, "config", "", "Config file path")
	cmd.Flags().BoolVar(&flags.dryRun, "dry-run", false, "Show what would be done without writing")
	cmd.Flags().BoolVar(&flags.verbose, "verbose", false, "Verbose output")
	cmd.Flags().StringVar(&flags.wikiRoot, "wiki-root", "", "Wiki root directory")
	cmd.Flags().StringVar(&flags.outputDir, "output-dir", "", "Output directory (overrides wiki-root)")

	return cmd
}

func writeExportResult(result *internal.ExportResult) error {
	if result.DryRun {
		if _, err := fmt.Fprintf(os.Stdout, "Dry run: would write to %s\n", result.OutputPath); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(os.Stdout, "Topic: %s\n", result.TopicPath); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(os.Stdout, "Title: %s\n", result.Title); err != nil {
			return err
		}

		return nil
	}

	if _, err := fmt.Fprintf(os.Stdout, "Exported session to %s\n", result.OutputPath); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(os.Stdout, "Topic: %s\n", result.TopicPath); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(os.Stdout, "Title: %s\n", result.Title); err != nil {
		return err
	}

	return nil
}
