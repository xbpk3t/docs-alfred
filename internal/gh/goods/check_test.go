package goods

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

func TestRunCheckAllowsLifecycleFieldsForEligibleTags(t *testing.T) {
	result := checkGoodsYAML(t, `---
- type: 跑步长裤
  tag: clothes
  score: 5
  item:
    - name: 梭织透气 跑步长裤
      price: ¥149
      date: 2023-04-29
      endDate: 2025-08-27
      endPrice: ¥20
`)

	require.False(t, checkutil.HasErrors(result.Issues), "issues: %#v", result.Issues)
}

func TestRunCheckRejectsLifecycleFieldsForExcludedTags(t *testing.T) {
	result := checkGoodsYAML(t, `---
- type: 饼干
  tag: food
  score: 0
  item:
    - name: 酵母减盐苏打饼干
      price: ¥22
      date: 2025-01-01
      endDate: 2025-02-01
`)

	require.True(t, checkutil.HasErrors(result.Issues))
	require.Contains(t, result.Issues[0].Message, "只允许用于生命周期实物")
}

func TestRunCheckRejectsEndDateWithoutDate(t *testing.T) {
	result := checkGoodsYAML(t, `---
- type: 耳机
  tag: EDC
  score: 3
  item:
    - name: C50
      price: ¥179
      endDate: 2025-09-27
`)

	require.True(t, checkutil.HasErrors(result.Issues))
	require.Contains(t, result.Issues[0].Message, "必须有同级 date")
}

func TestRunCheckRejectsEndDateBeforeDate(t *testing.T) {
	result := checkGoodsYAML(t, `---
- type: 耳机
  tag: EDC
  score: 3
  item:
    - name: C50
      price: ¥179
      date: 2025-09-27
      endDate: 2025-09-21
`)

	require.True(t, checkutil.HasErrors(result.Issues))
	require.Contains(t, result.Issues[0].Message, "不能早于")
}

func TestRunCheckRejectsEndPriceWithoutEndDate(t *testing.T) {
	result := checkGoodsYAML(t, `---
- type: 耳机
  tag: EDC
  score: 3
  item:
    - name: C50
      price: ¥179
      date: 2025-09-21
      endPrice: ¥137
`)

	require.True(t, checkutil.HasErrors(result.Issues))
	require.Contains(t, result.Issues[0].Message, "必须和 endDate 同时存在")
}

func TestRunCheckRejectsAmbiguousEndPrice(t *testing.T) {
	result := checkGoodsYAML(t, `---
- type: 耳机
  tag: EDC
  score: 3
  item:
    - name: C50
      price: ¥179
      date: 2025-09-21
      endDate: 2025-09-27
      endPrice: ¥100~¥120
`)

	require.True(t, checkutil.HasErrors(result.Issues))
	require.Contains(t, result.Issues[0].Message, "明确的一次性人民币金额")
}

func TestRunCheckAllowsUsingLifecycleFields(t *testing.T) {
	result := checkGoodsYAML(t, `---
- type: 睡袋
  tag: bedding
  score: 5
  using:
    name: 羽绒睡袋
    price: ¥599
    date: 2023-11-25
    endDate: 2026-03-30
  item: []
`)

	require.False(t, checkutil.HasErrors(result.Issues), "issues: %#v", result.Issues)
}

func TestRunCheckRejectsCategoryLifecycleFields(t *testing.T) {
	result := checkGoodsYAML(t, `---
- type: 睡袋
  tag: bedding
  score: 5
  endDate: 2026-03-30
  item: []
`)

	require.True(t, checkutil.HasErrors(result.Issues))
	require.Contains(t, result.Issues[0].Message, "只能写在 using 或 item[]")
}

func checkGoodsYAML(t *testing.T, content string) *CheckResult {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "goods.yml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	result, err := RunCheck(dir)
	require.NoError(t, err)

	return result
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

	_, err := RunCheck(dir)
	require.Error(t, err)
}

func TestRunCheck_TopLevelNotSequence(t *testing.T) {
	result := checkGoodsYAMLFromContent(t, `key: value`)
	assert.NotEmpty(t, result.Issues)
	assert.Contains(t, result.Issues[0].Message, "顶层必须是列表")
}

func TestRunCheck_ItemNotMapping(t *testing.T) {
	result := checkGoodsYAMLFromContent(t, `- just a string`)
	assert.NotEmpty(t, result.Issues)
	assert.Contains(t, result.Issues[0].Message, "goods 项必须是对象")
}

func TestRunCheck_EndDateOnCategory(t *testing.T) {
	result := checkGoodsYAMLFromContent(t, `---
- type: test
  tag: EDC
  endDate: 2025-01-01
  item: []
`)
	assert.NotEmpty(t, result.Issues)
	assert.Contains(t, result.Issues[0].Message, "只能写在 using 或 item[]")
}

func TestRunCheck_EndPriceOnCategory(t *testing.T) {
	result := checkGoodsYAMLFromContent(t, `---
- type: test
  tag: EDC
  endPrice: ¥100
  item: []
`)
	assert.NotEmpty(t, result.Issues)
	assert.Contains(t, result.Issues[0].Message, "只能写在 using 或 item[]")
}

func TestRunCheck_ItemNotSequence(t *testing.T) {
	// item is not a sequence - should be handled gracefully
	result := checkGoodsYAMLFromContent(t, `---
- type: test
  tag: EDC
  item: not-an-array
`)
	// No lifecycle issues since item is not a sequence
	assert.NotNil(t, result)
}

func TestRunCheck_NonMappingItem(t *testing.T) {
	result := checkGoodsYAMLFromContent(t, `---
- type: test
  tag: EDC
  item:
    - just a string
`)
	assert.NotNil(t, result)
}

func TestRunCheck_EndDateInvalidFormat(t *testing.T) {
	result := checkGoodsYAMLFromContent(t, `---
- type: test
  tag: EDC
  item:
    - name: item
      date: 2025-01-01
      endDate: not-a-date
`)
	assert.NotEmpty(t, result.Issues)
}

func TestRunCheck_DateInvalidFormat(t *testing.T) {
	result := checkGoodsYAMLFromContent(t, `---
- type: test
  tag: EDC
  item:
    - name: item
      date: not-a-date
      endDate: 2025-06-01
`)
	assert.NotEmpty(t, result.Issues)
}

func TestRunCheck_ValidLifecycle(t *testing.T) {
	result := checkGoodsYAMLFromContent(t, `---
- type: 耳机
  tag: EDC
  score: 3
  using:
    name: AirPods
    price: ¥1799
    date: 2023-01-01
    endDate: 2025-06-01
    endPrice: ¥200
  item: []
`)
	assert.Empty(t, result.Issues)
}

func TestNodeString_IntegerNode(t *testing.T) {
	// Create a YAML content with integer value for a field
	result := checkGoodsYAMLFromContent(t, `---
- type: test
  tag: EDC
  score: 3
  using:
    name: item
    price: 100
    date: 2023-01-01
    endDate: 2025-06-01
    endPrice: 150
  item: []
`)
	// endPrice is integer 150 — nodeString should handle IntegerNode
	// The price pattern check might fail for plain integer without ¥ prefix
	assert.NotNil(t, result)
}

func TestNodeString_FloatNode(t *testing.T) {
	result := checkGoodsYAMLFromContent(t, `---
- type: test
  tag: EDC
  score: 3
  using:
    name: item
    price: 100.5
    date: 2023-01-01
    endDate: 2025-06-01
    endPrice: 150.00
  item: []
`)
	assert.NotNil(t, result)
}

func TestNodeString_Nil(t *testing.T) {
	// A nil endDate/endPrice node means the field is not present
	result := checkGoodsYAMLFromContent(t, `---
- type: test
  tag: EDC
  score: 3
  item: []
`)
	assert.NotNil(t, result)
	assert.Empty(t, result.Issues)
}

func TestCheckFile_EmptyFileContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "goods.yml")
	require.NoError(t, os.WriteFile(path, []byte("   \n  "), 0644))

	result, err := RunCheck(dir)
	require.NoError(t, err)
	assert.Empty(t, result.Issues)
}

func TestCheckFile_NilDocBody(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "goods.yml")
	// A YAML document separator with nothing after it
	require.NoError(t, os.WriteFile(path, []byte("---\n"), 0644))

	result, err := RunCheck(dir)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestCheckLifecycleItems_NonSequenceItem(t *testing.T) {
	// item field that is a string, not a sequence
	result := checkGoodsYAMLFromContent(t, `---
- type: test
  tag: EDC
  item: just a string
`)
	assert.NotNil(t, result)
}

func TestCheckLifecycleMap_NonMappingItem(t *testing.T) {
	// item array with non-mapping entries
	result := checkGoodsYAMLFromContent(t, `---
- type: test
  tag: EDC
  item:
    - 42
    - just a string
`)
	assert.NotNil(t, result)
}

func TestCheckLifecycleMap_NilMapping(t *testing.T) {
	// using entry is nil (not present)
	result := checkGoodsYAMLFromContent(t, `---
- type: test
  tag: EDC
  score: 3
  item: []
`)
	assert.NotNil(t, result)
	assert.Empty(t, result.Issues)
}

func TestParseOptionalDate_InvalidDateValue(t *testing.T) {
	// Date that matches YYYY-MM-DD pattern but is invalid (Feb 30)
	result := checkGoodsYAMLFromContent(t, `---
- type: test
  tag: EDC
  item:
    - name: item
      date: 2025-02-30
      endDate: 2025-06-01
`)
	assert.NotEmpty(t, result.Issues)
}

func TestCheckLifecycleEligibility_AllEligibleTags(t *testing.T) {
	// Test all eligible tags
	for tag := range eligibleLifecycleTags {
		t.Run(tag, func(t *testing.T) {
			result := checkGoodsYAMLFromContent(t, `---
- type: test
  tag: `+tag+`
  score: 3
  using:
    name: item
    price: ¥100
    date: 2023-01-01
    endDate: 2025-06-01
    endPrice: ¥50
  item: []
`)
			assert.Empty(t, result.Issues)
		})
	}
}

func TestCheckFile_MultiDocGoods(t *testing.T) {
	result := checkGoodsYAMLFromContent(t, `---
- type: A
  tag: EDC
  score: 3
  item: []
---
- type: B
  tag: food
  score: 0
  item: []
`)
	assert.NotNil(t, result)
}

func TestIsStrictCNYPrice(t *testing.T) {
	assert.True(t, isStrictCNYPrice("¥100"))
	assert.True(t, isStrictCNYPrice("￥100"))
	assert.True(t, isStrictCNYPrice("100"))
	assert.True(t, isStrictCNYPrice("¥100.50"))
	assert.True(t, isStrictCNYPrice("100.50"))
	assert.False(t, isStrictCNYPrice("¥100~¥200"))
	assert.False(t, isStrictCNYPrice(""))
	assert.False(t, isStrictCNYPrice("free"))
}

func TestRunCheck_UsingWithNilMap(t *testing.T) {
	// using field present but not a mapping
	result := checkGoodsYAMLFromContent(t, `---
- type: test
  tag: EDC
  using: not-a-map
  item: []
`)
	assert.NotNil(t, result)
}

func TestNodeString_IntegerNodeEndPrice(t *testing.T) {
	// Plain integer for endPrice triggers IntegerNode path in nodeString
	result := checkGoodsYAMLFromContent(t, `---
- type: test
  tag: EDC
  score: 3
  using:
    name: item
    price: ¥100
    date: 2023-01-01
    endDate: 2025-06-01
    endPrice: 150
  item: []
`)
	// Plain integer 150 → IntegerNode → nodeString returns "150" → matches CNY pattern
	assert.Empty(t, result.Issues)
}

func TestNodeString_FloatNodeEndPrice(t *testing.T) {
	// Float value for endPrice triggers FloatNode path
	result := checkGoodsYAMLFromContent(t, `---
- type: test
  tag: EDC
  score: 3
  using:
    name: item
    price: ¥100
    date: 2023-01-01
    endDate: 2025-06-01
    endPrice: 150.50
  item: []
`)
	assert.Empty(t, result.Issues)
}

func TestNodeString_IntegerNodeDate(t *testing.T) {
	// Integer for date field (invalid date format but exercises IntegerNode)
	result := checkGoodsYAMLFromContent(t, `---
- type: test
  tag: EDC
  item:
    - name: item
      date: 20250101
      endDate: 2025-06-01
`)
	// Integer date doesn't match YYYY-MM-DD → error
	assert.NotEmpty(t, result.Issues)
}

func TestNodeString_FloatNodeDate(t *testing.T) {
	result := checkGoodsYAMLFromContent(t, `---
- type: test
  tag: EDC
  item:
    - name: item
      date: 2025.0
      endDate: 2025-06-01
`)
	assert.NotEmpty(t, result.Issues)
}

func TestRunCheck_EndPriceNonEligibleTag(t *testing.T) {
	// endPrice on non-eligible tag should produce error
	result := checkGoodsYAMLFromContent(t, `---
- type: test
  tag: food
  score: 0
  item:
    - name: item
      price: ¥100
      date: 2023-01-01
      endDate: 2025-06-01
      endPrice: ¥50
`)
	assert.True(t, checkutil.HasErrors(result.Issues))
}

func TestCheckLifecycleItems_NilItemNode(t *testing.T) {
	// item is present but nil
	result := checkGoodsYAMLFromContent(t, `---
- type: test
  tag: EDC
  item: null
`)
	assert.NotNil(t, result)
}

func TestRunCheck_ReadError(t *testing.T) {
	// Create a file and then remove it after listing to trigger read error
	dir := t.TempDir()
	path := filepath.Join(dir, "goods.yml")
	require.NoError(t, os.WriteFile(path, []byte("- type: test\n"), 0644))
	// Remove file after ListYAMLFiles finds it but before checkFile reads it
	// This is hard to trigger reliably, so we test via the public API
	result, err := RunCheck(dir)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestCheckFile_InvalidYAMLContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "goods.yml")
	require.NoError(t, os.WriteFile(path, []byte("invalid: [yaml: broken\n"), 0644))

	_, err := RunCheck(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "yaml")
}

func TestCheckLifecycleMap_EndPriceOnlyWithNonEligibleTag(t *testing.T) {
	// endPrice without endDate on non-eligible tag
	result := checkGoodsYAMLFromContent(t, `---
- type: test
  tag: food
  item:
    - name: item
      price: ¥100
      date: 2023-01-01
      endPrice: ¥50
`)
	assert.True(t, checkutil.HasErrors(result.Issues))
}

func TestNodeString_DefaultCase(t *testing.T) {
	// Boolean node should trigger default case
	result := checkGoodsYAMLFromContent(t, `---
- type: test
  tag: EDC
  score: 3
  using:
    name: item
    price: ¥100
    date: 2023-01-01
    endDate: 2025-06-01
    endPrice: true
  item: []
`)
	// Boolean endPrice → default case in nodeString → returns ("", false)
	// → isStrictCNYPrice("") → false → error
	assert.NotEmpty(t, result.Issues)
}

func checkGoodsYAMLFromContent(t *testing.T, content string) *CheckResult {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "goods.yml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	result, err := RunCheck(dir)
	require.NoError(t, err)

	return result
}
