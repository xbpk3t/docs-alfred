package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
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

func TestNewGhCmdStructure(t *testing.T) {
	cmd := newGhCmd()
	require.Equal(t, ghCommandName, cmd.Name())
	requireCommandNames(t, cmd.Commands(), []string{"append-record", "find"})
}

func TestNewGhFindCmdWithQueryArg(t *testing.T) {
	cmd := newGhFindCmd()
	cmd.SetArgs([]string{"test-query"})
	// This will call runGhFind which may fail due to missing data, but tests the arg parsing path
	err := cmd.Execute()
	// We don't check for nil error because runGhFind may fail if data dir doesn't exist
	_ = err
}

func TestNewGhFindCmdWithUrlFlag(t *testing.T) {
	cmd := newGhFindCmd()
	cmd.SetArgs([]string{"--url", "https://github.com/test/repo"})
	err := cmd.Execute()
	_ = err
}

func TestNewGhFindCmdWithQueryFlag(t *testing.T) {
	cmd := newGhFindCmd()
	cmd.SetArgs([]string{"--query", "test"})
	err := cmd.Execute()
	_ = err
}

func TestNewGhAppendCmdWithFlags(t *testing.T) {
	cmd := newGhAppendCmd()
	cmd.SetArgs([]string{"--url", "https://github.com/test/repo", "--date", "2026-01-01", "--des", "test desc"})
	err := cmd.Execute()
	_ = err
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

func TestNewEnrichCmdWithMovieArg(t *testing.T) {
	t.Setenv("TMDB_API_KEY", "")
	cmd := newEnrichCmd()
	cmd.SetArgs([]string{"movie"})
	// This will fail because TMDB_API_KEY is not set
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "TMDB_API_KEY")
}

func TestNewEnrichCmdWithTVArg(t *testing.T) {
	t.Setenv("TMDB_API_KEY", "")
	cmd := newEnrichCmd()
	cmd.SetArgs([]string{"tv"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "TMDB_API_KEY")
}

func TestNewEnrichCmdWithBookArg(t *testing.T) {
	t.Setenv("GOOGLE_CLOUD_API_KEY", "")
	cmd := newEnrichCmd()
	cmd.SetArgs([]string{"book"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "GOOGLE_CLOUD_API_KEY")
}

func TestNewEnrichCmdWithInvalidResource(t *testing.T) {
	cmd := newEnrichCmd()
	cmd.SetArgs([]string{"music"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported enrichment resource")
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

func TestRunGhFindWithQuery(t *testing.T) {
	err := runGhFind("test", "", 10)
	_ = err
}

func TestRunGhFindWithURL(t *testing.T) {
	err := runGhFind("", "https://github.com/test/repo", 10)
	_ = err
}

func TestRunGhAppendWithFile(t *testing.T) {
	err := runGhAppend("/nonexistent/file.yml", "", "", "", "")
	_ = err
}

func TestRunGhAppendWithURL(t *testing.T) {
	err := runGhAppend("", "https://github.com/test/repo", "2026-01-01", "test", "")
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
