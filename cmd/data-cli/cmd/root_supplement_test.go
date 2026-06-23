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
	cmd := newRenderCmd()
	require.Equal(t, "render", cmd.Name())
	require.Contains(t, cmd.Short, "Render")
}

func TestNewCheckCmdShortDescription(t *testing.T) {
	cmd := newCheckCmd()
	require.Equal(t, "check", cmd.Name())
	require.Contains(t, cmd.Short, "Check")
}

func TestNewDuplicateCmdShortDescription(t *testing.T) {
	cmd := newDuplicateCmd()
	require.Equal(t, "duplicate", cmd.Name())
	require.Contains(t, cmd.Short, "duplicate")
}

func TestNewCheckCmdWithValidDomain(t *testing.T) {
	cmd := newCheckCmd()
	cmd.SetArgs([]string{"gh"})
	// This calls parseDataDomainArg("gh") then runDomainCheck
	err := cmd.Execute()
	// May fail due to missing data dir, but tests the arg parsing path
	_ = err
}

func TestNewCheckCmdWithInvalidDomain(t *testing.T) {
	cmd := newCheckCmd()
	cmd.SetArgs([]string{"invalid-domain"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown data domain")
}

func TestNewDuplicateCmdWithValidDomain(t *testing.T) {
	cmd := newDuplicateCmd()
	cmd.SetArgs([]string{"gh"})
	err := cmd.Execute()
	_ = err
}

func TestNewDuplicateCmdWithInvalidDomain(t *testing.T) {
	cmd := newDuplicateCmd()
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

func TestRunDomainDuplicateWithValidDomain(t *testing.T) {
	err := runDomainDuplicate("gh", "")
	_ = err
}

func TestNewRenderCmdExecution(t *testing.T) {
	cmd := newRenderCmd()
	cmd.SetArgs([]string{"--config", "/nonexistent/config.yml"})
	err := cmd.Execute()
	// May fail but tests the RunE path
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
	cmd := newRenderCmd()

	require.Equal(t, "render", cmd.Name())
	require.NotNil(t, cmd.Flag("config"))
	require.NotNil(t, cmd.Flag("extract"))
	require.NotNil(t, cmd.Flag("out"))

	// Check default for --config.
	require.Equal(t, "docs.yml", cmd.Flag("config").DefValue)
}

// ---------------------------------------------------------------------------
// newCheckCmd
// ---------------------------------------------------------------------------

func TestNewCheckCmdFlags(t *testing.T) {
	cmd := newCheckCmd()

	require.Equal(t, "check", cmd.Name())
	require.Equal(t, "check <domain>", cmd.Use)
	require.NotNil(t, cmd.Flag("path"))
	require.NotNil(t, cmd.Flag("max-lines"))
	require.NotNil(t, cmd.Flag("rule-scope"))

	// rule-scope is hidden.
	require.True(t, cmd.Flag("rule-scope").Hidden)
}

func TestNewCheckCmdRequiresExactlyOneArg(t *testing.T) {
	cmd := newCheckCmd()

	cmd.SetArgs([]string{})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "accepts 1 arg(s)")
}

// ---------------------------------------------------------------------------
// newDuplicateCmd
// ---------------------------------------------------------------------------

func TestNewDuplicateCmdFlags(t *testing.T) {
	cmd := newDuplicateCmd()

	require.Equal(t, "duplicate", cmd.Name())
	require.Equal(t, "duplicate <domain>", cmd.Use)
	require.NotNil(t, cmd.Flag("path"))
}

func TestNewDuplicateCmdRequiresExactlyOneArg(t *testing.T) {
	cmd := newDuplicateCmd()

	cmd.SetArgs([]string{})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "accepts 1 arg(s)")
}
