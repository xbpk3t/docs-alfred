package data

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

func TestListYAMLFiles(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.yml"), []byte("key: val\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.yaml"), []byte("key: val\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "c.txt"), []byte("text"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".hidden.yml"), []byte("key: val\n"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "subdir"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "subdir", "d.yml"), []byte("key: val\n"), 0644))

	files, err := listYAMLFiles(dir)
	require.NoError(t, err)
	assert.Equal(t, 2, len(files), "should find .yml and .yaml files, exclude hidden and subdirs")
}

func TestListYAMLFiles_NonExistentDir(t *testing.T) {
	files, err := listYAMLFiles("/tmp/nonexistent-dir-99999")
	require.Error(t, err)
	assert.Nil(t, files)
}

func TestCheckFile_NonExistent(t *testing.T) {
	issues := checkFile("/tmp/nonexistent-file.yml", "auto")
	assert.Greater(t, len(issues), 0)
	assert.Equal(t, checkutil.SeverityError, issues[0].Severity)
}

func TestCheckFile_EmptyContent(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "test.yml")
	require.NoError(t, os.WriteFile(file, []byte(""), 0644))

	issues := checkFile(file, "auto")
	assert.Equal(t, 0, len(issues), "empty file should produce no issues")
}

func TestCheckScoreField(t *testing.T) {
	tests := []struct {
		name   string
		val    any
		hasErr bool
	}{
		{"valid score 0", 0, false},
		{"valid score 5", 5, false},
		{"invalid score -1", -1, true},
		{"invalid score 6", 6, true},
		{"non-int score", "high", true},
		{"float valid", float64(3), false},
		{"float invalid", float64(6), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := checkScoreField("test.yml", tt.val)
			if tt.hasErr {
				assert.Greater(t, len(issues), 0)
			} else {
				assert.Equal(t, 0, len(issues))
			}
		})
	}
}

func TestCheckDateFieldValue_DateFull(t *testing.T) {
	tests := []struct {
		name   string
		val    any
		hasErr bool
	}{
		{"valid date", "2024-01-01", false},
		{"invalid pattern", "2024", true},
		{"invalid date", "not-a-date", true},
		{"wrong type", 12345, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := checkDateFieldValue("test.yml", tt.val, "field", DateFull, "date")
			if tt.hasErr {
				assert.Greater(t, len(issues), 0)
			} else {
				assert.Equal(t, 0, len(issues))
			}
		})
	}
}

func TestCheckDateFieldValue_Year(t *testing.T) {
	tests := []struct {
		name   string
		val    any
		hasErr bool
	}{
		{"valid year", "2024", false},
		{"invalid year", "not-a-year", true},
		{"wrong type", 12345, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := checkDateFieldValue("test.yml", tt.val, "publishAt", DateYear, "year")
			if tt.hasErr {
				assert.Greater(t, len(issues), 0)
			} else {
				assert.Equal(t, 0, len(issues))
			}
		})
	}
}

func TestCheckIsSequence(t *testing.T) {
	issues := checkIsSequence("test.yml", []any{"a", "b"}, "items")
	assert.Equal(t, 0, len(issues))

	issues = checkIsSequence("test.yml", "not-an-array", "items")
	assert.Greater(t, len(issues), 0)
}

func TestResolveScopeAuto(t *testing.T) {
	assert.Equal(t, ScopeMovie, ResolveScope("movie.yml", "auto"))
	assert.Equal(t, ScopeMusic, ResolveScope("music-jazz.yml", "auto"))
	assert.Equal(t, ScopeBooks, ResolveScope("unknown.yml", "auto"))
}

func TestAllowedFieldsForScope(t *testing.T) {
	fields := AllowedFieldsForScope(ScopeBooks)
	assert.True(t, fields["name"])
	assert.True(t, fields["author"])
	assert.True(t, fields["score"])

	fields = AllowedFieldsForScope(ScopeDiary)
	assert.True(t, fields["date"])
	assert.True(t, fields["review"])
}

func TestCheckResult_HasErrors(t *testing.T) {
	r := CheckResult{}
	assert.False(t, checkutil.HasErrors(r.Issues))

	r.Issues = append(r.Issues, checkutil.Issue{Severity: checkutil.SeverityWarn, Message: "warn"})
	assert.False(t, checkutil.HasErrors(r.Issues))

	r.Issues = append(r.Issues, checkutil.Issue{Severity: checkutil.SeverityError, Message: "error"})
	assert.True(t, checkutil.HasErrors(r.Issues))
}

func TestReportIssues(t *testing.T) {
	assert.True(t, checkutil.ReportIssues(nil, "test"))
	assert.True(t, checkutil.ReportIssues([]checkutil.Issue{{Severity: checkutil.SeverityWarn, Message: "warn"}}, "test"))
	assert.False(t, checkutil.ReportIssues([]checkutil.Issue{{Severity: checkutil.SeverityError, Message: "error"}}, "test"))
}
