package goods

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

const sampleGoodsYAML = `---
- type: 耐用品
  tag: goods
  topics:
    - topic: 收纳袋
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

- type: 衣物
  tag: goods
  topics:
    - topic: long-johns  # 秋裤
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

func TestExtractUsing_GroupsByTagTypeTopic(t *testing.T) {
	dir := writeGoodsFiles(t, map[string]string{"goods.test.yml": sampleGoodsYAML})

	out, err := ExtractUsing(dir)
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Equal(t, "goods", out[0].Tag)
	require.Len(t, out[0].Types, 2)

	// 耐用品 type
	first := out[0].Types[0]
	require.Equal(t, "耐用品", first.Type)
	require.Len(t, first.Topics, 2)
	assert.Equal(t, "收纳袋", first.Topics[0].Topic)
	require.Len(t, first.Topics[0].Items, 1)
	assert.Equal(t, "抽绳束口#防水#收纳袋（15D尼龙涂硅）", first.Topics[0].Items[0].Name)
	assert.Equal(t, "三峰出", first.Topics[0].Items[0].Brand)
	assert.Equal(t, "¥13", first.Topics[0].Items[0].Price)
	// 未标记 isUsing 的 item 不出现
	assert.Len(t, first.Topics[1].Items, 1)
	assert.Equal(t, "速干浴巾 NH19Y001-J", first.Topics[1].Items[0].Name)

	// 衣物 type
	second := out[0].Types[1]
	require.Len(t, second.Topics, 1)
	assert.Equal(t, "long-johns", second.Topics[0].Topic)
	require.Len(t, second.Topics[0].Items, 1)
	assert.Equal(t, "HEATTECH系列秋裤", second.Topics[0].Items[0].Name)
	assert.Equal(t, "现在在穿", second.Topics[0].Items[0].Des)
}

func TestExtractUsing_NoUsingItems(t *testing.T) {
	dir := writeGoodsFiles(t, map[string]string{"goods.test.yml": `---
- type: 耐用品
  tag: goods
  topics:
    - topic: 收纳袋
      table:
        - name: 天纵被子收纳袋
          price: ¥84
`})

	out, err := ExtractUsing(dir)
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Len(t, out[0].Types, 1)
	// 无 isUsing item 的 topic 不产生条目
	require.Len(t, out[0].Types[0].Topics, 0)
}

func TestExtractUsing_EmptyDir(t *testing.T) {
	out, err := ExtractUsing(t.TempDir())
	require.NoError(t, err)
	require.Empty(t, out)
}

func TestExtractUsing_NonExistentDir(t *testing.T) {
	_, err := ExtractUsing(filepath.Join(t.TempDir(), "missing"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list goods files")
}

func TestExtractUsing_MultipleFilesMergeTags(t *testing.T) {
	dir := writeGoodsFiles(t, map[string]string{
		"goods.a.yml": sampleGoodsYAML,
		"goods.b.yml": `---
- type: 食品
  tag: goods
  topics:
    - topic: 零食
      table:
        - name: 牛肉干
          isUsing: true
`,
	})

	out, err := ExtractUsing(dir)
	require.NoError(t, err)
	require.Len(t, out, 1) // 同一个 tag: goods 合并
	require.Equal(t, "goods", out[0].Tag)
	// 同 tag 下 3 个 type 合并（耐用品/衣物/食品）
	types := map[string]bool{}
	for _, tp := range out[0].Types {
		types[tp.Type] = true
	}
	assert.Len(t, types, 3)
}

func TestExtractUsing_IgnoreNonGoodsFiles(t *testing.T) {
	dir := writeGoodsFiles(t, map[string]string{
		"goods.test.yml":      sampleGoodsYAML,
		"other.yml":           "- type: unrelated\n  tag: other\n",
		"sub/goods.inner.yml": sampleGoodsYAML,
	})

	out, err := ExtractUsing(dir)
	require.NoError(t, err)
	// 只读直接子目录的文件，sub/ 下文件忽略
	require.Len(t, out, 1)
	require.Len(t, out[0].Types, 2)
}

func TestUsingItemsFromTable(t *testing.T) {
	items := usingItemsFromTable([]map[string]interface{}{
		{"name": "A", "isUsing": true},
		{"name": "B", "isUsing": false},
		{"name": "C"},                    // 无 isUsing
		{"isUsing": true},                // 无 name，跳过
		{"name": "D", "isUsing": "true"}, // 字符串 true
	})
	require.Len(t, items, 2)
	assert.Equal(t, "A", items[0].Name)
	assert.Equal(t, "D", items[1].Name)
}

func TestUsingItemsFromTable_ExtraFields(t *testing.T) {
	items := usingItemsFromTable([]map[string]interface{}{
		{"name": "X", "isUsing": true, "brand": "B", "scope": "home"},
	})
	require.Len(t, items, 1)
	assert.Equal(t, "B", items[0].Brand)
	// scope 是非标准字段，进 Extra
	require.NotNil(t, items[0].Extra)
	assert.Equal(t, "home", items[0].Extra["scope"])
}

func TestExtractUsing_NumericName(t *testing.T) {
	// name 是纯数字（如话费号码），YAML 解析为 int，必须转成 string 输出
	dir := writeGoodsFiles(t, map[string]string{"goods.test.yml": `---
- type: 虚拟物品
  tag: goods
  topics:
    - topic: 话费
      table:
        - name: 18616287252
          brand: 话费
          price: ¥8/月
          isUsing: true
`})

	out, err := ExtractUsing(dir)
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Len(t, out[0].Types, 1)
	require.Len(t, out[0].Types[0].Topics, 1)
	require.Len(t, out[0].Types[0].Topics[0].Items, 1)
	assert.Equal(t, "18616287252", out[0].Types[0].Topics[0].Items[0].Name)
}

func TestBoolTrue(t *testing.T) {
	assert.True(t, boolTrue(true))
	assert.True(t, boolTrue("true"))
	assert.True(t, boolTrue(" TRUE "))
	assert.False(t, boolTrue(false))
	assert.False(t, boolTrue("false"))
	assert.False(t, boolTrue(1))
	assert.False(t, boolTrue(nil))
}
