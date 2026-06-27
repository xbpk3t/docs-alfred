package output

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
)

// Stdout format values.
const (
	FormatText = "text"
	FormatJSON = "json"
)

const FormatFlagName = "format"

// FormatEnvVar is the environment variable that overrides --format.
const FormatEnvVar = "OUTPUT_FORMAT"

// FormatFlag registers a persistent --format flag on root and binds it to target.
// allowed is the list of accepted values; if empty, only text and json are accepted.
func FormatFlag(root *cobra.Command, target *string, defaultFmt string, allowed []string, description string) {
	root.PersistentFlags().StringVar(target, FormatFlagName, defaultFmt, description)
}

// GetFormat returns the resolved format for cmd.
// Priority: explicit --format flag > OUTPUT_FORMAT env var > flag default.
func GetFormat(cmd *cobra.Command) string {
	f := cmd.Flags().Lookup(FormatFlagName)
	if f == nil {
		return ""
	}

	// If the flag was explicitly passed on the command line, use it.
	if cmd.Flags().Changed(FormatFlagName) {
		return f.Value.String()
	}

	// Otherwise, check the environment variable.
	if env := os.Getenv(FormatEnvVar); env != "" {
		return env
	}

	return f.Value.String()
}

// NormalizeFormat validates and normalizes the format string.
// Empty input returns defaultFmt. Returns error for unsupported values.
func NormalizeFormat(format, defaultFmt string, allowed ...string) (string, error) {
	if format == "" {
		return defaultFmt, nil
	}

	format = strings.ToLower(strings.TrimSpace(format))

	for _, a := range allowed {
		if format == a {
			return format, nil
		}
	}

	return "", fmt.Errorf("unsupported output format %q (allowed: %s)", format, strings.Join(allowed, ", "))
}

// WriteJSON marshals v as indented JSON and writes to stdout.
func WriteJSON(v any) error {
	data, err := fileutil.MarshalJSON(v)
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}

	_, err = os.Stdout.WriteString(string(data) + "\n")

	return err
}
