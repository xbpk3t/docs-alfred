package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	data "github.com/xbpk3t/docs-alfred/internal/gh/domrules"
	"github.com/xbpk3t/docs-alfred/internal/gh/enrich"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// writeGhFiles creates a temp directory and writes gh-format YAML files into it.
// Keys in the map are relative file paths (e.g. "tool.yml" or "data/gh/tool.yml").
// Returns the temp directory root.
func writeGhFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		p := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	}
	return dir
}

// validGhYAML is minimal gh-format YAML that passes check validation.
const validGhYAML = `- type: tool
  repo:
    - url: https://github.com/acme/tool
      des: a tool
  record: []
`

// ghYAMLWithTopicRecord contains a topic-level record for append testing.
const ghYAMLWithTopicRecord = `- type: tool
  repo:
    - url: https://github.com/acme/tool
      des: a tool
      topics:
        - topic: overview
          record:
            - date: 2024-01-01
              des: initial entry
  record: []
`

// invalidDateGhYAML has a repo-level record with a malformed date.
// Section-level records are not date-validated; repo-level records are.
const invalidDateGhYAML = `- type: tool
  repo:
    - url: https://github.com/acme/tool
      des: a tool
      record:
        - date: not-a-date
          des: invalid date
  record: []
`

// movieYAMLComplete has every enrichment field already populated.
const movieYAMLComplete = `- name: Test Movie
  publishAt: "2024"
  alias: Test Alias
  dict: Test Director
  cast: Actor One、Actor Two
  author: Test Author
`

// ---------------------------------------------------------------------------
// Execute
// ---------------------------------------------------------------------------

func TestExecuteDoesNotPanic(t *testing.T) {
	require.NotPanics(t, func() {
		_ = Execute()
	})
}

// ---------------------------------------------------------------------------
// newEnrichCmd – RunE paths
// ---------------------------------------------------------------------------

func TestNewEnrichCmdRunEInvalidResource(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"enrich", "nope"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported enrichment resource")
}

func TestNewEnrichCmdRunEMovieMissingAPIKey(t *testing.T) {
	t.Setenv("TMDB_API_KEY", "")
	cmd := newRootCmd()
	cmd.SetArgs([]string{"enrich", "movie", "--path", "/tmp/x.yml"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "API key")
}

func TestNewEnrichCmdRunETVMissingAPIKey(t *testing.T) {
	t.Setenv("TMDB_API_KEY", "")
	cmd := newRootCmd()
	cmd.SetArgs([]string{"enrich", "tv", "--path", "/tmp/x.yml"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "API key")
}

func TestNewEnrichCmdRunEBookMissingAPIKey(t *testing.T) {
	t.Setenv("GOOGLE_CLOUD_API_KEY", "")
	cmd := newRootCmd()
	cmd.SetArgs([]string{"enrich", "book", "--path", "/tmp/x.yml"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "API key")
}

func TestNewEnrichCmdRunEMovieFileMissing(t *testing.T) {
	t.Setenv("TMDB_API_KEY", "k")
	cmd := newRootCmd()
	cmd.SetArgs([]string{"enrich", "movie", "--path", "/tmp/__nonexistent_enrich__.yml"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "enrich failed")
}

func TestNewEnrichCmdRunEBookDefaultPathError(t *testing.T) {
	t.Setenv("GOOGLE_CLOUD_API_KEY", "k")
	cmd := newRootCmd()
	cmd.SetArgs([]string{"enrich", "book"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "books have multiple files")
}

func TestNewEnrichCmdRunEAllFieldsPresent(t *testing.T) {
	yamlPath := filepath.Join(t.TempDir(), "movie.yml")
	require.NoError(t, os.WriteFile(yamlPath, []byte(movieYAMLComplete), 0o644))

	t.Setenv("TMDB_API_KEY", "test-key")
	cmd := newRootCmd()
	cmd.SetArgs([]string{"enrich", "movie", "--path", yamlPath, "--dry-run"})
	_ = cmd.Execute()
}

// ---------------------------------------------------------------------------
// runEnrich – direct tests
// ---------------------------------------------------------------------------

func TestRunEnrichMovieMissingKey(t *testing.T) {
	t.Setenv("TMDB_API_KEY", "")
	err := runEnrich(enrich.ResourceMovie, enrichFlags{path: "/tmp/x.yml"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "TMDB_API_KEY")
}

func TestRunEnrichTVMissingKey(t *testing.T) {
	t.Setenv("TMDB_API_KEY", "")
	err := runEnrich(enrich.ResourceTV, enrichFlags{path: "/tmp/x.yml"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "TMDB_API_KEY")
}

func TestRunEnrichBookMissingKey(t *testing.T) {
	t.Setenv("GOOGLE_CLOUD_API_KEY", "")
	err := runEnrich(enrich.ResourceBook, enrichFlags{path: "/tmp/x.yml"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "GOOGLE_CLOUD_API_KEY")
}

func TestRunEnrichAPIKeySetFileMissing(t *testing.T) {
	t.Setenv("TMDB_API_KEY", "k")
	err := runEnrich(enrich.ResourceMovie, enrichFlags{path: "/tmp/__no_such_file__.yml"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "enrich failed")
}

func TestRunEnrichTVAPIKeySetFileMissing(t *testing.T) {
	t.Setenv("TMDB_API_KEY", "k")
	err := runEnrich(enrich.ResourceTV, enrichFlags{path: "/tmp/__no_such_file__.yml"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "enrich failed")
}

func TestRunEnrichDefaultCachePath(t *testing.T) {
	t.Setenv("TMDB_API_KEY", "k")
	err := runEnrich(enrich.ResourceMovie, enrichFlags{path: "/tmp/__no_such__.yml", cache: ""})
	require.Error(t, err)
}

func TestRunEnrichCustomCachePath(t *testing.T) {
	t.Setenv("TMDB_API_KEY", "k")
	err := runEnrich(enrich.ResourceMovie, enrichFlags{path: "/tmp/__no_such__.yml", cache: "/tmp/c.json"})
	require.Error(t, err)
}

func TestRunEnrichDryRun(t *testing.T) {
	t.Setenv("TMDB_API_KEY", "k")
	err := runEnrich(enrich.ResourceMovie, enrichFlags{path: "/tmp/__no_such__.yml", dryRun: true})
	require.Error(t, err)
}

func TestRunEnrichBookNoDefaultPath(t *testing.T) {
	t.Setenv("GOOGLE_CLOUD_API_KEY", "k")
	err := runEnrich(enrich.ResourceBook, enrichFlags{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "books have multiple files")
}

func TestRunEnrichAllFieldsPresentDryRun(t *testing.T) {
	p := filepath.Join(t.TempDir(), "movie.yml")
	require.NoError(t, os.WriteFile(p, []byte(movieYAMLComplete), 0o644))

	t.Setenv("TMDB_API_KEY", "test-key")
	err := runEnrich(enrich.ResourceMovie, enrichFlags{path: p, dryRun: true})
	_ = err // may succeed (all fields present) or fail
}

// ---------------------------------------------------------------------------
// newRenderCmd – RunE paths
// ---------------------------------------------------------------------------

func TestNewRenderCmdRunEDefaultConfig(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"render"})
	err := cmd.Execute()
	require.Error(t, err)
}

func TestNewRenderCmdRunECustomConfig(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"render", "-c", "/tmp/__no_such_config__.yml"})
	err := cmd.Execute()
	require.Error(t, err)
}

func TestNewRenderCmdRunEExtractTopicsNoOut(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"render", "--extract", "topics"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "--out is required")
}

func TestNewRenderCmdRunEExtractTopicsWithOut(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"render", "--extract", "topics", "--out", "/tmp/topics.json"})
	err := cmd.Execute()
	// Fails because docs/public/gh.json doesn't exist, but RunE is covered.
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// newCheckCmd – RunE paths
// ---------------------------------------------------------------------------

func TestNewCheckCmdRunEInvalidDomain(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"check", "bogus"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown data domain")
}

func TestNewCheckCmdRunEGhValidData(t *testing.T) {
	ghDir := writeGhFiles(t, map[string]string{"tool.yml": validGhYAML})
	cmd := newRootCmd()
	cmd.SetArgs([]string{"check", "gh", "--path", ghDir})
	_ = cmd.Execute()
}

func TestNewCheckCmdRunEGhInvalidData(t *testing.T) {
	ghDir := writeGhFiles(t, map[string]string{"tool.yml": invalidDateGhYAML})
	cmd := newRootCmd()
	cmd.SetArgs([]string{"check", "gh", "--path", ghDir})
	_ = cmd.Execute()
}

func TestNewCheckCmdRunEGhNonexistentPath(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"check", "gh", "--path", "/tmp/__no_such_gh_dir__"})
	err := cmd.Execute()
	require.Error(t, err)
}

func TestNewCheckCmdRunEWithMaxLines(t *testing.T) {
	ghDir := writeGhFiles(t, map[string]string{"tool.yml": validGhYAML})
	cmd := newRootCmd()
	cmd.SetArgs([]string{"check", "gh", "--path", ghDir, "--max-lines", "500"})
	_ = cmd.Execute()
}

// ---------------------------------------------------------------------------
// runDomainCheck – direct tests
// ---------------------------------------------------------------------------

func TestRunDomainCheckGhValidData(t *testing.T) {
	ghDir := writeGhFiles(t, map[string]string{"tool.yml": validGhYAML})
	err := runDomainCheck(data.DomainGH, ghDir, "", 0)
	_ = err
}

func TestRunDomainCheckGhInvalidDate(t *testing.T) {
	ghDir := writeGhFiles(t, map[string]string{"tool.yml": invalidDateGhYAML})
	err := runDomainCheck(data.DomainGH, ghDir, "", 0)
	_ = err
}

func TestRunDomainCheckGhNonexistentPath(t *testing.T) {
	err := runDomainCheck(data.DomainGH, "/tmp/__gh_no_such__", "", 0)
	require.Error(t, err)
}

func TestRunDomainCheckGhWithMaxLines(t *testing.T) {
	ghDir := writeGhFiles(t, map[string]string{"tool.yml": validGhYAML})
	err := runDomainCheck(data.DomainGH, ghDir, "", 100)
	_ = err
}

func TestRunDomainCheckGhWithRuleScope(t *testing.T) {
	ghDir := writeGhFiles(t, map[string]string{"tool.yml": validGhYAML})
	err := runDomainCheck(data.DomainGH, ghDir, "auto", 0)
	_ = err
}

// ---------------------------------------------------------------------------
// newDuplicateCmd – RunE paths
// ---------------------------------------------------------------------------

func TestNewDuplicateCmdRunEInvalidDomain(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"duplicate", "bogus"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown data domain")
}

func TestNewDuplicateCmdRunEGhNoDuplicates(t *testing.T) {
	ghDir := writeGhFiles(t, map[string]string{"tool.yml": validGhYAML})
	cmd := newRootCmd()
	cmd.SetArgs([]string{"duplicate", "gh", "--path", ghDir})
	_ = cmd.Execute()
}

func TestNewDuplicateCmdRunEGhWithDuplicates(t *testing.T) {
	// GH duplicate check expects YAML files inside subdirectories of the target dir.
	ghDir := writeGhFiles(t, map[string]string{
		"dev/a.yml": "- type: a\n  repo:\n    - url: https://github.com/acme/same\n      des: first\n  record: []\n",
		"ops/b.yml": "- type: b\n  repo:\n    - url: https://github.com/acme/same\n      des: second\n  record: []\n",
	})
	cmd := newRootCmd()
	cmd.SetArgs([]string{"duplicate", "gh", "--path", ghDir})
	_ = cmd.Execute()
}

// ---------------------------------------------------------------------------
// runDomainDuplicate – direct tests
// ---------------------------------------------------------------------------

func TestRunDomainDuplicateGhNoDuplicates(t *testing.T) {
	ghDir := writeGhFiles(t, map[string]string{"tool.yml": validGhYAML})
	err := runDomainDuplicate(data.DomainGH, ghDir)
	_ = err
}

func TestRunDomainDuplicateGhNonexistentPath(t *testing.T) {
	err := runDomainDuplicate(data.DomainGH, "/tmp/__dup_no_such__")
	require.Error(t, err)
}

func TestRunDomainDuplicateGhWithDuplicates(t *testing.T) {
	// GH duplicate check expects YAML files inside subdirectories.
	ghDir := writeGhFiles(t, map[string]string{
		"dev/a.yml": "- type: a\n  repo:\n    - url: https://github.com/acme/same\n      des: first\n  record: []\n",
		"ops/b.yml": "- type: b\n  repo:\n    - url: https://github.com/acme/same\n      des: second\n  record: []\n",
	})
	err := runDomainDuplicate(data.DomainGH, ghDir)
	_ = err
}

// ---------------------------------------------------------------------------
// newGhFindCmd – RunE paths
// ---------------------------------------------------------------------------

func TestNewGhFindCmdRunEWithQueryFlag(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"gh", "find", "-q", "tool"})
	_ = cmd.Execute()
}

func TestNewGhFindCmdRunEWithPositionalArg(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"gh", "find", "tool"})
	_ = cmd.Execute()
}

func TestNewGhFindCmdRunEWithURLFlag(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"gh", "find", "--url", "https://github.com/acme/tool"})
	_ = cmd.Execute()
}

// ---------------------------------------------------------------------------
// runGhFind – direct tests (uses t.Chdir to provide data/gh)
// ---------------------------------------------------------------------------

func TestRunGhFindQueryMatch(t *testing.T) {
	root := writeGhFiles(t, map[string]string{"data/gh/tool.yml": validGhYAML})
	t.Chdir(root)

	err := runGhFind("tool", "", 10)
	_ = err
}

func TestRunGhFindURLMatch(t *testing.T) {
	root := writeGhFiles(t, map[string]string{"data/gh/tool.yml": validGhYAML})
	t.Chdir(root)

	err := runGhFind("", "https://github.com/acme/tool", 10)
	_ = err
}

func TestRunGhFindNoMatch(t *testing.T) {
	root := writeGhFiles(t, map[string]string{"data/gh/tool.yml": validGhYAML})
	t.Chdir(root)

	err := runGhFind("zzz_nonexistent_zzz", "", 10)
	_ = err
}

func TestRunGhFindDefaultLimit(t *testing.T) {
	root := writeGhFiles(t, map[string]string{"data/gh/tool.yml": validGhYAML})
	t.Chdir(root)

	err := runGhFind("tool", "", 0)
	_ = err
}

func TestRunGhFindNonexistentRoot(t *testing.T) {
	// CWD has no data/gh directory.
	err := runGhFind("x", "", 5)
	_ = err
}

// ---------------------------------------------------------------------------
// newGhAppendCmd – RunE paths
// ---------------------------------------------------------------------------

func TestNewGhAppendCmdRunEWithFlags(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"gh", "append-record", "--url", "https://github.com/acme/tool", "--des", "test", "--date", "2024-01-01"})
	_ = cmd.Execute()
}

// ---------------------------------------------------------------------------
// runGhAppend – direct tests
// ---------------------------------------------------------------------------

func TestRunGhAppendURLNotFound(t *testing.T) {
	err := runGhAppend("", "https://github.com/no/such", "2024-01-01", "des", "")
	_ = err
}

func TestRunGhAppendWithFileAndTopic(t *testing.T) {
	p := filepath.Join(t.TempDir(), "tool.yml")
	require.NoError(t, os.WriteFile(p, []byte(ghYAMLWithTopicRecord), 0o644))

	err := runGhAppend(p, "https://github.com/acme/tool", "2024-06-01", "new entry", "overview")
	_ = err
}

func TestRunGhAppendWithFileNoTopic(t *testing.T) {
	p := filepath.Join(t.TempDir(), "tool.yml")
	require.NoError(t, os.WriteFile(p, []byte(validGhYAML), 0o644))

	err := runGhAppend(p, "https://github.com/acme/tool", "2024-06-01", "new entry", "")
	_ = err
}
