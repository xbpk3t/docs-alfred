package books

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

// validBooksYAML follows the real books.*.yml section shape
// (type → topics → table), shared with data/ntl.
const validBooksYAML = `---
- type: 编程
  topics:
    - topic: Go
      score: 5
      table:
        - name: 《Go 程序设计语言》
          author: Donovan
          publishAt: 2015
          readAt: 2023-04-01
          url: https://gopl.io
      record:
        - date: 2023-04-01
          des: 读完。
      qs:
        - 有什么收获？
`

func checkBooksYAML(t *testing.T, content string) *CheckResult {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "books.test.yml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	result, err := RunCheckWithOptions(dir, CheckOptions{})
	require.NoError(t, err)

	return result
}

func msgsOf(result *CheckResult) []string {
	msgs := make([]string, 0, len(result.Issues))
	for _, i := range result.Issues {
		msgs = append(msgs, i.Message)
	}

	return msgs
}

// joinedMsgs joins all issue messages with a space so assertions can use
// substring matching (assert.Contains on []string is exact-element).
func joinedMsgs(result *CheckResult) string {
	return strings.Join(msgsOf(result), " ")
}

func TestRunCheck_ValidBooksStructure(t *testing.T) {
	result := checkBooksYAML(t, validBooksYAML)
	assert.Empty(t, result.Issues)
}

func TestRunCheck_MissingType(t *testing.T) {
	result := checkBooksYAML(t, `---
- topics:
    - topic: x
      table:
        - name: 书
`)
	assert.True(t, checkutil.HasErrors(result.Issues))
	assert.Contains(t, joinedMsgs(result), "missing property 'type'")
}

func TestRunCheck_TopLevelNotSequence(t *testing.T) {
	result := checkBooksYAML(t, `key: value`)
	assert.NotEmpty(t, result.Issues)
	assert.Contains(t, joinedMsgs(result), "want array")
}

func TestRunCheck_UndefinedField(t *testing.T) {
	result := checkBooksYAML(t, `---
- type: 编程
  bogus_field: x
  topics:
    - topic: x
`)
	assert.NotEmpty(t, result.Issues)
	assert.Contains(t, joinedMsgs(result), "additional properties 'bogus_field'")
}

func TestRunCheck_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	result, err := RunCheckWithOptions(dir, CheckOptions{})
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.Issues)
}

func TestRunCheck_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.yml"), []byte("invalid: [yaml:\n"), 0644))

	result, err := RunCheckWithOptions(dir, CheckOptions{})
	require.NoError(t, err)
	assert.True(t, checkutil.HasErrors(result.Issues))
	assert.Contains(t, joinedMsgs(result), "YAML parse error")
}

func TestRunCheck_IgnoresHiddenByDefault(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".hidden.yml"), []byte(`---
- topics: []
`), 0644))

	result, err := RunCheckWithOptions(dir, CheckOptions{})
	require.NoError(t, err)
	assert.Empty(t, result.Issues)
}

func TestRunCheck_IncludeHiddenChecksHidden(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".hidden.yml"), []byte(`---
- topics: []
`), 0644))

	result, err := RunCheckWithOptions(dir, CheckOptions{IncludeHidden: true})
	require.NoError(t, err)
	require.True(t, checkutil.HasErrors(result.Issues))
	assert.Contains(t, joinedMsgs(result), "missing property 'type'")
	assert.Contains(t, result.Issues[0].File, ".hidden.yml")
}

func TestRunCheck_TableRowMissingName(t *testing.T) {
	result := checkBooksYAML(t, `---
- type: 编程
  topics:
    - topic: 技术
      table:
        - author: 某作者
`)
	assert.NotEmpty(t, result.Issues)
	assert.Contains(t, joinedMsgs(result), "missing property 'name'")
}

func TestRunCheck_PublishAtIntValid(t *testing.T) {
	result := checkBooksYAML(t, `---
- type: 编程
  topics:
    - topic: 技术
      table:
        - name: 书
          publishAt: 2026
`)
	assert.Empty(t, result.Issues)
}

func TestRunCheck_PublishAtStringRejected(t *testing.T) {
	// publishAt 已迁移为 integer：带引号的字符串会被拒绝。
	result := checkBooksYAML(t, `---
- type: 编程
  topics:
    - topic: 技术
      table:
        - name: 书
          publishAt: "2026"
`)
	assert.NotEmpty(t, result.Issues)
	assert.Contains(t, joinedMsgs(result), "got string, want integer")
}

func TestRunCheck_PublishAtOutOfRange(t *testing.T) {
	// 999 低于 1000：schema 用 minimum 兜住旧正则 ^\d{4}$ 的语义。
	result := checkBooksYAML(t, `---
- type: 编程
  topics:
    - topic: 技术
      table:
        - name: 书
          publishAt: 999
`)
	assert.NotEmpty(t, result.Issues)
	assert.Contains(t, joinedMsgs(result), "minimum")
}

func TestRunCheck_ReadAtBadPattern(t *testing.T) {
	result := checkBooksYAML(t, `---
- type: 编程
  topics:
    - topic: 技术
      table:
        - name: 书
          readAt: 2024-13
`)
	assert.NotEmpty(t, result.Issues)
	assert.Contains(t, joinedMsgs(result), "does not match pattern")
}

func TestRunCheck_URLBadPattern(t *testing.T) {
	result := checkBooksYAML(t, `---
- type: 编程
  topics:
    - topic: 技术
      table:
        - name: 书
          url: not-a-url
`)
	assert.NotEmpty(t, result.Issues)
	assert.Contains(t, joinedMsgs(result), "does not match pattern")
}

func TestRunCheck_ScoreOutOfRange(t *testing.T) {
	result := checkBooksYAML(t, `---
- type: 编程
  topics:
    - topic: 技术
      table:
        - name: 书
          score: 6
`)
	assert.NotEmpty(t, result.Issues)
	assert.Contains(t, joinedMsgs(result), "maximum")
}
