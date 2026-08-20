package domrules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

// The structured check now only serves the diary domain (books/ntl/goods are
// validated against their JSON Schemas, gh uses the walker). These tests cover
// the generic file/parse mechanics plus diary field validation.

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
	assert.Len(t, files, 2, "should find .yml and .yaml files, exclude hidden and subdirs")
}

func TestListYAMLFiles_NonExistentDir(t *testing.T) {
	files, err := listYAMLFiles("/tmp/nonexistent-dir-99999")
	require.Error(t, err)
	assert.Nil(t, files)
}

func TestCheckFile_NonExistent(t *testing.T) {
	issues := checkFile("/tmp/nonexistent-file.yml", "diary")
	assert.NotEmpty(t, issues)
	assert.Equal(t, checkutil.SeverityError, issues[0].Severity)
}

func TestCheckFile_EmptyContent(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "test.yml")
	require.NoError(t, os.WriteFile(file, []byte(""), 0644))

	issues := checkFile(file, "diary")
	assert.Empty(t, issues, "empty file should produce no issues")
}

func TestCheckFile_InvalidYAML(t *testing.T) {
	issues := checkYAMLContent(t, "diary.yml", "diary", "invalid: [yaml:\n")
	assertHasError(t, issues, true)
}

func TestCheckFile_TopLevelNotSequence(t *testing.T) {
	issues := checkYAMLContent(t, "diary.yml", "diary", "key: value\n")
	assert.NotEmpty(t, issues)
}

func TestResolveScopeDiary(t *testing.T) {
	// books/ntl/goods 走 schema;只有 diary 走 structured check → 恒为 ScopeDiary
	assert.Equal(t, ScopeDiary, ResolveScope("diary.yml", "diary"))
	assert.Equal(t, ScopeDiary, ResolveScope("books.yml", "books"))
	assert.Equal(t, ScopeDiary, ResolveScope("movie.yml", "auto"))
}

func TestAllowedFieldsForScopeDiary(t *testing.T) {
	fields := AllowedFieldsForScope(ScopeDiary)
	assert.True(t, fields["date"])
	assert.True(t, fields["review"])
	assert.True(t, fields["des"])
	assert.True(t, fields["score"])
	assert.True(t, fields["week"])
	assert.True(t, fields["url"])

	// 任意 scope 都返回 diary 字段集(结构化 check 只剩 diary)
	fields = AllowedFieldsForScope(RuleScope("unknown"))
	assert.True(t, fields["date"])
}

func TestCheckFile_DiaryValid(t *testing.T) {
	issues := checkYAMLContent(t, "diary.yml", "diary", "- date: 2024-01-01\n  review: 今天状态不错\n  score: 4\n  week: 32\n")
	assertHasError(t, issues, false)
}

func TestCheckFile_DiaryNoRequiredName(t *testing.T) {
	// diary 不强制 name 字段
	issues := checkYAMLContent(t, "diary.yml", "diary", "- date: 2024-01-01\n")
	assertHasError(t, issues, false)
}

func TestCheckFile_DiaryForbiddenField(t *testing.T) {
	issues := checkYAMLContent(t, "diary.yml", "diary", "- date: 2024-01-01\n  category: bad\n")
	assertHasError(t, issues, true)
}

func TestCheckFile_DiaryUndefinedField(t *testing.T) {
	// undefined 字段是 warn 级别(非 error),但会被标记
	issues := checkYAMLContent(t, "diary.yml", "diary", "- date: 2024-01-01\n  bogus: x\n")
	assert.NotEmpty(t, issues)
	assert.Contains(t, issues[0].Message, "未在规则中定义的字段")
}

func TestCheckFile_DiaryScoreRange(t *testing.T) {
	tests := []struct {
		score  string
		hasErr bool
	}{
		{"0", false}, {"5", false}, {"-1", true}, {"6", true}, {"high", true},
	}
	for _, tt := range tests {
		issues := checkYAMLContent(t, "diary.yml", "diary", "- date: 2024-01-01\n  score: "+tt.score+"\n")
		assertHasError(t, issues, tt.hasErr)
	}
}

func TestCheckFile_DiaryDateFull(t *testing.T) {
	tests := []struct {
		date   string
		hasErr bool
	}{
		{`"2024-01-01"`, false},
		{`"2024"`, true},
		{`"not-a-date"`, true},
		{"12345", true},
	}
	for _, tt := range tests {
		issues := checkYAMLContent(t, "diary.yml", "diary", "- date: "+tt.date+"\n  review: x\n")
		assertHasError(t, issues, tt.hasErr)
	}
}

func TestCheckFile_DiaryDesEmptyAllowed(t *testing.T) {
	// des 字段允许空值(不触发 empty value 警告)
	issues := checkYAMLContent(t, "diary.yml", "diary", "- date: 2024-01-01\n  des: \"\"\n")
	assertHasError(t, issues, false)
}

func TestCheckFile_DiaryNilScore(t *testing.T) {
	issues := checkYAMLContent(t, "diary.yml", "diary", "- date: 2024-01-01\n  score:\n")
	assertHasError(t, issues, true)
}

func TestRunStructuredDataCheckWithOptions(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "diary.yml"), []byte("- date: 2024-01-01\n  review: good\n"), 0644))

	result, err := RunStructuredDataCheckWithOptions(tmpDir, "diary", RunStructuredCheckOptions{})
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestRunStructuredDataCheck_WithIssues(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "diary.yml"), []byte("- date: 2024-01-01\n  bogus: x\n"), 0644))

	result, err := RunStructuredDataCheckWithOptions(tmpDir, "diary", RunStructuredCheckOptions{})
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Issues) // undefined 字段是 warn 级别
}

func TestCheckResult_HasErrors(t *testing.T) {
	assert.False(t, checkutil.HasErrors(nil))
	assert.False(t, checkutil.HasErrors([]checkutil.Issue{{Severity: checkutil.SeverityWarn}}))
	assert.True(t, checkutil.HasErrors([]checkutil.Issue{{Severity: checkutil.SeverityError}}))
}

func TestReportIssues(t *testing.T) {
	_, ok := checkutil.ReportIssues(nil, "test")
	assert.True(t, ok)
	_, ok = checkutil.ReportIssues([]checkutil.Issue{{Severity: checkutil.SeverityWarn, Message: "warn"}}, "test")
	assert.True(t, ok)
	_, ok = checkutil.ReportIssues([]checkutil.Issue{{Severity: checkutil.SeverityError, Message: "error"}}, "test")
	assert.False(t, ok)
}

func checkYAMLContent(t *testing.T, filename, scope, content string) []checkutil.Issue {
	t.Helper()

	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, filename)
	require.NoError(t, os.WriteFile(file, []byte(content), 0644))

	return checkFile(file, scope)
}

func assertHasError(t *testing.T, issues []checkutil.Issue, want bool) {
	t.Helper()
	assert.Equal(t, want, checkutil.HasErrors(issues), "issues: %#v", issues)
}
