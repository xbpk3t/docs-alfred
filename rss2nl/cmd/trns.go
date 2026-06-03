package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const defaultTrnsSource = "podcast"

type trnsFlags struct {
	asr      *bool
	publish  *bool
	outDir   string
	language string
	limit    int
	refresh  bool
	strict   bool
}

// newTrnsCmd creates `rss2nl trns` (with optional source positional arg)
// and `rss2nl trns check` (with optional source positional arg).
func newTrnsCmd() *cobra.Command {
	flags := &trnsFlags{}

	cmd := &cobra.Command{
		Use:       "trns [source]",
		Short:     "Fetch transcript data for a source",
		Long:      "Fetch transcript/transcription data for a source (e.g. podcast).",
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: []string{defaultTrnsSource},
		RunE: func(cmd *cobra.Command, args []string) error {
			source := defaultTrnsSource
			if len(args) > 0 {
				source = args[0]
			}

			return runTrns(source, flags)
		},
	}

	cmd.Flags().StringVar(&flags.outDir, "out", ".cache/rss2nl/trns", "Trns cache/output directory")
	cmd.Flags().IntVar(&flags.limit, "limit", 0, "Episodes to process per feed")
	cmd.Flags().BoolVar(&flags.refresh, "refresh", false, "Ignore existing cached trns data")
	flags.asr = cmd.Flags().Bool("asr", false, "Enable ASR fallback")
	cmd.Flags().StringVar(&flags.language, "language", "", "ASR language")
	flags.publish = cmd.Flags().Bool("publish", false, "Temporary upload")

	// rss2nl trns check [source]
	checkCmd := &cobra.Command{
		Use:       "check [source]",
		Short:     "Check transcript availability",
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: []string{defaultTrnsSource},
		RunE: func(cmd *cobra.Command, args []string) error {
			source := defaultTrnsSource
			if len(args) > 0 {
				source = args[0]
			}

			return runTrnsCheck(source, flags)
		},
	}
	checkCmd.Flags().StringVar(&flags.outDir, "out", ".cache/rss2nl/trns", "Trns cache/output directory")
	checkCmd.Flags().IntVar(&flags.limit, "limit", 0, "Episodes to inspect per feed")
	checkCmd.Flags().BoolVar(&flags.strict, "strict", false, "Exit non-zero when any trns feed fails")

	cmd.AddCommand(checkCmd)

	return cmd
}

func runTrns(source string, flags *trnsFlags) error {
	fmt.Fprintf(os.Stderr, "Fetching trns data for source=%q...\n", source)
	// TODO: full port from TS rss2nl/src/transcript/
	return nil
}

func runTrnsCheck(source string, flags *trnsFlags) error {
	fmt.Fprintf(os.Stderr, "Checking trns availability for source=%q...\n", source)
	// TODO: full port from TS rss2nl/src/transcript/
	return nil
}
