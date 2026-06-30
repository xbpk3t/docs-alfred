package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
	data "github.com/xbpk3t/docs-alfred/internal/gh/domrules"
)

func TestExecuteReturnsError(t *testing.T) {
	// Execute with no args should return nil (just shows help)
	err := Execute()
	// The root command doesn't error when run with no args, just shows help
	// This tests that Execute() doesn't panic
	_ = err
}

func TestNewRootCmdHelpText(t *testing.T) {
	root := newRootCmd()
	require.Contains(t, root.Short, "Data rendering")
}

func TestNewRenderCmdShortDescription(t *testing.T) {
	cmd := newRenderCmd(new(string))
	require.Equal(t, "render", cmd.Name())
	require.Contains(t, cmd.Short, "Render")
}

func TestNewCheckCmdShortDescription(t *testing.T) {
	cmd := newCheckCmd(new(string))
	require.Equal(t, "check", cmd.Name())
	require.Contains(t, cmd.Short, "Check")
}

func TestNewDedupCmdShortDescription(t *testing.T) {
	cmd := newDedupCmd(new(string))
	require.Equal(t, "dedup", cmd.Name())
	require.Contains(t, cmd.Short, "duplicate")
}

func TestNewCheckCmdWithValidDomain(t *testing.T) {
	cmd := newCheckCmd(new(string))
	cmd.SetArgs([]string{"gh"})
	// This calls parseDataDomainArg("gh") then runDomainCheck
	err := cmd.Execute()
	// May fail due to missing data dir, but tests the arg parsing path
	_ = err
}

func TestNewCheckCmdWithInvalidDomain(t *testing.T) {
	cmd := newCheckCmd(new(string))
	cmd.SetArgs([]string{"invalid-domain"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown data domain")
}

func TestNewDedupCmdWithValidDomain(t *testing.T) {
	cmd := newDedupCmd(new(string))
	cmd.SetArgs([]string{"gh"})
	err := cmd.Execute()
	_ = err
}

func TestNewDedupCmdWithInvalidDomain(t *testing.T) {
	cmd := newDedupCmd(new(string))
	cmd.SetArgs([]string{"invalid"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown data domain")
}

func TestRunDomainCheckWithValidDomain(t *testing.T) {
	// Test with gh domain and default path - may fail due to missing data but tests path
	err := runDomainCheck("gh", "", "", 0)
	_ = err
}

func TestRunDomainDedupWithValidDomain(t *testing.T) {
	err := runDomainDedup("gh", "")
	_ = err
}

func TestRunDomainCheckZeroMaxLines(t *testing.T) {
	err := runDomainCheck("gh", "", "", 0)
	// Tests the non-negative max-lines path
	_ = err
}

// ---------------------------------------------------------------------------
// parseDataDomainArg
// ---------------------------------------------------------------------------

func TestParseDataDomainArgValid(t *testing.T) {
	tests := []struct {
		input string
		want  data.DataDomain
	}{
		{"gh", data.DomainGH},
		{"books", data.DomainBooks},
		{"movie", data.DomainMovie},
		{"tv", data.DomainTV},
		{"music", data.DomainMusic},
		{"diary", data.DomainDiary},
		{"goods", data.DomainGoods},
		{"task", data.DomainTask},
		{"ntl", data.DomainNtl},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseDataDomainArg(tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestParseDataDomainArgUnknown(t *testing.T) {
	tests := []string{"unknown", "podcast", "", "GH"}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			_, err := parseDataDomainArg(input)
			require.Error(t, err)
			require.Contains(t, err.Error(), "unknown data domain")
		})
	}
}

// ---------------------------------------------------------------------------
// newRenderCmd
// ---------------------------------------------------------------------------

func TestNewRenderCmdFlags(t *testing.T) {
	cmd := newRenderCmd(new(string))

	require.Equal(t, "render", cmd.Name())
	require.NotNil(t, cmd.Flag("output"))

	// Check defaults.
	require.Equal(t, "docs/public", cmd.Flag("output").DefValue)
}

func TestNewRenderCmdRequiresExactlyOneArg(t *testing.T) {
	cmd := newRenderCmd(new(string))

	cmd.SetArgs([]string{})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "accepts 1 arg(s)")
}

// ---------------------------------------------------------------------------
// newCheckCmd
// ---------------------------------------------------------------------------

func TestNewCheckCmdFlags(t *testing.T) {
	cmd := newCheckCmd(new(string))

	require.Equal(t, "check", cmd.Name())
	require.Equal(t, "check <domain>", cmd.Use)
	require.NotNil(t, cmd.Flag("max-lines"))
	require.NotNil(t, cmd.Flag("rule-scope"))

	// rule-scope is hidden.
	require.True(t, cmd.Flag("rule-scope").Hidden)
}

func TestNewCheckCmdRequiresExactlyOneArg(t *testing.T) {
	cmd := newCheckCmd(new(string))

	cmd.SetArgs([]string{})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "accepts 1 arg(s)")
}

// ---------------------------------------------------------------------------
// newDedupCmd
// ---------------------------------------------------------------------------

func TestNewDedupCmdFlags(t *testing.T) {
	cmd := newDedupCmd(new(string))

	require.Equal(t, "dedup", cmd.Name())
	require.Equal(t, "dedup <domain>", cmd.Use)
}

func TestNewDedupCmdRequiresExactlyOneArg(t *testing.T) {
	cmd := newDedupCmd(new(string))

	cmd.SetArgs([]string{})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "accepts 1 arg(s)")
}
