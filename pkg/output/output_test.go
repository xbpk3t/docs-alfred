package output

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatFlag_Default(t *testing.T) {
	root := &cobra.Command{Use: "test"}
	var format string
	FormatFlag(root, &format, FormatText, []string{FormatText, FormatJSON}, "output format")

	cmd := &cobra.Command{Use: "sub", RunE: func(cmd *cobra.Command, args []string) error { return nil }}
	root.AddCommand(cmd)

	root.SetArgs([]string{"sub"})
	require.NoError(t, root.Execute())

	assert.Equal(t, FormatText, GetFormat(cmd))
	assert.Equal(t, FormatText, format)
}

func TestFormatFlag_Override(t *testing.T) {
	root := &cobra.Command{Use: "test"}
	var format string
	FormatFlag(root, &format, FormatText, []string{FormatText, FormatJSON}, "output format")

	cmd := &cobra.Command{Use: "sub", RunE: func(cmd *cobra.Command, args []string) error { return nil }}
	root.AddCommand(cmd)

	root.SetArgs([]string{"sub", "--format", "json"})
	require.NoError(t, root.Execute())

	assert.Equal(t, FormatJSON, GetFormat(cmd))
	assert.Equal(t, FormatJSON, format)
}

func TestGetFormat_NoFlag(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	assert.Equal(t, "", GetFormat(cmd))
}

func TestGetFormat_EnvVarFallback(t *testing.T) {
	root := &cobra.Command{Use: "test"}
	var format string
	FormatFlag(root, &format, FormatText, []string{FormatText, FormatJSON}, "output format")

	cmd := &cobra.Command{Use: "sub", RunE: func(cmd *cobra.Command, args []string) error { return nil }}
	root.AddCommand(cmd)

	t.Setenv(FormatEnvVar, "json")
	root.SetArgs([]string{"sub"})
	require.NoError(t, root.Execute())

	assert.Equal(t, FormatJSON, GetFormat(cmd))
}

func TestGetFormat_FlagOverridesEnvVar(t *testing.T) {
	root := &cobra.Command{Use: "test"}
	var format string
	FormatFlag(root, &format, FormatText, []string{FormatText, FormatJSON}, "output format")

	cmd := &cobra.Command{Use: "sub", RunE: func(cmd *cobra.Command, args []string) error { return nil }}
	root.AddCommand(cmd)

	t.Setenv(FormatEnvVar, "json")
	root.SetArgs([]string{"sub", "--format", "text"})
	require.NoError(t, root.Execute())

	assert.Equal(t, FormatText, GetFormat(cmd))
}

func TestNormalizeFormat(t *testing.T) {
	tests := []struct {
		format  string
		def     string
		allowed []string
		want    string
		wantErr bool
	}{
		{"", "text", []string{"text", "json"}, "text", false},
		{"json", "text", []string{"text", "json"}, "json", false},
		{"JSON", "text", []string{"text", "json"}, "json", false},
		{" json ", "text", []string{"text", "json"}, "json", false},
		{"yaml", "text", []string{"text", "json"}, "", true},
		{"alfred", "text", []string{"text", "json"}, "", true},
	}

	for _, tt := range tests {
		got, err := NormalizeFormat(tt.format, tt.def, tt.allowed...)
		if tt.wantErr {
			assert.Error(t, err)
		} else {
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		}
	}
}
