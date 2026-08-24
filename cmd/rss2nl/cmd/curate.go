package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/xbpk3t/docs-alfred/internal/rss/curate"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
	"github.com/xbpk3t/docs-alfred/pkg/output"
)

// newCurateCmd builds the `rss2nl curate` subcommand: turn an external seed
// list into candidate rss2nl.yml entries (preview-only; never appends).
func newCurateCmd() *cobra.Command {
	var opts struct {
		rss2nl string
		cfg    string
		out    string
		cache  string
		seed   []string
	}

	cmd := &cobra.Command{
		Use:   "curate",
		Short: "Curate external RSS seed lists into rss2nl candidate entries",
		Long: `Curate an external seed list into candidate rss2nl.yml entries.

Pipeline: seed -> (MAF resolve) -> fetch/frequency gate -> (MAF classify + per-type
curator) -> assemble preview (keep YAML / drops MD / summary). Only preview is
written under --out; nothing is appended to rss2nl.yml automatically.

Run "rss2nl curate --help" for flags.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCurate(&opts, output.GetFormat(cmd))
		},
	}

	cmd.Flags().StringVarP(&opts.rss2nl, "rss2nl", "r", "rss2nl.yml", "Path to rss2nl.yml (types + dedupe reference)")
	cmd.Flags().StringSliceVarP(&opts.seed, "seed", "s", nil, "Seed file(s) to curate (repeatable or comma-separated)")
	cmd.Flags().StringVarP(&opts.cfg, "config", "c", "", "Path to optional curate rules config")
	cmd.Flags().StringVar(&opts.out, "out", "", "Output directory (default <tmp>/rss-curate-<ts>)")
	cmd.Flags().StringVar(&opts.cache, "cache", fileutil.CachePath("rss2nl/curate/freq-cache.json"), "Fetch cache file")

	return cmd
}

// runCurate wires flags into curate.Run and reports the summary.
func runCurate(opts *struct {
	rss2nl string
	cfg    string
	out    string
	cache  string
	seed   []string
}, format string) error {
	if len(opts.seed) == 0 {
		return fmt.Errorf("curate requires at least one --seed file")
	}

	res, err := curate.Run(context.Background(), &curate.RunOptions{
		RSS2NLPath: opts.rss2nl,
		SeedPaths:  opts.seed,
		ConfigPath: opts.cfg,
		OutDir:     opts.out,
		CachePath:  opts.cache,
	})
	if err != nil {
		return fmt.Errorf("curate: %w", err)
	}

	slog.Info("curate complete",
		"resolved", res.Summary.Resolved,
		"freq_pass", res.Summary.FreqPass,
		"kept", res.Summary.Kept,
		"dropped", res.Summary.Dropped,
		"out", opts.out,
	)

	if format == output.FormatJSON {
		payload, merr := fileutil.MarshalJSON(res.Summary)
		if merr != nil {
			return merr
		}
		fmt.Fprintln(os.Stdout, string(payload)) //nolint:errcheck // CLI stdout write
	} else {
		fmt.Fprintf(os.Stdout, "resolved=%d freq_pass=%d kept=%d dropped=%d out=%s\n", //nolint:errcheck // CLI stdout write
			res.Summary.Resolved, res.Summary.FreqPass, res.Summary.Kept, res.Summary.Dropped, opts.out)
	}

	return nil
}
