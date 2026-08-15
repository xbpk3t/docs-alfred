package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	data "github.com/xbpk3t/docs-alfred/internal/gh/domrules"
	"github.com/xbpk3t/docs-alfred/pkg/carboninit"
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
	rootCmd.AddCommand(newDumpCmd(&dataPath))
	rootCmd.AddCommand(newGoodsCmd(&dataPath))
	rootCmd.AddCommand(schema.SchemaCmd(rootCmd))
	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})

	return rootCmd
}

func parseDataDomainArg(value string) (data.DataDomain, error) {
	domain := data.DataDomain(value)
	if _, ok := data.SpecForDomain(domain); !ok {
		return "", fmt.Errorf("unknown data domain %q", value)
	}

	return domain, nil
}

func writeOutput(s string) error {
	_, err := os.Stdout.WriteString(s)

	return err
}
