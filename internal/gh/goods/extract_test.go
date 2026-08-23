package goods

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	modelgoods "github.com/xbpk3t/docs-alfred/internal/gh/model/goods"
	"github.com/xbpk3t/docs-alfred/internal/gh/schema"
	"github.com/xbpk3t/docs-alfred/pkg/schemacheck"
)

// writeGoodsFiles creates a temp dir and writes goods-format YAML files.
func writeGoodsFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		p := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	}

	return dir
}

// durableGoodsYAML / clothingGoodsYAML are new-type per-file layouts: each file
// is one flat topic array and the type is derived from the file name.
const durableGoodsYAML = `- topic: 收纳袋
  table:
    - name: 抽绳束口#防水#收纳袋（15D尼龙涂硅）
      brand: 三峰出
      param: S码（12x28/15g）
      price: "¥13"
      isUsing: true
    - name: 天纵被子收纳袋
      brand: 天纵
      price: ¥84
      date: 2019-08-21
- topic: 速干浴巾
  table:
    - name: 速干浴巾 NH19Y001-J
      brand: 挪客
      price: ¥49
      isUsing: true
    - name: 雅棉全棉面巾
      brand: 雅棉
      price: ¥58.5
`

const clothingGoodsYAML = `- topic: long-johns  # 秋裤
  table:
    - name: 250 base功能内衣
      brand: 美利奴
      price: ¥800
    - name: HEATTECH系列秋裤
      brand: 优衣库
      price: ¥129
      isUsing: true
      des: 现在在穿
`

func TestExtractUsing_GroupsByTypeTopic(t *testing.T) {
	dir := writeGoodsFiles(t, map[string]string{
		"goods.耐用品.yml": durableGoodsYAML,
		"goods.衣物.yml":  clothingGoodsYAML,
	})

	out, err := ExtractUsing(dir)
	require.NoError(t, err)
	require.Len(t, out, 2) // 两个 type（由文件名派生）

	// 耐用品 type
	first := out[0]
	require.Equal(t, "耐用品", first.Type)
	require.Len(t, first.Topics, 2)
	assert.Equal(t, "收纳袋", first.Topics[0].Topic)
	require.Len(t, first.Topics[0].Items, 1)
	assert.Equal(t, "抽绳束口#防水#收纳袋（15D尼龙涂硅）", first.Topics[0].Items[0].Name)
	assert.Equal(t, "三峰出", strDeref(first.Topics[0].Items[0].Brand))
	assert.Equal(t, "¥13", strDeref(first.Topics[0].Items[0].Price))
	// 未标记 isUsing 的 item 不出现
	assert.Len(t, first.Topics[1].Items, 1)
	assert.Equal(t, "速干浴巾 NH19Y001-J", first.Topics[1].Items[0].Name)

	// 衣物 type
	second := out[1]
	require.Equal(t, "衣物", second.Type)
	require.Len(t, second.Topics, 1)
	assert.Equal(t, "long-johns", second.Topics[0].Topic)
	require.Len(t, second.Topics[0].Items, 1)
	assert.Equal(t, "HEATTECH系列秋裤", second.Topics[0].Items[0].Name)
	assert.Equal(t, "现在在穿", strDeref(second.Topics[0].Items[0].Des))
}

func TestExtractUsing_NoUsingItems(t *testing.T) {
	dir := writeGoodsFiles(t, map[string]string{"goods.耐用品.yml": `- topic: 收纳袋
  table:
    - name: 天纵被子收纳袋
      price: ¥84
`})

	out, err := ExtractUsing(dir)
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Len(t, out[0].Topics, 0)
}

func TestExtractUsing_EmptyDir(t *testing.T) {
	out, err := ExtractUsing(t.TempDir())
	require.NoError(t, err)
	require.Empty(t, out)
}

// TestExtractUsing_OutputValidatesAgainstSchema locks the using output to the
// using.schema.json contract: the marshaled JSON must validate, including empty
// types emitting `topics: []` rather than null.
func TestExtractUsing_OutputValidatesAgainstSchema(t *testing.T) {
	dir := writeGoodsFiles(t, map[string]string{
		"goods.空.yml": `- topic: 无在用
  table:
    - name: 未使用
`,
		"goods.在用.yml": `- topic: 收纳袋
  table:
    - name: 在用袋
      isUsing: true
      price: ¥13
`,
	})

	out, err := ExtractUsing(dir)
	require.NoError(t, err)

	raw, err := json.Marshal(out)
	require.NoError(t, err)
	var doc any
	require.NoError(t, json.Unmarshal(raw, &doc))

	sch, err := schemacheck.CompileBytes(schema.Using)
	require.NoError(t, err)
	require.Empty(t, schemacheck.Validate(sch, doc), "using output must validate against using.schema.json")
}

func TestExtractUsing_NonExistentDir(t *testing.T) {
	_, err := ExtractUsing(filepath.Join(t.TempDir(), "missing"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list goods files")
}

func TestExtractUsing_MultipleFilesMergeTypes(t *testing.T) {
	dir := writeGoodsFiles(t, map[string]string{
		"goods.aa.yml": durableGoodsYAML,
		"goods.bb.yml": `- topic: 零食
  table:
    - name: 牛肉干
      isUsing: true
`,
		"goods.cc.yml": clothingGoodsYAML,
	})

	out, err := ExtractUsing(dir)
	require.NoError(t, err)
	require.Len(t, out, 3) // aa/bb/cc 三个 type 合并
	types := map[string]bool{}
	for _, tp := range out {
		types[tp.Type] = true
	}
	assert.Len(t, types, 3)
	for _, k := range []string{"aa", "bb", "cc"} {
		assert.Contains(t, types, k)
	}
}

func TestExtractUsing_IgnoreNonGoodsFiles(t *testing.T) {
	dir := writeGoodsFiles(t, map[string]string{
		"goods.test.yml":      durableGoodsYAML,
		"other.yml":           "- topic: unrelated\n  table:\n    - name: x\n",
		"sub/goods.inner.yml": durableGoodsYAML,
	})

	out, err := ExtractUsing(dir)
	require.NoError(t, err)
	// 只读 goods.*.yml 且只读直接子目录；other.yml 不是 goods 命名（被忽略），
	// sub/ 下文件忽略，故仅 goods.test.yml 一个 type。
	require.Len(t, out, 1)
	assert.Equal(t, "test", out[0].Type)
}

func TestUsingItemsFromTable(t *testing.T) {
	items := usingItemsFromTable([]modelgoods.TableItem{
		{Name: "A", IsUsing: boolPtr(true)},
		{Name: "B", IsUsing: boolPtr(false)},
		{Name: "C"},              // 无 isUsing
		{IsUsing: boolPtr(true)}, // 无 name，跳过
		{Name: "D", IsUsing: boolPtr(true)},
	})
	require.Len(t, items, 2)
	assert.Equal(t, "A", items[0].Name)
	assert.Equal(t, "D", items[1].Name)
}

func TestUsingItemsFromTable_CopiesFields(t *testing.T) {
	// goods.schema.json 已把 name/price 收紧为 string，映射是纯字段拷贝；
	// 数字形态的 name（手机号）必须引号化成字符串，不再是 interface{}。
	items := usingItemsFromTable([]modelgoods.TableItem{
		{Name: "18616287252", IsUsing: boolPtr(true), Brand: strPtr("B"), Price: strPtr("¥8/月")},
	})
	require.Len(t, items, 1)
	assert.Equal(t, "18616287252", items[0].Name)
	assert.Equal(t, "B", strDeref(items[0].Brand))
	assert.Equal(t, "¥8/月", strDeref(items[0].Price))
}

func strDeref(p *string) string {
	if p == nil {
		return ""
	}

	return *p
}

func boolPtr(b bool) *bool {
	return &b
}

func strPtr(s string) *string {
	return &s
}

func TestExtractUsing_NumericName(t *testing.T) {
	// 数字形态 name（话费号码）由 goods.schema.json 强制为 string：未引号的纯
	// 数字会被 check 拒绝，数据必须写成 "18616287252"，输出保持字符串。
	dir := writeGoodsFiles(t, map[string]string{"goods.虚拟.yml": `- topic: 话费
  table:
    - name: "18616287252"
      brand: 话费
      price: ¥8/月
      isUsing: true
`})

	out, err := ExtractUsing(dir)
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Len(t, out[0].Topics, 1)
	require.Len(t, out[0].Topics[0].Items, 1)
	assert.Equal(t, "18616287252", out[0].Topics[0].Items[0].Name)
}

// TestExtractUsing_RealWorldShapes covers real goods.*.yml patterns:
// block-scalar des, inline comments, mixed indent, multi-topic/type files.
func TestExtractUsing_RealWorldShapes(t *testing.T) {
	dir := writeGoodsFiles(t, map[string]string{
		"goods.耐用品.yml": `- topic: 速干浴巾         # quick-dry-towel
  table:
    - name: 速干浴巾 NH19Y001-J
      brand: 挪客
      price: ¥49
      date: 2022-06-09
      isUsing: true
      des: |
        有啥用？
        - 在家直接用来装衣服，或者脏衣袋用。
        - 用来当枕头用。

    - name: 雅棉全棉面巾
      brand: 雅棉
      price: ¥58.5

- topic: 收纳袋
  table:
    - name: 抽绳束口#防水#收纳袋（15D尼龙涂硅）
      brand: 三峰出
      param: S码（21*28/15g）
      price: "¥13"
      isUsing: true
`,
		"goods.虚拟物品.yml": `- topic: membership  # 会员
  table:
    - name: 老乡鸡会员
      price: ¥8/月
    - name: 88VIP
      price: ¥88/年
      isUsing: true
`,
	})

	out, err := ExtractUsing(dir)
	require.NoError(t, err)
	require.Len(t, out, 2)

	// 文件名 unicode 排序：耐(U+8010) < 虚(U+865A)，故 out[0]=耐用品
	durs := out[0]
	require.Len(t, durs.Topics, 2)
	towel := durs.Topics[0]
	require.Len(t, towel.Items, 1)
	assert.Equal(t, "速干浴巾 NH19Y001-J", towel.Items[0].Name)
	// 块标量 des 完整保留(含多行)
	assert.Contains(t, strDeref(towel.Items[0].Des), "在家直接用来装衣服")
	assert.Contains(t, strDeref(towel.Items[0].Des), "用来当枕头用")

	bags := durs.Topics[1]
	require.Len(t, bags.Items, 1)
	assert.Equal(t, "抽绳束口#防水#收纳袋（15D尼龙涂硅）", bags.Items[0].Name)

	// 第二个 type 的 isUsing 也提取
	virtual := out[1]
	require.Len(t, virtual.Topics, 1)
	require.Len(t, virtual.Topics[0].Items, 1)
	assert.Equal(t, "88VIP", virtual.Topics[0].Items[0].Name)
}
