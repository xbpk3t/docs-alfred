package cmd

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/internal/data/ops"
	data "github.com/xbpk3t/docs-alfred/internal/gh/domrules"
	"github.com/xbpk3t/docs-alfred/pkg/carboninit"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
	"github.com/xbpk3t/docs-alfred/pkg/output"
	"github.com/xbpk3t/docs-alfred/pkg/schema"
	"github.com/xbpk3t/docs-alfred/pkg/validator"
)

// Execute is the entry point for the data-cli binary.
func Execute() error {
	carboninit.Setup()
	validator.Setup()

	return newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	var dataPath, format string

	rootCmd := &cobra.Command{
		Use:   "data-cli",
		Short: "Data rendering and validation commands",
	}

	rootCmd.PersistentFlags().StringVar(&dataPath, "path", "", "Override data directory")
	output.FormatFlag(rootCmd, &format, output.FormatText, []string{output.FormatText, output.FormatJSON}, "Output format: text or json")

	rootCmd.AddCommand(newRenderCmd(&dataPath))
	rootCmd.AddCommand(newCheckCmd(&dataPath))
	rootCmd.AddCommand(newDedupCmd(&dataPath))
	rootCmd.AddCommand(schema.SchemaCmd(rootCmd))
	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})

	return rootCmd
}

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

func newCheckCmd(dataPath *string) *cobra.Command {
	var ruleScope string
	var ghMaxLines int

	cmd := &cobra.Command{
		Use:   "check <domain>",
		Short: "Check data validity for a domain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			domain, err := parseDataDomainArg(args[0])
			if err != nil {
				return err
			}

			return runDomainCheck(domain, *dataPath, ruleScope, ghMaxLines)
		},
	}

	cmd.Flags().IntVar(&ghMaxLines, "max-lines", 0, "Override data/gh maximum YAML file line count for gh checks")
	cmd.Flags().StringVar(&ruleScope, "rule-scope", "", "Override structured data check rule scope")
	_ = cmd.Flags().MarkHidden("rule-scope")

	return cmd
}

func runDomainCheck(domain data.DataDomain, dataPath, ruleScope string, ghMaxLines int) error {
	if ghMaxLines < 0 {
		return errors.New("--max-lines must be greater than or equal to 0")
	}

	result, err := dataops.RunDomainCheck(dataops.DomainCheckInput{
		Domain:     domain,
		Path:       dataPath,
		RuleScope:  ruleScope,
		GhMaxLines: ghMaxLines,
	})
	if err != nil {
		return err
	}

	report, ok := checkutil.ReportIssues(result.Issues, "data check "+string(domain))
	if report != "" {
		if err := writeOutput(report); err != nil {
			return err
		}
	}
	if !ok {
		return fmt.Errorf("data check %s failed", domain)
	}

	return nil
}

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

func parseDataDomainArg(value string) (data.DataDomain, error) {
	domain := data.DataDomain(value)
	if _, ok := data.SpecForDomain(domain); !ok {
		return "", fmt.Errorf("unknown data domain %q", value)
	}

	return domain, nil
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

func writeOutput(s string) error {
	_, err := os.Stdout.WriteString(s)

	return err
}
