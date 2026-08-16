package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/cmd/skx/internal/skx"
)

// writeRouteError prints the unresolved-name branch to stderr.
func writeRouteError(cmd *cobra.Command, name, dir string) error {
	names, err := skx.AvailableNames(dir)
	if err != nil {
		return err
	}
	msg := fmt.Sprintf("ERROR: unknown subcommand: %s\nAvailable: %v\n", name, names)
	_, werr := cmd.ErrOrStderr().Write([]byte(msg))
	return werr
}

func newRouteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "route <name>",
		Short: "Resolve a prompt name to its YAML path (replaces zzz.nu)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := flags.dir
			if dir == "" {
				dir = defaultDir()
			}
			name := args[0]
			// route always resolves against a dir (the positional is a name,
			// not a path), so validate it here rather than a deep walk error.
			if err := validateReferencesDir(dir); err != nil {
				return err
			}
			rel, ymlAbs, ok, resolveErr := skx.ResolvePrompt(dir, name)
			if resolveErr != nil {
				return resolveErr
			}
			if !ok {
				return writeRouteError(cmd, name, dir)
			}

			// Count the hit (write side of stats), best-effort.
			path := expandHome(DefaultStatsFile)
			if statsErr := skx.RecordHit(path, name, rel); statsErr != nil {
				_, werr := cmd.ErrOrStderr().Write([]byte("warning: stats: " + statsErr.Error() + "\n"))
				if werr != nil {
					return werr
				}
			}

			_, werr := cmd.OutOrStdout().Write([]byte(ymlAbs + "\n"))
			return werr
		},
	}
	return cmd
}
