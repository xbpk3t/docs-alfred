package domrules

import (
	"os"
	"path/filepath"
	"strings"
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
	assert.Len(t, files, 2, "should find .yml and .yaml files, exclude hidden and subdirs")
}

func TestListYAMLFiles_NonExistentDir(t *testing.T) {
	files, err := listYAMLFiles("/tmp/nonexistent-dir-99999")
	require.Error(t, err)
	assert.Nil(t, files)
}

func TestCheckFile_NonExistent(t *testing.T) {
	issues := checkFile("/tmp/nonexistent-file.yml", "auto")
	assert.NotEmpty(t, issues)
	assert.Equal(t, checkutil.SeverityError, issues[0].Severity)
}

func TestCheckFile_EmptyContent(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "test.yml")
	require.NoError(t, os.WriteFile(file, []byte(""), 0644))

	issues := checkFile(file, "auto")
	assert.Empty(t, issues, "empty file should produce no issues")
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
	_, ok := checkutil.ReportIssues(nil, "test")
	assert.True(t, ok)
	_, ok = checkutil.ReportIssues([]checkutil.Issue{{Severity: checkutil.SeverityWarn, Message: "warn"}}, "test")
	assert.True(t, ok)
	_, ok = checkutil.ReportIssues([]checkutil.Issue{{Severity: checkutil.SeverityError, Message: "error"}}, "test")
	assert.False(t, ok)
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

func TestCheckFile_GoodsTableDateInvalidFormat(t *testing.T) {
	issues := checkYAMLContent(t, "goods.EDC.yml", "goods", `- type: EDC
  tag: goods
  topics:
    - topic: x
      score: 5
      table:
        - name: a
          date: 2018-4-3
`)
	assertHasError(t, issues, true)
	require.True(t, containsIssue(issues, "date 必须是 YYYY-MM-DD"), "issues: %#v", issues)
}

func TestCheckFile_GoodsTableDateValid(t *testing.T) {
	issues := checkYAMLContent(t, "goods.EDC.yml", "goods", `- type: EDC
  tag: goods
  topics:
    - topic: x
      score: 5
      table:
        - name: a
          date: 2018-04-03
`)
	assertHasError(t, issues, false)
}

func TestCheckFile_GoodsTableScoreOutOfRange(t *testing.T) {
	issues := checkYAMLContent(t, "goods.EDC.yml", "goods", `- type: EDC
  tag: goods
  topics:
    - topic: x
      score: 5
      table:
        - name: a
          score: 99
`)
	assertHasError(t, issues, true)
	require.True(t, containsIssue(issues, "score 范围必须是 0-5"), "issues: %#v", issues)
}

func TestCheckFile_GoodsTableUndefinedField(t *testing.T) {
	issues := checkYAMLContent(t, "goods.EDC.yml", "goods", `- type: EDC
  tag: goods
  topics:
    - topic: x
      score: 5
      table:
        - name: a
          bogus: 1
`)
	require.Equal(t, 1, countIssues(issues, "未在规则中定义的字段: bogus"), "should warn once, not duplicate")
}

func TestCheckFile_GoodsMissingType(t *testing.T) {
	issues := checkYAMLContent(t, "goods.EDC.yml", "goods", `- tag: goods
  topics: []
`)
	assertHasError(t, issues, true)
	require.True(t, containsIssue(issues, "缺少必填字段 type"), "issues: %#v", issues)
}

func TestCheckFile_GoodsTableNonArray(t *testing.T) {
	issues := checkYAMLContent(t, "goods.EDC.yml", "goods", `- type: EDC
  tag: goods
  topics:
    - topic: x
      score: 5
      table: not-array
`)
	assertHasError(t, issues, true)
	require.True(t, containsIssue(issues, "table 必须是数组"), "issues: %#v", issues)
}

// helpers

func containsIssue(issues []checkutil.Issue, substr string) bool {
	for _, i := range issues {
		if strings.Contains(i.Message, substr) {
			return true
		}
	}

	return false
}

func countIssues(issues []checkutil.Issue, substr string) int {
	n := 0
	for _, i := range issues {
		if strings.Contains(i.Message, substr) {
			n++
		}
	}

	return n
}

func TestCheckFile_GoodsRecordStringItem(t *testing.T) {
	issues := checkYAMLContent(t, "goods.EDC.yml", "goods", `- type: EDC
  tag: goods
  topics:
    - topic: x
      score: 5
      table:
        - name: a
          record:
            - 1、【不喜欢硬板】
`)
	assertHasError(t, issues, true)
	require.True(t, containsIssue(issues, "record 项必须是对象"), "issues: %#v", issues)
}

func TestCheckFile_GoodsRecordValidMappings(t *testing.T) {
	issues := checkYAMLContent(t, "goods.EDC.yml", "goods", `- type: EDC
  tag: goods
  topics:
    - topic: x
      score: 5
      table:
        - name: a
          record:
            - date: 2024-12-06
              des: 换新
`)
	assertHasError(t, issues, false)
}

func TestCheckFile_GoodsRecordDateInvalidFormat(t *testing.T) {
	issues := checkYAMLContent(t, "goods.EDC.yml", "goods", `- type: EDC
  tag: goods
  topics:
    - topic: x
      score: 5
      table:
        - name: a
          record:
            - date: 2020-2-1
              des: x
`)
	assertHasError(t, issues, true)
	require.True(t, containsIssue(issues, "date 必须是 YYYY-MM-DD"), "issues: %#v", issues)
}

func TestCheckFile_GoodsUsingLegacyFlagged(t *testing.T) {
	// using is a legacy pre-migration key; it must be flagged as undefined.
	issues := checkYAMLContent(t, "goods.EDC.yml", "goods", `- type: EDC
  tag: goods
  topics:
    - topic: x
      score: 5
      using:
        name: a
        date: 2020-02-01
      table: []
`)
	assert.NotEmpty(t, issues)
	require.True(t, containsIssue(issues, "未在规则中定义的字段: using"), "issues: %#v", issues)
}

func TestCheckFile_GoodsScoreMinusOne(t *testing.T) {
	// goods allows -1 as "no rating" placeholder.
	issues := checkYAMLContent(t, "goods.EDC.yml", "goods", `- type: EDC
  tag: goods
  topics:
    - topic: x
      score: -1
      table: []
`)
	assertHasError(t, issues, false)
}

func TestCheckFile_GoodsScoreMinusOneRejectedNonGoods(t *testing.T) {
	// books scope still rejects -1.
	issues := checkYAMLContent(t, "books.yml", "books", "- name: test\n  score: -1\n")
	assertHasError(t, issues, true)
}

func TestCheckFile_GoodsFullValidStructure(t *testing.T) {
	issues := checkYAMLContent(t, "goods.EDC.yml", "goods", `- type: EDC
  tag: goods
  topics:
    - topic: 眼镜
      score: 5
      table:
        - name: 钛框眼镜
          brand: JINS
          price: ¥100
          date: 2019-07-01
          isUsing: true
      record:
        - date: 2024-12-06
          des: 换新
      qs:
        - 怎么选？
`)
	assertHasError(t, issues, false)
}

func TestCheckFile_GoodsTableStringItem(t *testing.T) {
	issues := checkYAMLContent(t, "goods.EDC.yml", "goods", `- type: EDC
  tag: goods
  topics:
    - topic: x
      score: 5
      table:
        - 1、这是字符串项
        - name: a
          price: ¥100
`)
	assertHasError(t, issues, true)
	require.True(t, containsIssue(issues, "table 项必须是对象"), "issues: %#v", issues)
}

func TestCheckFile_GoodsQsObjectItem(t *testing.T) {
	issues := checkYAMLContent(t, "goods.EDC.yml", "goods", `- type: EDC
  tag: goods
  topics:
    - topic: x
      score: 5
      qs:
        - name: 对象混入
`)
	assertHasError(t, issues, true)
	require.True(t, containsIssue(issues, "qs 项必须是字符串"), "issues: %#v", issues)
}

func TestCheckFile_GoodsQsValidStrings(t *testing.T) {
	issues := checkYAMLContent(t, "goods.EDC.yml", "goods", `- type: EDC
  tag: goods
  topics:
    - topic: x
      score: 5
      qs:
        - 怎么选？
        - 要注意什么？
`)
	assertHasError(t, issues, false)
}

func TestCheckFile_BooksQsObjectAllowed(t *testing.T) {
	// Non-goods scopes keep qs free-form.
	issues := checkYAMLContent(t, "books.yml", "books", "- name: test\n  qs:\n    - name: 对象也行\n")
	assertHasError(t, issues, false)
}

func TestCheckFile_GoodsHtoObjectItem(t *testing.T) {
	issues := checkYAMLContent(t, "goods.EDC.yml", "goods", `- type: EDC
  tag: goods
  topics:
    - topic: x
      score: 5
      hto:
        - name: 对象混入
`)
	assertHasError(t, issues, true)
	require.True(t, containsIssue(issues, "hto 项必须是字符串"), "issues: %#v", issues)
}

func TestCheckFile_GoodsWhyValidStrings(t *testing.T) {
	issues := checkYAMLContent(t, "goods.EDC.yml", "goods", `- type: EDC
  tag: goods
  topics:
    - topic: x
      score: 5
      why:
        - 原因1
        - 原因2
`)
	assertHasError(t, issues, false)
}

func TestCheckFile_GoodsNestedTopicsDateInvalid(t *testing.T) {
	issues := checkYAMLContent(t, "goods.EDC.yml", "goods", `- type: EDC
  tag: goods
  topics:
    - topic: 外层
      score: 5
      topics:
        - topic: 内层
          score: 5
          table:
            - name: a
              date: 2020-2-1
`)
	assertHasError(t, issues, true)
	require.Equal(t, 1, countIssues(issues, "date 必须是"), "should report once, not duplicate: %#v", issues)
}

func TestCheckFile_GoodsNestedTopicsValid(t *testing.T) {
	issues := checkYAMLContent(t, "goods.EDC.yml", "goods", `- type: EDC
  tag: goods
  topics:
    - topic: 外层
      score: 5
      topics:
        - topic: 内层
          score: 5
          table:
            - name: a
              date: 2020-02-01
`)
	assertHasError(t, issues, false)
}

func TestCheckFile_GoodsEndDateInvalidFormat(t *testing.T) {
	issues := checkYAMLContent(t, "goods.EDC.yml", "goods", `- type: EDC
  tag: goods
  topics:
    - topic: x
      score: 5
      table:
        - name: a
          price: ¥100
          endDate: 2025-1-1
          endPrice: ¥50
`)
	assertHasError(t, issues, true)
	require.True(t, containsIssue(issues, "endDate 必须是 YYYY-MM-DD"), "issues: %#v", issues)
}

func TestCheckFile_GoodsEndDateValid(t *testing.T) {
	issues := checkYAMLContent(t, "goods.EDC.yml", "goods", `- type: EDC
  tag: goods
  topics:
    - topic: x
      score: 5
      table:
        - name: a
          price: ¥100
          endDate: 2025-01-01
          endPrice: ¥50
`)
	assertHasError(t, issues, false)
}

func TestCheckFile_GoodsNestedTopicsTooDeep(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("- type: EDC\n  tag: goods\n  topics:\n")
	indent := 4
	for i := 0; i < maxNestedTopics+50; i++ {
		sb.WriteString(strings.Repeat(" ", indent) + "- topic: x\n")
		indent += 2
		sb.WriteString(strings.Repeat(" ", indent) + "topics:\n")
		indent += 2
	}
	sb.WriteString(strings.Repeat(" ", indent) + "- topic: bottom\n")

	issues := checkYAMLContent(t, "goods.EDC.yml", "goods", sb.String())
	require.True(t, containsIssue(issues, "嵌套过深"), "issues: %#v", issues)
}

func TestCheckFile_GoodsQsIntegerItem(t *testing.T) {
	issues := checkYAMLContent(t, "goods.EDC.yml", "goods", `- type: EDC
  tag: goods
  topics:
    - topic: x
      score: 5
      qs:
        - 12345
        - 正常问题
`)
	assertHasError(t, issues, true)
	require.True(t, containsIssue(issues, "qs 项必须是字符串"), "issues: %#v", issues)
}

func TestRunStructuredDataCheck_IncludeHiddenAcrossDomains(t *testing.T) {
	tmpDir := t.TempDir()
	// Hidden file with a books-scope violation is checked when IncludeHidden is set.
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".books.yml"), []byte("- name: test\n  bogus: x\n"), 0644))

	result, err := RunStructuredDataCheckWithOptions(tmpDir, "books", RunStructuredCheckOptions{IncludeHidden: true})
	require.NoError(t, err)
	require.True(t, containsIssue(result.Issues, "未在规则中定义的字段: bogus"), "issues: %#v", result.Issues)

	// Without IncludeHidden the hidden file is skipped.
	result, err = RunStructuredDataCheck(tmpDir, "books")
	require.NoError(t, err)
	assert.False(t, containsIssue(result.Issues, "bogus"), "hidden file should be skipped by default")
}

func TestCheckFile_GoodsTableItemMissingName(t *testing.T) {
	issues := checkYAMLContent(t, "goods.EDC.yml", "goods", `- type: EDC
  tag: goods
  topics:
    - topic: x
      score: 5
      table:
        - price: ¥100
`)
	assertHasError(t, issues, true)
	require.True(t, containsIssue(issues, "缺少必填字段 name"), "issues: %#v", issues)
}
