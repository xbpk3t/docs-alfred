package cmd

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/pkg/carboninit"
	"github.com/xbpk3t/docs-alfred/pkg/output"
	"github.com/xbpk3t/docs-alfred/pkg/schema"
	"github.com/xbpk3t/docs-alfred/pkg/validator"
)

// Execute is the entry point for rss2nl.
// Root without subcommand shows help and exits 0 (does not send).
func Execute() {
	carboninit.Setup()
	validator.Setup()

	var format string

	rootCmd := &cobra.Command{
		Use:   "rss2nl",
		Short: "RSS newsletter, transcription and source discovery tools",
		Long: `rss2nl: send newsletters, fetch transcripts, and discover sources.

Subcommands:
  send          Merge feeds and send newsletter
  trns          Fetch transcript data for a source
  trns check    Check transcript availability
  hunt          Discover high-quality source URLs

Run "rss2nl <subcommand> --help" for more details.`,
	}

	output.FormatFlag(rootCmd, &format, output.FormatText, []string{output.FormatText, output.FormatJSON}, "Output format: text or json")

	rootCmd.AddCommand(newSendCmd())
	rootCmd.AddCommand(newTrnsCmd())
	rootCmd.AddCommand(newHuntCmd())
	rootCmd.AddCommand(schema.SchemaCmd(rootCmd))
	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})

	if err := rootCmd.Execute(); err != nil {
		slog.Error("command execution failed", "error", err)
		os.Exit(1)
	}
}
