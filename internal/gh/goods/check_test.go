package goods

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

// validGoodsYAML follows the real goods.*.yml structure (tag removed: the
// section tag key is no longer part of the goods data model).
const validGoodsYAML = `---
- type: 耐用品
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

func TestRunCheck_ValidGoodsStructure(t *testing.T) {
	result := checkGoodsYAML(t, validGoodsYAML)
	assert.Empty(t, result.Issues)
}

func TestRunCheck_MissingType(t *testing.T) {
	result := checkGoodsYAML(t, `---
- topics:
    - topic: x
      score: 5
      table:
        - name: item
`)
	assert.True(t, checkutil.HasErrors(result.Issues))
	assert.Contains(t, joinedMsgs(result), "missing property 'type'")
}

func TestRunCheck_TopLevelNotSequence(t *testing.T) {
	result := checkGoodsYAML(t, `key: value`)
	assert.NotEmpty(t, result.Issues)
	assert.Contains(t, joinedMsgs(result), "want array")
}

func TestRunCheck_UndefinedField(t *testing.T) {
	result := checkGoodsYAML(t, `---
- type: 耐用品
  bogus_field: x
  topics:
    - topic: x
      kind: tools
`)
	assert.NotEmpty(t, result.Issues)
	assert.Contains(t, joinedMsgs(result), "additional properties 'bogus_field'")
}

func TestRunCheck_TagIsRejected(t *testing.T) {
	// tag was removed from the goods data model; the shared schema must flag it.
	result := checkGoodsYAML(t, `---
- type: 耐用品
  tag: goods
  topics:
    - topic: x
`)
	assert.NotEmpty(t, result.Issues)
	assert.Contains(t, joinedMsgs(result), "additional properties 'tag'")
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

	result, err := RunCheck(dir)
	require.NoError(t, err)
	assert.True(t, checkutil.HasErrors(result.Issues))
	assert.Contains(t, joinedMsgs(result), "YAML parse error")
}

func TestRunCheck_IgnoresHiddenByDefault(t *testing.T) {
	dir := t.TempDir()
	// Hidden file that violates the goods structure (missing type).
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".goods.EDC.yml"), []byte(`---
- topics: []
`), 0644))

	result, err := RunCheck(dir)
	require.NoError(t, err)
	assert.Empty(t, result.Issues)
}

func TestRunCheck_IncludeHiddenChecksHidden(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".goods.EDC.yml"), []byte(`---
- topics: []
`), 0644))

	result, err := RunCheckWithOptions(dir, CheckOptions{IncludeHidden: true})
	require.NoError(t, err)
	require.True(t, checkutil.HasErrors(result.Issues))
	assert.Contains(t, joinedMsgs(result), "missing property 'type'")
	assert.Contains(t, result.Issues[0].File, ".goods.EDC.yml")
}

func TestRunCheck_IncludeHiddenValidHidden(t *testing.T) {
	// A hidden file that follows the goods.*.yml structure passes.
	result := checkGoodsYAMLHidden(t, `---
- type: 耐用品
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
	assert.Contains(t, joinedMsgs(result), "YAML parse error")
}

func TestRunCheck_IncludeHiddenLegacyStructure(t *testing.T) {
	// A legacy hidden file (using/item top-level, pre-migration) must be
	// flagged by --include-hidden: using/item are not defined goods fields.
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".goods.EDC.yml"), []byte(`---
- type: sling-bag
  score: 3
  using:
    name: Packable Tote
    price: ¥178
  item:
    - name: 257挎包
      price: ¥1359
`), 0644))

	result, err := RunCheckWithOptions(dir, CheckOptions{IncludeHidden: true})
	require.NoError(t, err)
	assert.NotEmpty(t, result.Issues, "legacy hidden file should be flagged: %#v", result.Issues)

	joined := joinedMsgs(result)
	assert.Contains(t, joined, "additional properties")
	assert.Contains(t, joined, "'using'", "legacy using field must be flagged")
	assert.Contains(t, joined, "'item'", "legacy item field must be flagged")
	assert.Contains(t, result.Issues[0].File, ".goods.EDC.yml")
}

func TestRunCheck_TableRowMissingName(t *testing.T) {
	result := checkGoodsYAML(t, `---
- type: 耐用品
  topics:
    - topic: 收纳
      table:
        - price: ¥13
`)
	assert.NotEmpty(t, result.Issues)
	assert.Contains(t, joinedMsgs(result), "missing property 'name'")
}

func TestRunCheck_EndPriceWithoutEndDate(t *testing.T) {
	result := checkGoodsYAML(t, `---
- type: 耐用品
  topics:
    - topic: 收纳
      table:
        - name: 物品
          endPrice: ¥50
`)
	assert.NotEmpty(t, result.Issues)
	assert.Contains(t, joinedMsgs(result), "required, if 'endPrice' exists")
}

func TestRunCheck_EndDateAloneAllowed(t *testing.T) {
	result := checkGoodsYAML(t, `---
- type: 耐用品
  topics:
    - topic: 收纳
      table:
        - name: 物品
          endDate: "2025-01-01"
`)
	assert.Empty(t, result.Issues)
}

func TestRunCheck_EndDateAtTopicLevelRejected(t *testing.T) {
	result := checkGoodsYAML(t, `---
- type: 耐用品
  topics:
    - topic: 收纳
      endDate: "2025-01-01"
      table:
        - name: 物品
`)
	assert.NotEmpty(t, result.Issues)
	assert.Contains(t, joinedMsgs(result), "additional properties 'endDate'")
}
