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
		Short: "Validate zzz prompt markdown files",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := targetOrDir(flags, args)

			res, err := skx.CheckDir(dir)
			if err != nil {
				return err
			}

			var out strings.Builder
			fmt.Fprintf(&out, "checked %d files (%d errors, %d warnings)\n", res.Files, res.Errors, res.Warnings)
			for _, iss := range res.Issues {
				fmt.Fprintf(&out, "  %s: %s: %s\n", iss.Severity, iss.Path, iss.Message)
			}
			if _, err := os.Stdout.WriteString(out.String()); err != nil {
				return err
			}
			if res.HasErrors() {
				return fmt.Errorf("check failed with %d error(s)", res.Errors)
			}
			if _, err := os.Stdout.WriteString("ok\n"); err != nil {
				return err
			}
			return nil
		},
	}
}
