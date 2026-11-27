package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
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
				kept, removed, err := pruneGhosts(entries, flags.dir)
				if err != nil {
					return err
				}
				entries = kept
				if err := skx.SaveStats(path, entries); err != nil {
					return err
				}
				fmt.Fprintf(os.Stderr, "pruned %d ghost entries (target md missing)\n", len(removed))
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
	cmd.Flags().BoolVar(&prune, "prune", false, "drop entries whose target md is missing (writes back)")
	return cmd
}

// pruneGhosts keeps only entries whose target resolves to an existing source
// md under refsDir. Returns the kept entries and the removed names. Only
// fs.ErrNotExist counts as a ghost; any other filesystem error aborts the
// prune so a transient permission or I/O failure cannot permanently delete
// stats.
func pruneGhosts(entries []skx.StatsEntry, refsDir string) (kept []skx.StatsEntry, removed []string, err error) {
	for _, e := range entries {
		if e.Target == "" {
			removed = append(removed, e.Name)
			continue
		}
		exists, statErr := targetExists(refsDir, e.Target)
		if statErr != nil {
			return nil, nil, fmt.Errorf("prune %s: %w", e.Name, statErr)
		}
		if !exists {
			removed = append(removed, e.Name)
			continue
		}
		kept = append(kept, e)
	}
	return kept, removed, nil
}

// targetExists reports whether <refsDir>/<target>.md exists. The exact path is
// probed first; a directory scan then tolerates case-insensitive extensions
// (*.MD), matching CollectMarkdown routing. Errors other than fs.ErrNotExist
// are returned instead of being treated as a missing target.
func targetExists(refsDir, target string) (bool, error) {
	exact := filepath.Join(refsDir, target+".md")
	if _, err := os.Stat(exact); err == nil {
		return true, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}

	parent := filepath.Dir(exact)
	dirents, err := os.ReadDir(parent)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	want := filepath.Base(target)
	for _, d := range dirents {
		if d.IsDir() {
			continue
		}
		name := d.Name()
		if strings.TrimSuffix(name, filepath.Ext(name)) != want {
			continue
		}
		if !strings.EqualFold(filepath.Ext(name), ".md") {
			continue
		}
		if _, err := os.Stat(filepath.Join(parent, name)); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	}
	return false, nil
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
