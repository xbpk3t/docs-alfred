package cmd

import (
	"log/slog"

	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/internal/data/ops"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
)

func newRenderCmd(dataPath *string) *cobra.Command {
	var outDir string

	cmd := &cobra.Command{
		Use:   "render <domain>",
		Short: "Render YAML data for a domain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			domain, err := parseDataDomainArg(args[0])
			if err != nil {
				return err
			}

			if ve := fileutil.ValidateOutputPath(outDir); ve != nil {
				return ve
			}

			result, err := dataops.RunDomainRender(dataops.DomainRenderInput{
				Domain: domain,
				Path:   *dataPath,
				OutDir: outDir,
			})
			if err != nil {
				return err
			}

			for _, f := range result.OutputFiles {
				slog.Info("Rendered", "domain", string(domain), "output", f)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&outDir, "output", "docs/public", "Output directory")

	return cmd
}
