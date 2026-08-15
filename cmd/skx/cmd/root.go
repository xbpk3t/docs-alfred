package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// validateReferencesDir ensures --dir is set and points at an existing
// directory so commands fail with a clear error instead of a bare "walk: no
// such file" from deep inside the package. No machine-specific default path is
// baked into the binary: the references dir is required and explicit.
func validateReferencesDir(dir string) error {
	if dir == "" {
		return fmt.Errorf("references dir is required (--dir)")
	}
	fi, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("references dir %s: %w", dir, err)
	}
	if !fi.IsDir() {
		return fmt.Errorf("references dir %s is not a directory", dir)
	}
	return nil
}

type rootFlags struct {
	dir    string
	schema string
	dryRun bool
}

// Execute runs the skx root command.
func Execute() error {
	return newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	flags := &rootFlags{}

	root := &cobra.Command{
		Use:   "skx",
		Short: "Render, validate and graph zzz prompt YAML files",
		// Bare `skx` or `skx <path>` defaults to render.
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRender(targetOrDir(flags, args), flags.dryRun, flags.schema, flags.dir)
		},
		SilenceUsage:  true,
		SilenceErrors: true, // main prints the error once; avoids double-printing
	}

	root.PersistentFlags().StringVar(&flags.dir, "dir", "", "zzz references dir")
	_ = root.MarkFlagRequired("dir")

	// Fail fast with an actionable error when the references dir is missing or
	// points at a deleted worktree instead of a deep filesystem error.
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		return validateReferencesDir(flags.dir)
	}

	root.PersistentFlags().StringVar(&flags.schema, "schema", "", "prpt.yml schema path (default: auto-located above dir)")
	root.PersistentFlags().BoolVar(&flags.dryRun, "dry-run", false, "print what would be written without writing")

	root.AddCommand(newRenderCmd(flags))
	root.AddCommand(newCheckCmd(flags))
	root.AddCommand(newGraphCmd(flags))
	root.AddCommand(newStatsCmd(flags))
	root.AddCommand(newRouteCmd(flags))

	root.SetHelpCommand(&cobra.Command{Hidden: true})

	return root
}

// targetOrDir resolves a positional path, falling back to --dir.
func targetOrDir(flags *rootFlags, args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return flags.dir
}
