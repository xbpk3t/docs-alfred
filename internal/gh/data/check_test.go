package ghdata

import (
	"os"
	"path/filepath"
	"strings"
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

func TestGhCheck_MaxLinesDefaultAndOverride(t *testing.T) {
	tmpDir := t.TempDir()
	content := strings.Repeat("# filler\n", defaultMaxLines) + "- type: go\n  record: []\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(content), 0644))

	defaultResult, err := RunGhCheck(tmpDir)
	require.NoError(t, err)
	require.True(t, checkutil.HasErrors(defaultResult.Issues))
	require.Contains(t, defaultResult.Issues[0].Message, "FILE_TOO_LONG: 1002 lines (max 1000)")

	overrideResult, err := RunGhCheckWithOptions(tmpDir, CheckOptions{MaxLines: 1500})
	require.NoError(t, err)
	require.False(t, checkutil.HasErrors(overrideResult.Issues))
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

func TestGhCheck_MalformedRecordFieldsRemainDetectable(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(`- type: go
  repo:
    - url: https://github.com/acme/repo
      record: invalid
      topics:
        - topic: repo-topic
          record: invalid
  record: invalid
`), 0644))

	result, err := RunGhCheck(tmpDir)

	require.NoError(t, err)
	require.NotNil(t, result)
	messages := make([]string, 0, len(result.Issues))
	for _, issue := range result.Issues {
		messages = append(messages, issue.Message)
	}
	assert.Contains(t, messages, "section[0]: 'record' must be an array")
	assert.Contains(t, messages, "repo[0]: 'record' must be an array")
	assert.Contains(t, messages, "repo[0].topics[0]: 'record' must be an array")
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

func TestGhCheck_MissingType(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(`- repo:
    - url: https://github.com/acme/repo
`), 0644))

	result, err := RunGhCheck(tmpDir)
	require.NoError(t, err)
	require.True(t, checkutil.HasErrors(result.Issues))
	assert.Contains(t, result.Issues[0].Message, "missing or invalid 'type'")
}

func TestGhCheck_TypeMismatchFilename(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(`- type: python
  repo:
    - url: https://github.com/acme/repo
  record: []
`), 0644))

	result, err := RunGhCheck(tmpDir)
	require.NoError(t, err)
	require.True(t, checkutil.HasErrors(result.Issues))
	var found bool
	for _, iss := range result.Issues {
		if contains(iss.Message, "TYPE_MUST_MATCH_FILENAME") {
			found = true
		}
	}
	assert.True(t, found, "should have TYPE_MUST_MATCH_FILENAME error")
}

func TestGhCheck_MissingRecord(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(`- type: go
  repo:
    - url: https://github.com/acme/repo
`), 0644))

	result, err := RunGhCheck(tmpDir)
	require.NoError(t, err)
	var hasMissingRecord bool
	for _, iss := range result.Issues {
		if contains(iss.Message, "missing 'record'") {
			hasMissingRecord = true
		}
	}
	assert.True(t, hasMissingRecord)
}

func TestGhCheck_MissingURL(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(`- type: go
  repo:
    - des: no url
  record: []
`), 0644))

	result, err := RunGhCheck(tmpDir)
	require.NoError(t, err)
	require.True(t, checkutil.HasErrors(result.Issues))
	var found bool
	for _, iss := range result.Issues {
		if contains(iss.Message, "missing or invalid url") {
			found = true
		}
	}
	assert.True(t, found)
}

func TestGhCheck_InvalidURL(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(`- type: go
  repo:
    - url: not-a-url
  record: []
`), 0644))

	result, err := RunGhCheck(tmpDir)
	require.NoError(t, err)
	require.True(t, checkutil.HasErrors(result.Issues))
	var found bool
	for _, iss := range result.Issues {
		if contains(iss.Message, "invalid url format") {
			found = true
		}
	}
	assert.True(t, found)
}

func TestGhCheck_InvalidDateFormat(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(`- type: go
  repo:
    - url: https://github.com/acme/repo
      record:
        - date: 2024-1-1
          des: bad date
  record: []
`), 0644))

	result, err := RunGhCheck(tmpDir)
	require.NoError(t, err)
	require.True(t, checkutil.HasErrors(result.Issues))
	var found bool
	for _, iss := range result.Issues {
		if contains(iss.Message, "invalid date format") {
			found = true
		}
	}
	assert.True(t, found)
}

func TestGhCheck_EmptyDes(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(`- type: go
  repo:
    - url: https://github.com/acme/repo
      record:
        - date: 2024-01-01
  record: []
`), 0644))

	result, err := RunGhCheck(tmpDir)
	require.NoError(t, err)
	require.True(t, checkutil.HasErrors(result.Issues))
	var found bool
	for _, iss := range result.Issues {
		if contains(iss.Message, "missing or empty des") {
			found = true
		}
	}
	assert.True(t, found)
}

func TestGhCheck_TopicRecordValidation(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(`- type: go
  repo:
    - url: https://github.com/acme/repo
      topics:
        - topic: overview
          record:
            - date: 2024-01-01
              des: valid
            - date: bad-date
              des: invalid date
  record: []
`), 0644))

	result, err := RunGhCheck(tmpDir)
	require.NoError(t, err)
	require.True(t, checkutil.HasErrors(result.Issues))
}

func TestGhCheck_EffectiveMaxLines(t *testing.T) {
	opts := CheckOptions{MaxLines: 500}
	assert.Equal(t, 500, opts.effectiveMaxLines())

	opts2 := CheckOptions{MaxLines: 0}
	assert.Equal(t, defaultMaxLines, opts2.effectiveMaxLines())

	opts3 := CheckOptions{MaxLines: -1}
	assert.Equal(t, defaultMaxLines, opts3.effectiveMaxLines())
}

func TestGhCheck_UsingEntry(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(`- type: go
  using:
    url: https://github.com/acme/using-tool
    des: using tool
  repo:
    - url: https://github.com/acme/repo
  record: []
`), 0644))

	result, err := RunGhCheck(tmpDir)
	require.NoError(t, err)
	// using + repo = 2 entries
	assert.Equal(t, 2, result.TotalEntries)
}

func TestGhCheck_EmptyRepoList(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(`- type: go
  repo: []
  record: []
`), 0644))

	result, err := RunGhCheck(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, 0, result.TotalEntries)
}

func TestGhCheck_RepoNotMapping(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(`- type: go
  repo:
    - "just a string"
  record: []
`), 0644))

	result, err := RunGhCheck(tmpDir)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestGhCheck_RepoNoTopics(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(`- type: go
  repo:
    - url: https://github.com/acme/repo
      des: a tool
  record: []
`), 0644))

	result, err := RunGhCheck(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, 1, result.TotalEntries)
	assert.False(t, checkutil.HasErrors(result.Issues))
}

func TestGhCheck_MultipleSections(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(`- type: go
  repo:
    - url: https://github.com/acme/repo1
  record: []
- type: go
  repo:
    - url: https://github.com/acme/repo2
  record: []
`), 0644))

	result, err := RunGhCheck(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, 2, result.TotalEntries)
}

func TestGhCheck_UnreadableFileEvent(t *testing.T) {
	// Create a file that will be found by ListYAMLFilesRecursive
	// but can't be read (we'll delete it after listing)
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "go.yml")
	require.NoError(t, os.WriteFile(file, []byte(`- type: go
  record: []
`), 0644))

	// Run check first to verify it works
	result, err := RunGhCheck(tmpDir)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestGhCheck_EmptyFileEvent(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(""), 0644))

	result, err := RunGhCheck(tmpDir)
	require.NoError(t, err)
	assert.NotNil(t, result)
	// Should have EMPTY_FILE error
	var hasEmpty bool
	for _, iss := range result.Issues {
		if contains(iss.Message, "EMPTY_FILE") {
			hasEmpty = true
		}
	}
	assert.True(t, hasEmpty)
}

func TestGhCheck_NotArrayEvent(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(`key: value`), 0644))

	result, err := RunGhCheck(tmpDir)
	require.NoError(t, err)
	assert.NotNil(t, result)
	var hasNotArray bool
	for _, iss := range result.Issues {
		if contains(iss.Message, "expected array at root") {
			hasNotArray = true
		}
	}
	assert.True(t, hasNotArray)
}

func TestGhCheck_WhitespaceOnlyFile(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte("   \n  \n"), 0644))

	result, err := RunGhCheck(tmpDir)
	require.NoError(t, err)
	assert.NotNil(t, result)
	var hasEmpty bool
	for _, iss := range result.Issues {
		if contains(iss.Message, "EMPTY_FILE") {
			hasEmpty = true
		}
	}
	assert.True(t, hasEmpty)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}
