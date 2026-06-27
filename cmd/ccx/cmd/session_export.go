package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/cmd/ccx/internal"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
)

func newSessionExportCmd() *cobra.Command {
	var flags struct {
		config    string
		wikiRoot  string
		outputDir string
		session   string
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

			if flags.outputDir != "" {
				if ve := fileutil.ValidateOutputPath(flags.outputDir); ve != nil {
					return ve
				}
			}

			input := internal.ExportInput{
				DryRun:    flags.dryRun,
				Verbose:   flags.verbose,
				WikiRoot:  cfg.WikiRoot,
				OutputDir: flags.outputDir,
				SessionID: flags.session,
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
	cmd.Flags().StringVar(&flags.session, "session", "", "Session ID to export (overrides CLAUDE_CODE_SESSION_ID)")

	return cmd
}

func writeExportResult(result *internal.ExportResult) error {
	if result.DryRun {
		return writeLines("Dry run: would write to %s", result)
	}

	return writeLines("Exported session to %s", result)
}

func writeLines(prefix string, result *internal.ExportResult) error {
	lines := []struct{ f, v string }{
		{prefix, result.OutputPath},
		{"Topic: %s", result.TopicPath},
		{"Title: %s", result.Title},
		{"EngTitle: %s", result.EngTitle},
	}
	for _, l := range lines {
		if _, err := fmt.Fprintf(os.Stdout, l.f+"\n", l.v); err != nil {
			return err
		}
	}

	return nil
}
