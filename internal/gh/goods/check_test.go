package goods

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

// validGoodsYAML follows the real goods.*.yml structure.
const validGoodsYAML = `---
- type: 耐用品
  tag: goods
  topics:
    - topic: 收纳袋
      score: 5
      table:
        - name: 抽绳束口#防水#收纳袋
          brand: 三峰出
          param: S码（12x28/15g）
          price: ¥13
          isUsing: true
          des: 衣物的分类收纳。
      record:
        - date: 2024-12-06
          des: 分类打包。
      qs:
        - 日常怎么收纳衣服？
`

func checkGoodsYAML(t *testing.T, content string) *CheckResult {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "goods.yml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	result, err := RunCheck(dir)
	require.NoError(t, err)

	return result
}

func checkGoodsYAMLHidden(t *testing.T, content string) *CheckResult {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, ".goods.EDC.yml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	result, err := RunCheckWithOptions(dir, CheckOptions{IncludeHidden: true})
	require.NoError(t, err)

	return result
}

func TestRunCheck_ValidGoodsStructure(t *testing.T) {
	result := checkGoodsYAML(t, validGoodsYAML)
	assert.Empty(t, result.Issues)
}

func TestRunCheck_MissingType(t *testing.T) {
	result := checkGoodsYAML(t, `---
- tag: goods
  topics:
    - topic: x
      score: 5
      table:
        - name: item
`)
	assert.True(t, checkutil.HasErrors(result.Issues))
	assert.Contains(t, result.Issues[0].Message, "缺少必填字段 type")
}

func TestRunCheck_TopLevelNotSequence(t *testing.T) {
	result := checkGoodsYAML(t, `key: value`)
	assert.NotEmpty(t, result.Issues)
	assert.Contains(t, result.Issues[0].Message, "顶层必须是列表")
}

func TestRunCheck_UndefinedField(t *testing.T) {
	result := checkGoodsYAML(t, `---
- type: 耐用品
  tag: goods
  bogus_field: x
  topics: []
`)
	assert.NotEmpty(t, result.Issues)
	assert.Contains(t, result.Issues[0].Message, "未在规则中定义的字段")
}

func TestRunCheck_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	result, err := RunCheck(dir)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.Issues)
}

func TestRunCheck_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.yml"), []byte("invalid: [yaml:\n"), 0644))

	// The shared structured engine reports parse errors as issues, not Go errors.
	result, err := RunCheck(dir)
	require.NoError(t, err)
	assert.True(t, checkutil.HasErrors(result.Issues))
	assert.Contains(t, result.Issues[0].Message, "YAML parse error")
}

func TestRunCheck_IgnoresHiddenByDefault(t *testing.T) {
	dir := t.TempDir()
	// Hidden file that violates the goods structure (missing type).
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".goods.EDC.yml"), []byte(`---
- tag: goods
  topics: []
`), 0644))

	result, err := RunCheck(dir)
	require.NoError(t, err)
	assert.Empty(t, result.Issues)
}

func TestRunCheck_IncludeHiddenChecksHidden(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".goods.EDC.yml"), []byte(`---
- tag: goods
  topics: []
`), 0644))

	result, err := RunCheckWithOptions(dir, CheckOptions{IncludeHidden: true})
	require.NoError(t, err)
	require.True(t, checkutil.HasErrors(result.Issues))
	assert.Contains(t, result.Issues[0].Message, "缺少必填字段 type")
	assert.Contains(t, result.Issues[0].File, ".goods.EDC.yml")
}

func TestRunCheck_IncludeHiddenValidHidden(t *testing.T) {
	// A hidden file that already follows the goods.*.yml structure passes.
	result := checkGoodsYAMLHidden(t, `---
- type: 耐用品
  tag: goods
  topics:
    - topic: 收纳袋
      score: 5
      table:
        - name: item
          price: ¥100
`)
	assert.Empty(t, result.Issues)
}

func TestRunCheck_IncludeHiddenYAMLError(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".goods.EDC.yml"), []byte("invalid: [yaml:\n"), 0644))

	result, err := RunCheckWithOptions(dir, CheckOptions{IncludeHidden: true})
	require.NoError(t, err)
	assert.True(t, checkutil.HasErrors(result.Issues))
	assert.Contains(t, result.Issues[0].Message, "YAML parse error")
}
