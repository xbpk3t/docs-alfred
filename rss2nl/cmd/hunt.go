package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// newHuntCmd creates `rss2nl hunt`.
func newHuntCmd() *cobra.Command {
	var opts struct {
		reportHTML  string
		config      string
		providers   string
		reportMd    string
		state       string
		reportJSON  string
		category    []string
		blocked     []string
		max         int
		providerMax int
		seedLimit   int
		perCat      int
		newOnly     bool
		dryRun      bool
		sendMail    bool
	}

	cmd := &cobra.Command{
		Use:   "hunt",
		Short: "Discover high-quality source URLs",
		Long:  "Discover high-quality source URLs via Exa/Tavily providers and generate review reports.",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(os.Stderr, "Running hunt with state=%q...\n", opts.state)
			// TODO: full port from TS rss2nl/src/hunt/
			return nil
		},
	}

	cmd.Flags().StringVarP(&opts.config, "config", "c", "rss2nl.yml", "配置文件路径")
	cmd.Flags().StringVar(&opts.state, "state", ".cache/rss2nl/hunt/feeds-hunt-state.json", "State file path")
	cmd.Flags().StringArrayVar(&opts.category, "category", nil, "Category to scan")
	cmd.Flags().StringVar(&opts.providers, "providers", "", "Providers: exa,tavily")
	cmd.Flags().IntVar(&opts.max, "max", 0, "Global candidate cap")
	cmd.Flags().IntVar(&opts.perCat, "per-category", 0, "Candidate cap per category")
	cmd.Flags().IntVar(&opts.providerMax, "provider-max", 0, "Raw candidates per provider per category")
	cmd.Flags().IntVar(&opts.seedLimit, "seed-limit", 0, "Seed source cap per category")
	cmd.Flags().StringVar(&opts.reportMd, "report-md", ".cache/rss2nl/hunt/feeds-hunt-report.md", "Markdown report")
	cmd.Flags().StringVar(&opts.reportHTML, "report-html", ".cache/rss2nl/hunt/feeds-hunt-report.html", "HTML report")
	cmd.Flags().StringVar(&opts.reportJSON, "report-json", ".cache/rss2nl/hunt/feeds-hunt-report.json", "JSON report")
	cmd.Flags().StringArrayVar(&opts.blocked, "blocked-domain", nil, "Extra blocked domain")
	cmd.Flags().BoolVar(&opts.newOnly, "new-only", false, "Only accept candidates not in state")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "Write reports only")
	cmd.Flags().BoolVar(&opts.sendMail, "send-mail", false, "Send HTML report through Resend")

	return cmd
}
