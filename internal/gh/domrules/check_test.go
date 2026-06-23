package domrules

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

func TestCheckFile_ScoreValidation(t *testing.T) {
	tests := []struct {
		name   string
		score  string
		hasErr bool
	}{
		{"valid score 0", "0", false},
		{"valid score 5", "5", false},
		{"invalid score -1", "-1", true},
		{"invalid score 6", "6", true},
		{"non-int score", "high", true},
		{"float valid", "3.0", false},
		{"float invalid", "3.5", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := checkYAMLContent(t, "books.yml", "books", "- name: test\n  score: "+tt.score+"\n")
			assertHasError(t, issues, tt.hasErr)
		})
	}
}

func TestCheckFile_DateFullValidation(t *testing.T) {
	tests := []struct {
		name   string
		date   string
		hasErr bool
	}{
		{"valid date", "\"2024-01-01\"", false},
		{"invalid pattern", "\"2024\"", true},
		{"invalid date", "\"not-a-date\"", true},
		{"wrong type", "12345", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := checkYAMLContent(t, "books.yml", "books", "- name: test\n  readAt: "+tt.date+"\n")
			assertHasError(t, issues, tt.hasErr)
		})
	}
}

func TestCheckFile_YearValidation(t *testing.T) {
	tests := []struct {
		name   string
		year   string
		hasErr bool
	}{
		{"valid year string", "\"2024\"", false},
		{"valid year int", "2024", false},
		{"invalid year", "\"not-a-year\"", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := checkYAMLContent(t, "books.yml", "books", "- name: test\n  publishAt: "+tt.year+"\n")
			assertHasError(t, issues, tt.hasErr)
		})
	}
}

func TestCheckFile_SequenceValidation(t *testing.T) {
	issues := checkYAMLContent(t, "books.yml", "books", "- name: test\n  record:\n    - date: 2024-01-01\n")
	assertHasError(t, issues, false)

	issues = checkYAMLContent(t, "books.yml", "books", "- name: test\n  record: not-an-array\n")
	assertHasError(t, issues, true)
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

func TestResolveScopeAuto(t *testing.T) {
	assert.Equal(t, ScopeMovie, ResolveScope("movie.yml", "auto"))
	assert.Equal(t, ScopeMovie, ResolveScope("tv.yml", "auto"))
	assert.Equal(t, ScopeMusic, ResolveScope("music-jazz.yml", "auto"))
	assert.Equal(t, ScopeBooks, ResolveScope("unknown.yml", "auto"))
}

func TestResolveScopeExplicit(t *testing.T) {
	assert.Equal(t, ScopeBooks, ResolveScope("any.yml", "books"))
	assert.Equal(t, ScopeMovie, ResolveScope("any.yml", "movie"))
	assert.Equal(t, ScopeMovie, ResolveScope("any.yml", "tv"))
	assert.Equal(t, ScopeMusic, ResolveScope("any.yml", "music"))
	assert.Equal(t, ScopeDiary, ResolveScope("any.yml", "diary"))
	assert.Equal(t, ScopeJav, ResolveScope("jav.yml", "ntl"))
	assert.Equal(t, ScopeVG, ResolveScope("vg.yml", "ntl"))
	assert.Equal(t, ScopeMovie, ResolveScope("other.yml", "ntl"))
}

func TestAllowedFieldsForScope(t *testing.T) {
	fields := AllowedFieldsForScope(ScopeBooks)
	assert.True(t, fields["name"])
	assert.True(t, fields["author"])
	assert.True(t, fields["score"])

	fields = AllowedFieldsForScope(ScopeDiary)
	assert.True(t, fields["date"])
	assert.True(t, fields["review"])

	fields = AllowedFieldsForScope(ScopeMusic)
	assert.True(t, fields["perf"])

	fields = AllowedFieldsForScope(ScopeJav)
	assert.True(t, fields["rel"])

	fields = AllowedFieldsForScope(ScopeVG)
	assert.True(t, fields["developer"])

	// Unknown scope returns ContentFields
	fields = AllowedFieldsForScope(RuleScope("unknown"))
	assert.True(t, fields["name"])
}

func TestCheckResult_HasErrors(t *testing.T) {
	r := CheckResult{}
	assert.False(t, checkutil.HasErrors(r.Issues))

	r.Issues = append(r.Issues, checkutil.Issue{Severity: checkutil.SeverityWarn, Message: "warn"})
	assert.False(t, checkutil.HasErrors(r.Issues))

	r.Issues = append(r.Issues, checkutil.Issue{Severity: checkutil.SeverityError, Message: "error"})
	assert.True(t, checkutil.HasErrors(r.Issues))
}

func TestCheckFile_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "bad.yml")
	require.NoError(t, os.WriteFile(file, []byte("invalid: [yaml: broken\n"), 0644))

	issues := checkFile(file, "auto")
	assert.NotEmpty(t, issues)
	assert.Equal(t, checkutil.SeverityError, issues[0].Severity)
}

func TestCheckFile_TopLevelNotSequence(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "map.yml")
	require.NoError(t, os.WriteFile(file, []byte("key: value\n"), 0644))

	issues := checkFile(file, "auto")
	assert.NotEmpty(t, issues)
	assert.Contains(t, issues[0].Message, "顶层必须是列表")
}

func TestCheckFile_ForbiddenField(t *testing.T) {
	issues := checkYAMLContent(t, "books.yml", "books", "- name: test\n  category: bad\n")
	assertHasError(t, issues, true)
}

func TestCheckFile_UndefinedField(t *testing.T) {
	issues := checkYAMLContent(t, "books.yml", "books", "- name: test\n  unknownField: val\n")
	// Undefined fields produce warnings, not errors
	assert.NotEmpty(t, issues)
}

func TestCheckFile_EmptyFieldValue(t *testing.T) {
	issues := checkYAMLContent(t, "books.yml", "books", "- name: test\n  alias: \"\"\n")
	// Empty field value produces warning
	assert.NotEmpty(t, issues)
}

func TestCheckFile_MissingName(t *testing.T) {
	issues := checkYAMLContent(t, "books.yml", "books", "- score: 4\n")
	assertHasError(t, issues, true)
	assert.Contains(t, issues[0].Message, "缺少必填字段 name")
}

func TestCheckFile_SubField(t *testing.T) {
	issues := checkYAMLContent(t, "books.yml", "books", "- name: test\n  sub:\n    - name: sub-item\n      score: 3\n")
	assertHasError(t, issues, false)
}

func TestCheckFile_SubFieldNotArray(t *testing.T) {
	issues := checkYAMLContent(t, "books.yml", "books", "- name: test\n  sub: not-an-array\n")
	assertHasError(t, issues, true)
}

func TestCheckFile_ItemFieldNotArray(t *testing.T) {
	issues := checkYAMLContent(t, "books.yml", "books", "- name: test\n  item: not-an-array\n")
	assertHasError(t, issues, true)
}

func TestCheckFile_TagsNotArray(t *testing.T) {
	issues := checkYAMLContent(t, "books.yml", "books", "- name: test\n  tags: not-an-array\n")
	// Tags not being an array is a warning
	assert.NotEmpty(t, issues)
}

func TestCheckFile_TableNotArray(t *testing.T) {
	issues := checkYAMLContent(t, "books.yml", "books", "- name: test\n  table: not-an-array\n")
	assertHasError(t, issues, true)
}

func TestCheckFile_ReciteNotArray(t *testing.T) {
	issues := checkYAMLContent(t, "books.yml", "books", "- name: test\n  recite: not-an-array\n")
	assertHasError(t, issues, true)
}

func TestCheckFile_SubItemNotMapping(t *testing.T) {
	issues := checkYAMLContent(t, "books.yml", "books", "- name: test\n  sub:\n    - just a string\n")
	assertHasError(t, issues, true)
}

func TestCheckFile_SequenceItemNotMapping(t *testing.T) {
	issues := checkYAMLContent(t, "books.yml", "books", "- just a string\n")
	assertHasError(t, issues, true)
}

func TestCheckFile_PublishAtForDifferentScopes(t *testing.T) {
	// publishAt validation only applies to books, movie, jav, vg scopes
	issues := checkYAMLContent(t, "diary.yml", "diary", "- date: 2024-01-01\n  publishAt: 2024\n")
	// diary scope doesn't validate publishAt
	assertHasError(t, issues, false)
}

func TestRunStructuredDataCheck(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "books.yml"), []byte("- name: test\n  score: 4\n"), 0644))

	result, err := RunStructuredDataCheck(tmpDir, "books")
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestCheckFile_DateIntValue(t *testing.T) {
	// readAt as integer should produce error (must be string)
	issues := checkYAMLContent(t, "books.yml", "books", "- name: test\n  readAt: 20240101\n")
	assertHasError(t, issues, true)
}

func TestCheckFile_DateYearIntValue(t *testing.T) {
	// publishAt as integer should be valid for year scope
	issues := checkYAMLContent(t, "books.yml", "books", "- name: test\n  publishAt: 2024\n")
	assertHasError(t, issues, false)
}

func TestCheckFile_ScoreNilValue(t *testing.T) {
	// Score with nil value produces warning + error
	issues := checkYAMLContent(t, "books.yml", "books", "- name: test\n  score:\n")
	assertHasError(t, issues, true)
}

func TestCheckFile_ReadAtNilValue(t *testing.T) {
	issues := checkYAMLContent(t, "books.yml", "books", "- name: test\n  readAt:\n")
	// nil readAt produces warning for empty field
	assert.NotEmpty(t, issues)
}

func TestReportIssues(t *testing.T) {
	assert.True(t, checkutil.ReportIssues(nil, "test"))
	assert.True(t, checkutil.ReportIssues([]checkutil.Issue{{Severity: checkutil.SeverityWarn, Message: "warn"}}, "test"))
	assert.False(t, checkutil.ReportIssues([]checkutil.Issue{{Severity: checkutil.SeverityError, Message: "error"}}, "test"))
}

func TestCheckFile_DesEmptyValue(t *testing.T) {
	// des field is exempt from empty value warning
	issues := checkYAMLContent(t, "books.yml", "books", "- name: test\n  des: \"\"\n")
	assertHasError(t, issues, false)
}

func TestCheckFile_DiaryScope(t *testing.T) {
	// diary scope has no required 'name' field
	issues := checkYAMLContent(t, "diary.yml", "diary", "- date: 2024-01-01\n  review: good\n")
	assertHasError(t, issues, false)
}

func TestCheckFile_JavScope(t *testing.T) {
	// jav scope has no required 'name' field
	issues := checkYAMLContent(t, "jav.yml", "ntl", "- url: https://example.com\n  score: 3\n")
	assertHasError(t, issues, false)
}

func TestCheckFile_VGScopePublishAt(t *testing.T) {
	// vg scope validates publishAt as year
	issues := checkYAMLContent(t, "vg.yml", "ntl", "- name: game\n  publishAt: 2024\n")
	assertHasError(t, issues, false)
}

func TestCheckFile_NilMappingValue(t *testing.T) {
	// A mapping with a nil value for a field
	issues := checkYAMLContent(t, "books.yml", "books", "- name: test\n  score: \n  tags:\n    - a\n")
	// score nil triggers warning for empty field
	assert.NotEmpty(t, issues)
}

func TestCheckFile_MultiDoc(t *testing.T) {
	content := "---\n- name: first\n  score: 3\n---\n- name: second\n  score: 4\n"
	issues := checkYAMLContent(t, "books.yml", "books", content)
	assertHasError(t, issues, false)
}

func TestCheckFile_MultiDocWithNilBody(t *testing.T) {
	content := "---\n- name: first\n---\n---\n- name: second\n"
	issues := checkYAMLContent(t, "books.yml", "books", content)
	assertHasError(t, issues, false)
}

func TestRunStructuredDataCheck_WithIssues(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "bad.yml"), []byte("- score: 6\n"), 0644))

	result, err := RunStructuredDataCheck(tmpDir, "books")
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Issues)
}

func TestCheckFile_PublishAtScopeBooks(t *testing.T) {
	// publishAt validated for books scope
	issues := checkYAMLContent(t, "books.yml", "books", "- name: test\n  publishAt: 2024\n")
	assertHasError(t, issues, false)
}

func TestCheckFile_PublishAtScopeMovie(t *testing.T) {
	issues := checkYAMLContent(t, "movie.yml", "movie", "- name: test\n  publishAt: 2024\n")
	assertHasError(t, issues, false)
}

func TestCheckFile_ScoreIntegerNodeValid(t *testing.T) {
	issues := checkYAMLContent(t, "books.yml", "books", "- name: test\n  score: 3\n")
	assertHasError(t, issues, false)
}

func TestCheckFile_SubFieldWithNilKV(t *testing.T) {
	// sub item with a mapping that has empty key
	issues := checkYAMLContent(t, "books.yml", "books", "- name: test\n  sub:\n    - name: sub1\n")
	assertHasError(t, issues, false)
}
