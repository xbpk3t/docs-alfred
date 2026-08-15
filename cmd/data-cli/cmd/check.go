package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/internal/data/ops"
	data "github.com/xbpk3t/docs-alfred/internal/gh/domrules"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

func newCheckCmd(dataPath *string) *cobra.Command {
	var ruleScope string
	var includeHidden bool

	cmd := &cobra.Command{
		Use:   "check <domain>",
		Short: "Check data validity for a domain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			domain, err := parseDataDomainArg(args[0])
			if err != nil {
				return err
			}

			return runDomainCheck(domain, *dataPath, ruleScope, includeHidden)
		},
	}

	cmd.Flags().StringVar(&ruleScope, "rule-scope", "", "Override structured data check rule scope")
	_ = cmd.Flags().MarkHidden("rule-scope")
	cmd.Flags().BoolVar(&includeHidden, "include-hidden", false, "Include hidden (dot-prefixed) YAML files in the check")

	return cmd
}

func runDomainCheck(domain data.DataDomain, dataPath, ruleScope string, includeHidden bool) error {
	result, err := dataops.RunDomainCheck(dataops.DomainCheckInput{
		Domain:        domain,
		Path:          dataPath,
		RuleScope:     ruleScope,
		IncludeHidden: includeHidden,
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
