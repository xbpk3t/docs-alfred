package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

const (
	cmdCheck         = "check"
	outputFormatText = "text"
	outputFormatJSON = "json"
)

type checkCommandOutput struct {
	Summary map[string]any    `json:"summary,omitempty"`
	Name    string            `json:"name"`
	Issues  []checkutil.Issue `json:"issues"`
	Actions []string          `json:"actions,omitempty"`
	OK      bool              `json:"ok"`
}

func addFormatFlag(cmd *cobra.Command, target *string) {
	cmd.Flags().StringVar(target, "format", outputFormatText, "Output format: text or json")
}

func normalizeOutputFormat(format string) (string, error) {
	switch format {
	case "", outputFormatText:
		return outputFormatText, nil
	case outputFormatJSON:
		return outputFormatJSON, nil
	default:
		return "", fmt.Errorf("unsupported output format %q", format)
	}
}

func writeCheckCommandOutput(format string, output *checkCommandOutput, textDetails string) error {
	format, err := normalizeOutputFormat(format)
	if err != nil {
		return err
	}

	output.OK = !checkutil.HasErrors(output.Issues)
	if format == outputFormatJSON {
		data, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return err
		}

		return writeOutput(string(data))
	}

	report := (&checkutil.Result{Issues: output.Issues}).ReportResult(output.Name)
	if report != "" {
		fmt.Fprint(os.Stderr, report)
	}
	if textDetails != "" {
		fmt.Fprint(os.Stderr, textDetails)
		if textDetails[len(textDetails)-1] != '\n' {
			fmt.Fprintln(os.Stderr)
		}
	}
	if len(output.Actions) > 0 {
		fmt.Fprintln(os.Stderr, "[actions]")
		for _, action := range output.Actions {
			fmt.Fprintf(os.Stderr, "  %s\n", action)
		}
	}

	return nil
}

func writeJSONOutput(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}

	return writeOutput(string(data))
}

func writeOutput(s string) error {
	_, err := os.Stdout.WriteString(s + "\n")

	return err
}
