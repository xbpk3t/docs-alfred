package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
)

const (
	cmdCheck         = "check"
	outputFormatText = "text"
	outputFormatJSON = "json"
)

type checkCommandOutput = CommandOutput

// CommandOutput is the common docs-cli structured output envelope.
type CommandOutput struct {
	Results any               `json:"results,omitempty"`
	Summary map[string]any    `json:"summary,omitempty"`
	Name    string            `json:"name"`
	Issues  []checkutil.Issue `json:"issues,omitempty"`
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
		return writeJSONOutput(output)
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
	writeActions(output.Actions)

	return nil
}

func writeCommandOutput(format string, output *CommandOutput, textDetails string) error {
	format, err := normalizeOutputFormat(format)
	if err != nil {
		return err
	}
	if format == outputFormatJSON {
		return writeJSONOutput(output)
	}

	if textDetails != "" {
		fmt.Fprint(os.Stderr, textDetails)
		if textDetails[len(textDetails)-1] != '\n' {
			fmt.Fprintln(os.Stderr)
		}
	}
	writeActions(output.Actions)

	return nil
}

func writeActions(actions []string) {
	if len(actions) == 0 {
		return
	}

	fmt.Fprintln(os.Stderr, "[actions]")
	for _, action := range actions {
		fmt.Fprintf(os.Stderr, "  %s\n", action)
	}
}

func writeJSONOutput(v any) error {
	data, err := fileutil.MarshalJSON(v)
	if err != nil {
		return err
	}

	return writeOutput(string(data))
}

func writeOutput(s string) error {
	_, err := os.Stdout.WriteString(s + "\n")

	return err
}
