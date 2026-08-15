package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/cmd/skx/internal/skx"
)

// DefaultStatsFile is where `skx route` writes routing statistics.
const DefaultStatsFile = "~/.claude/zzz-stats.json"

func newStatsCmd(flags *rootFlags) *cobra.Command {
	var (
		statsFile string
		asJSON    bool
		prune     bool
	)
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show routing statistics (read ~/.claude/zzz-stats.json)",
		// Read-only stats needs no references dir; --prune validates flags.dir
		// inside RunE. Shadow the root's dir-required pre-run for this command.
		PersistentPreRunE: func(*cobra.Command, []string) error { return nil },
		RunE: func(cmd *cobra.Command, args []string) error {
			path := expandHome(statsFile)
			entries, err := skx.LoadStats(path)
			if err != nil {
				return err
			}
			if prune {
				// --prune needs the references dir to detect ghost entries;
				// read-only stats does not.
				if err := validateReferencesDir(flags.dir); err != nil {
					return err
				}
				kept, removed := pruneGhosts(entries, flags.dir)
				entries = kept
				if err := skx.SaveStats(path, entries); err != nil {
					return err
				}
				fmt.Fprintf(os.Stderr, "pruned %d ghost entries (target yml missing)\n", len(removed))
			}
			rankings := skx.RankStats(entries, 0)
			if asJSON {
				return json.NewEncoder(os.Stdout).Encode(rankings)
			}
			writeStatsText(os.Stdout, rankings)
			return nil
		},
	}

	cmd.Flags().StringVar(&statsFile, "stats", DefaultStatsFile, "path to the stats file")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output as JSON")
	cmd.Flags().BoolVar(&prune, "prune", false, "drop entries whose target yml is missing (writes back)")
	return cmd
}

// pruneGhosts keeps only entries whose target resolves to an existing source
// yml under refsDir. Returns the kept entries and the removed names.
func pruneGhosts(entries []skx.StatsEntry, refsDir string) (kept []skx.StatsEntry, removed []string) {
	for _, e := range entries {
		if e.Target == "" {
			removed = append(removed, e.Name)
			continue
		}
		yml := filepath.Join(refsDir, e.Target+".yml")
		if _, err := os.Stat(yml); err != nil {
			removed = append(removed, e.Name)
			continue
		}
		kept = append(kept, e)
	}
	return kept, removed
}

func expandHome(p string) string {
	if len(p) > 1 && p[0] == '~' {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[1:])
		}
	}
	return p
}

func writeStatsText(out *os.File, rankings []skx.StatsRanking) {
	var sb strings.Builder
	if len(rankings) == 0 {
		sb.WriteString("(no stats yet)\n")
	} else {
		fmt.Fprintf(&sb, "%-4s %-16s %6s %6s  %-20s %s\n", "rank", "prompt", "count", "pct", "last_ts", "sug")
		for _, r := range rankings {
			fmt.Fprintf(&sb, "%-4d %-16s %6d %5.1f%%  %-20s %s\n", r.Rank, r.Name, r.Count, r.Percent, r.LastTs, r.Sug)
		}
	}
	if _, err := out.WriteString(sb.String()); err != nil {
		fmt.Fprintf(os.Stderr, "write stats output: %v\n", err)
	}
}
