package cmd

import (
	"log/slog"

	"github.com/spf13/cobra"
)

// Execute is the entry point for rss2nl.
// Root without subcommand shows help and exits 0 (does not send).
func Execute() {
	rootCmd := &cobra.Command{
		Use:   "rss2nl",
		Short: "RSS newsletter, transcription, source discovery and wiki tools",
		Long: `rss2nl: send newsletters, fetch transcripts, discover sources, and manage wiki entries.

Subcommands:
  send          Merge feeds and send newsletter
  trns          Fetch transcript data for a source
  trns check    Check transcript availability
  hunt          Discover high-quality source URLs
  wiki          Classify and summarize URLs into wiki knowledge base

Run "rss2nl <subcommand> --help" for more details.`,
	}

	rootCmd.AddCommand(newSendCmd())
	rootCmd.AddCommand(newTrnsCmd())
	rootCmd.AddCommand(newHuntCmd())
	rootCmd.AddCommand(newWikiCmd())

	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})

	if err := rootCmd.Execute(); err != nil {
		slog.Error("command execution failed", "error", err)
	}
}
