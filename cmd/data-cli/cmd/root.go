package cmd

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/internal/data/ops"
	data "github.com/xbpk3t/docs-alfred/internal/gh/domrules"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

type renderFlags struct {
	config  string
	extract string
	out     string
}

// Execute is the entry point for the data-cli binary.
func Execute() error {
	return newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "data-cli",
		Short: "Data rendering and validation commands",
	}

	rootCmd.AddCommand(newRenderCmd())
	rootCmd.AddCommand(newCheckCmd())
	rootCmd.AddCommand(newDuplicateCmd())
	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})

	return rootCmd
}

func newRenderCmd() *cobra.Command {
	var flags renderFlags

	cmd := &cobra.Command{
		Use:   "render",
		Short: "Render YAML data into outputs",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := dataops.RunRender(dataops.RenderInput{
				Config:  flags.config,
				Extract: flags.extract,
				Out:     flags.out,
			})

			return err
		},
	}

	cmd.Flags().StringVarP(&flags.config, "config", "c", "docs.yml", "Render config path")
	cmd.Flags().StringVar(&flags.extract, "extract", "", "Extract backbone: topics")
	cmd.Flags().StringVar(&flags.out, "out", "", "Output path for extracted backbone")

	return cmd
}

func newCheckCmd() *cobra.Command {
	var dataPath, ruleScope string
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

			return runDomainCheck(domain, dataPath, ruleScope, ghMaxLines)
		},
	}

	cmd.Flags().StringVar(&dataPath, "path", "", "Override data directory")
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

	checkutil.ReportIssues(result.Issues, "data check "+string(domain))
	if checkutil.HasErrors(result.Issues) {
		return fmt.Errorf("data check %s failed", domain)
	}

	return nil
}

func newDuplicateCmd() *cobra.Command {
	var dataPath string

	cmd := &cobra.Command{
		Use:   "duplicate <domain>",
		Short: "Find duplicate records for a domain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			domain, err := parseDataDomainArg(args[0])
			if err != nil {
				return err
			}

			return runDomainDuplicate(domain, dataPath)
		},
	}

	cmd.Flags().StringVar(&dataPath, "path", "", "Override data directory")

	return cmd
}

func parseDataDomainArg(value string) (data.DataDomain, error) {
	domain := data.DataDomain(value)
	if _, ok := data.SpecForDomain(domain); !ok {
		return "", fmt.Errorf("unknown data domain %q", value)
	}

	return domain, nil
}

func runDomainDuplicate(domain data.DataDomain, dataPath string) error {
	result, err := dataops.RunDomainDuplicate(dataops.DomainDuplicateInput{
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
		fmt.Fprint(os.Stderr, data.FormatGHDuplicateReport(report))

		return fmt.Errorf("data duplicate %s found %d duplicate URLs", domain, len(report.URLDuplicates))
	}
	fmt.Fprint(os.Stderr, data.FormatDuplicateReport(report))

	return fmt.Errorf("data duplicate %s found duplicates", domain)
}
