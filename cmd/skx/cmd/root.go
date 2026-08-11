package cmd

import (
	"github.com/spf13/cobra"
)

// DefaultReferencesDir is the zzz references source of truth: the directory
// that holds references/**/*.yml. It currently points at the YAML-skills
// worktree inside dotfiles; switch it to the main checkout path once the data
// migrates there (single place to change).
const DefaultReferencesDir = "/Users/luck/Desktop/dotfiles/.worktrees/YAML-skills/home/base/AI/skills/zzz/references"

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
			return runRender(targetOrDir(flags, args), flags.dryRun)
		},
		SilenceUsage: true,
	}

	root.PersistentFlags().StringVar(&flags.dir, "dir", DefaultReferencesDir, "zzz references dir (default: "+DefaultReferencesDir+")")
	root.PersistentFlags().StringVar(&flags.schema, "schema", "", "prpt.yml schema path (default: auto-located above dir)")
	root.PersistentFlags().BoolVar(&flags.dryRun, "dry-run", false, "print what would be written without writing")

	root.AddCommand(newRenderCmd(flags))
	root.AddCommand(newCheckCmd(flags))
	root.AddCommand(newGraphCmd(flags))

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
