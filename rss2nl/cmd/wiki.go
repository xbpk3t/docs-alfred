package cmd

import (
	"log/slog"

	"github.com/spf13/cobra"
)

// newWikiCmd creates `rss2nl wiki [urls...]`.
func newWikiCmd() *cobra.Command {
	var config, wikiRoot string
	var inbox bool

	cmd := &cobra.Command{
		Use:   "wiki [urls...]",
		Short: "Classify and summarize URLs into wiki knowledge base",
		Long: `Classify and summarize URLs into wiki knowledge base.
Use --inbox to process wiki/inbox.md. Pass URLs as positional args to process directly.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inbox {
				return runWikiInbox(config)
			}
			if len(args) == 0 {
				// No URL and no --inbox: warning + exit 0
				slog.Warn("rss2nl wiki: no URLs provided and --inbox not set. Doing nothing.")

				return nil
			}

			return runWikiURLs(config, args, wikiRoot)
		},
	}

	cmd.Flags().StringVarP(&config, "config", "c", "rss2nl.yml", "Config file path")
	cmd.Flags().StringVar(&wikiRoot, "wiki-root", "", "Wiki root directory (overrides config)")
	cmd.Flags().BoolVar(&inbox, "inbox", false, "Read URLs from wiki/inbox.md, process, and flush")

	return cmd
}

func runWikiInbox(config string) error {
	slog.Info("Processing wiki inbox", "config", config)
	// TODO: full port from TS rss2nl/src/wiki/
	return nil
}

func runWikiURLs(config string, urls []string, wikiRoot string) error {
	slog.Info("Processing wiki URLs", "count", len(urls), "config", config)
	// TODO: full port from TS rss2nl/src/wiki/
	return nil
}
