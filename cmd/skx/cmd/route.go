package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/cmd/skx/internal/skx"
)

// writeRouteError prints the unresolved branch to stderr. It distinguishes an
// unknown name from a known prompt whose rendered .md is missing.
func writeRouteError(cmd *cobra.Command, name, dir string, known bool) error {
	var msg string
	if known {
		msg = fmt.Sprintf("ERROR: prompt %q exists but its .md is not rendered; run `skx render` first\n", name)
	} else {
		names, err := skx.AvailableNames(dir)
		if err != nil {
			return err
		}
		msg = fmt.Sprintf("ERROR: unknown subcommand: %s\nAvailable: %v\n", name, names)
	}
	_, werr := cmd.ErrOrStderr().Write([]byte(msg))
	return werr
}

func newRouteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "route <name>",
		Short: "Resolve a prompt name to its rendered .md path (replaces zzz.nu)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := flags.dir
			name := args[0]
			rel, mdAbs, ok, resolveErr := skx.ResolvePrompt(dir, name)
			if resolveErr != nil {
				return resolveErr
			}
			if !ok {
				return writeRouteError(cmd, name, dir, false)
			}

			// The rendered .md must exist to route to it (matches old zzz.nu).
			if _, statErr := os.Stat(mdAbs); statErr != nil {
				return writeRouteError(cmd, name, dir, true)
			}

			// Count the hit (write side of stats), best-effort.
			path := expandHome(DefaultStatsFile)
			if statsErr := skx.RecordHit(path, name, rel); statsErr != nil {
				_, werr := cmd.ErrOrStderr().Write([]byte("warning: stats: " + statsErr.Error() + "\n"))
				if werr != nil {
					return werr
				}
			}

			_, werr := cmd.OutOrStdout().Write([]byte(mdAbs + "\n"))
			return werr
		},
	}
	return cmd
}
