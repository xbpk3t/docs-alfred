package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// validateReferencesDir ensures the references dir is set and points at an
// existing directory, so commands fail with a clear error instead of a bare
// "walk: no such file" from deep inside the package. dir is the resolved
// path (positional arg, --dir, or defaultDir).
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
}

// Execute runs the skx root command.
func Execute() error {
	return newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	flags := &rootFlags{}

	root := &cobra.Command{
		Use:   "skx",
		Short: "Validate, route and graph zzz prompt YAML files",
		// Bare `skx` shows help; positional paths are handled by the
		// subcommands via targetOrDir.
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
		SilenceUsage:  true,
		SilenceErrors: true, // main prints the error once; avoids double-printing
	}

	root.PersistentFlags().StringVar(&flags.dir, "dir", "", "zzz references dir")

	// Fail fast with an actionable error when the references dir is missing or
	// points at a deleted worktree instead of a deep filesystem error.
	//
	// A positional target is validated by the command itself: route takes a
	// name and check takes a directory, so a positional is not necessarily a
	// directory. --dir is only validated when no positional is given; the
	// deployed default (defaultDir) is validated by the command's RunE.
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return nil
		}
		if flags.dir == "" {
			return nil
		}
		return validateReferencesDir(flags.dir)
	}

	root.PersistentFlags().StringVar(&flags.schema, "schema", "", "prpt.yml schema path (default: auto-located above dir)")

	root.AddCommand(newCheckCmd(flags))
	root.AddCommand(newGraphCmd(flags))
	root.AddCommand(newStatsCmd(flags))
	root.AddCommand(newRouteCmd(flags))

	root.SetHelpCommand(&cobra.Command{Hidden: true})

	return root
}

// defaultDir returns the fallback references dir when neither a positional
// path nor --dir is given: the nix-deployed zzz skill, resolved through the
// ~/.claude/skills/zzz symlink. Host-independent via $HOME (same convention on
// the Linux homelab). SKX_DIR overrides it for checking a different copy
// (e.g. the git source before a rebuild). --dir still always wins.
func defaultDir() string {
	if v := os.Getenv("SKX_DIR"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "skills", "zzz", "references")
}

// targetOrDir resolves a positional path, falling back to --dir, then to the
// deployed default (defaultDir).
func targetOrDir(flags *rootFlags, args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	if flags.dir != "" {
		return flags.dir
	}
	return defaultDir()
}
