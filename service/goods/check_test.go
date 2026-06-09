package goods

import (
	"os"
	"path/filepath"
	"testing"

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
