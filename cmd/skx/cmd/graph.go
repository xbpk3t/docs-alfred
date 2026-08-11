package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/cmd/skx/internal/skx"
)

func newGraphCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "graph [dir]",
		Short: "Output the prompt dependency graph and cycles as JSON",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			g, skipped, err := skx.BuildGraph(targetOrDir(flags, args))
			if err != nil {
				return err
			}
			for _, f := range skipped {
				fmt.Fprintf(os.Stderr, "warning: skipped unparseable %s\n", f)
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(g)
		},
	}
}
