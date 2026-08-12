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
	)
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show routing statistics (read ~/.claude/zzz-stats.json)",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := expandHome(statsFile)
			entries, err := skx.LoadStats(path)
			if err != nil {
				return err
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
	return cmd
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
