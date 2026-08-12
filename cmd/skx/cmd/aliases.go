package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/cmd/skx/internal/skx"
)

func newAliasesCmd(flags *rootFlags) *cobra.Command {
	var out string
	cmd := &cobra.Command{
		Use:   "aliases [dir]",
		Short: "Generate aliases.json (name -> path) from zzz prompt YAML",
		Long: "Generate aliases.json mapping every prompt name to its references path,\n" +
			"from the .yml source files. Replaces gen-aliases.nu; output shape stays\n" +
			"compatible with what zzz.nu reads (a flat name -> dir/stem map).",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := targetOrDir(flags, args)
			aliases, err := skx.BuildAliases(dir)
			if err != nil {
				return err
			}

			data, err := json.MarshalIndent(aliases, "", "  ")
			if err != nil {
				return err
			}
			data = append(data, '\n')

			if out != "" {
				if writeErr := os.WriteFile(out, data, 0o600); writeErr != nil {
					return fmt.Errorf("write %s: %w", out, writeErr)
				}
				fmt.Fprintf(os.Stderr, "wrote %s (%d aliases)\n", out, len(aliases))
				return nil
			}
			_, err = os.Stdout.Write(data)
			return err
		},
	}

	cmd.Flags().StringVar(&out, "out", "", "write aliases.json to this path (default: stdout)")
	return cmd
}
