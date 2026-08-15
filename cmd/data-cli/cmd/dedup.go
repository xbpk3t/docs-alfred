package cmd

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/internal/data/ops"
	data "github.com/xbpk3t/docs-alfred/internal/gh/domrules"
)

func newDedupCmd(dataPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dedup <domain>",
		Short: "Find duplicate records for a domain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			domain, err := parseDataDomainArg(args[0])
			if err != nil {
				return err
			}

			return runDomainDedup(domain, *dataPath)
		},
	}

	return cmd
}

func runDomainDedup(domain data.DataDomain, dataPath string) error {
	result, err := dataops.RunDomainDedup(dataops.DomainDedupInput{
		Domain: domain,
		Path:   dataPath,
	})
	if err != nil {
		return err
	}

	report := result.Report
	if len(report.URLDuplicates) == 0 && len(report.NameAuthorDuplicates) == 0 {
		slog.Info("Data duplicate passed", "domain", domain)

		return nil
	}
	if domain == data.DomainGH {
		if err := writeOutput(data.FormatGHDuplicateReport(report)); err != nil {
			return err
		}

		return fmt.Errorf("data duplicate %s found %d duplicate URLs", domain, len(report.URLDuplicates))
	}
	if err := writeOutput(data.FormatDuplicateReport(report)); err != nil {
		return err
	}

	return fmt.Errorf("data duplicate %s found duplicates", domain)
}
