package ghdata

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

func TestIsValidURL(t *testing.T) {
	assert.True(t, isValidURL("https://github.com/owner/repo"))
	assert.True(t, isValidURL("http://example.com"))
	assert.True(t, isValidURL("https://gitlab.com/group/project.git"))
	assert.False(t, isValidURL(""))
	assert.False(t, isValidURL("not-a-url"))
	assert.False(t, isValidURL("://bad"))
}

func TestCheckResult_HasErrors(t *testing.T) {
	r := &CheckResult{}
	assert.False(t, checkutil.HasErrors(r.Issues))

	r.Issues = append(r.Issues, checkutil.Issue{Severity: "warn", Message: "warning"})
	assert.False(t, checkutil.HasErrors(r.Issues))

	r.Issues = append(r.Issues, checkutil.Issue{Severity: "error", Message: "error"})
	assert.True(t, checkutil.HasErrors(r.Issues))
}

func TestGhCheck_UnreadableDir(t *testing.T) {
	result, err := RunGhCheck("/tmp/nonexistent-gh-check-dir-67890")
	require.Error(t, err)
	require.NotNil(t, result, "RunGhCheck always returns a result")
}

func TestGhCheck_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	result, err := RunGhCheck(tmpDir)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 0, result.ScannedFiles)
}

func TestGhCheck_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	badFile := filepath.Join(tmpDir, "bad.yml")
	require.NoError(t, os.WriteFile(badFile, []byte("invalid: [yaml: broken\n"), 0644))

	result, err := RunGhCheck(tmpDir)
	require.Error(t, err, "YAML parse error should be propagated")
	assert.Contains(t, err.Error(), "yaml")
	require.NotNil(t, result)
}

func TestWalkGhRepos_TypedEvents(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(`- type: go
  topics:
    - topic: overview
      meta:
        slug: intro
        hasPic: true
  using:
    url: https://github.com/acme/tool
  repo:
    - url: https://github.com/acme/repo
      des: repo desc
      zk: repo zk
      topics:
        - topic: repo-topic
          record:
            - date: 2024-01-02
              des: shipped
  record: []
`), 0644))

	var section Section
	var repo Repo
	relations := make(map[string]bool)
	err := WalkGhRepos(tmpDir, func(ev WalkerEvent) error {
		switch ev.Type {
		case evSection:
			section = ev.Section
		case evRepo:
			if ev.Relation == evTypeRepo {
				repo = ev.Repo
			}
			relations[ev.Relation] = true
		}

		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, "go", section.Type)
	require.Len(t, section.Topics, 1)
	assert.Equal(t, "intro", section.Topics[0].Meta.Slug)
	assert.True(t, section.Topics[0].HasPicture())
	require.NotNil(t, section.Using)
	assert.Equal(t, "https://github.com/acme/tool", section.Using.URL)
	assert.Equal(t, "https://github.com/acme/repo", repo.URL)
	assert.Equal(t, "repo desc", repo.Des)
	require.Len(t, repo.Topics, 1)
	require.Len(t, repo.Topics[0].Record, 1)
	assert.Equal(t, "shipped", repo.Topics[0].Record[0].Des)
	assert.True(t, relations[evTypeRepo])
	assert.True(t, relations["using"])
}

func TestGhCheck_TypedRecordValidation(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(`- type: go
  repo:
    - url: https://github.com/acme/repo
      record:
        - date: 2024-01-02
          des: repo record
      topics:
        - topic: repo-topic
          record:
            - date: 2024-01-03
              des: topic record
  record: []
`), 0644))

	result, err := RunGhCheck(tmpDir)

	require.NoError(t, err)
	assert.Equal(t, 1, result.TotalEntries)
	assert.Equal(t, 2, result.TotalRecords)
	assert.False(t, checkutil.HasErrors(result.Issues))
}

func TestCheckResult_ReportResult(t *testing.T) {
	r := &CheckResult{}
	r.addIssue("file.yml", "warn", "test warning")
	r.addIssue("file.yml", "error", "test error")

	report := (&checkutil.Result{Issues: r.Issues}).ReportResult("test command")
	assert.Contains(t, report, "WARN file.yml: test warning")
	assert.Contains(t, report, "ERROR file.yml: test error")
	assert.Contains(t, report, "test command failed")
}
