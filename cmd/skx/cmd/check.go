package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/cmd/skx/internal/skx"
)

func newCheckCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "check [dir]",
		Short: "Validate zzz prompt YAML against the prpt.yml schema",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := targetOrDir(flags, args)

			schema := flags.schema
			if schema == "" {
				var err error
				schema, err = skx.FindSchema(dir)
				if err != nil {
					return err
				}
			}

			res, err := skx.CheckDir(dir, schema)
			if err != nil {
				return err
			}

			out := make([]string, 0, len(res.Issues)+2)
			out = append(out, fmt.Sprintf("checked %d files against %s\n", res.Files, schema))
			for _, iss := range res.Issues {
				out = append(out, fmt.Sprintf("  %s: %s\n", iss.Path, iss.Message))
			}
			if _, err := os.Stdout.WriteString(strings.Join(out, "")); err != nil {
				return err
			}
			if res.HasErrors() {
				return fmt.Errorf("check failed with %d issue(s)", len(res.Issues))
			}
			if _, err := os.Stdout.WriteString("ok\n"); err != nil {
				return err
			}
			return nil
		},
	}
}
