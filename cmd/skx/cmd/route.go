package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/cmd/skx/internal/skx"
)

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
				names, namesErr := skx.AvailableNames(dir)
				if namesErr != nil {
					return namesErr
				}
				msg := fmt.Sprintf("ERROR: unknown subcommand: %s\nAvailable: %v\n", name, names)
				_, werr := cmd.ErrOrStderr().Write([]byte(msg))
				return werr
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
