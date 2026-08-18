package md

import (
	"encoding/json"
	"strings"
	"testing"

	yaml "github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func rec(pairs ...string) yaml.MapSlice {
	var out yaml.MapSlice
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, yaml.MapItem{Key: pairs[i], Value: pairs[i+1]})
	}
	return out
}

func TestDataTableColumns_FirstSeenOrder(t *testing.T) {
	dt := NewDataTable([]yaml.MapSlice{
		rec("组", "IP 质量", "裁决", "keep ping0"),
		rec("组", "DNS", "裁决", "PD 栈", "缺口", "已落地"),
	})
	assert.Equal(t, []string{"组", "裁决", "缺口"}, dt.Columns())
}

func TestDataTableColumns_UnionAcrossRecords(t *testing.T) {
	dt := NewDataTable([]yaml.MapSlice{
		rec("a", "1"),
		rec("b", "2", "a", "3"),
	})
	// Union, first-seen order: a (row1) then b (row2).
	assert.Equal(t, []string{"a", "b"}, dt.Columns())
}

func TestDataTableTerminal(t *testing.T) {
	dt := NewDataTable([]yaml.MapSlice{
		rec("组", "IP 质量", "裁决", "keep ping0", "缺口", "书签未清"),
		rec("组", "DNS", "裁决", "PD 栈"),
	})
	got := dt.Terminal()
	// Terminal render: box-drawing borders (StyleLight), not markdown pipes.
	assert.Contains(t, got, "┌")
	assert.Contains(t, got, "┐")
	// Header cells are centered (StyleLight centers headers).
	assert.Contains(t, got, "组")
	assert.Contains(t, got, "裁决")
	assert.Contains(t, got, "缺口")
	assert.Contains(t, got, "IP 质量")
	// Missing key in row 2 renders an empty cell (PD 栈 freed).
	assert.Contains(t, got, "PD 栈")
	// Column separator inside the box.
	assert.Contains(t, got, "├")
	assert.Contains(t, got, "┤")
}

func TestDataTableMarkdownSource(t *testing.T) {
	dt := NewDataTable([]yaml.MapSlice{
		rec("组", "IP 质量", "裁决", "keep ping0", "缺口", "书签未清"),
	})
	got := dt.Markdown()
	// Markdown source: pipe tables, no box borders.
	assert.Contains(t, got, "| 组 |")
	assert.Contains(t, got, "| 裁决 |")
	assert.Contains(t, got, "| 缺口 |")
	assert.Contains(t, got, "IP 质量")
	assert.NotContains(t, got, "┌")
	assert.Contains(t, got, "| --- |") // separator row
}

func TestDataTableEmpty(t *testing.T) {
	assert.Empty(t, NewDataTable(nil).Terminal())
	assert.Empty(t, NewDataTable(nil).Markdown())
	assert.Empty(t, NewDataTable([]yaml.MapSlice{}).Terminal())
	assert.Empty(t, NewDataTable([]yaml.MapSlice{}).Markdown())
}

func TestDataTableJSON_OrderPreserved(t *testing.T) {
	rows := []yaml.MapSlice{
		rec("组", "IP 质量", "裁决", "keep ping0", "缺口", "书签未清"),
		rec("组", "DNS", "裁决", "PD 栈"),
	}
	b, err := NewDataTable(rows).JSON()
	require.NoError(t, err)

	// Decode to confirm it is a valid JSON array of objects;
	// key-ORDER is asserted on raw bytes below (encoding/json would sort
	// map keys, which is exactly what we must NOT do).
	var got []map[string]any
	require.NoError(t, json.Unmarshal(b, &got))
	require.Len(t, got, 2)
	bStr := string(b)
	assert.True(t, strings.Index(bStr, `"组"`) < strings.Index(bStr, `"裁决"`),
		"key 组 must appear before 裁决 in raw JSON: %s", bStr)
	assert.True(t, strings.Index(bStr, `"裁决"`) < strings.Index(bStr, `"缺口"`),
		"key 裁决 must appear before 缺口 in raw JSON: %s", bStr)
}

func TestDataTableJSON_ValuePreserved(t *testing.T) {
	b, err := NewDataTable([]yaml.MapSlice{rec("topic", "AI/LLM-res", "count", "18")}).JSON()
	require.NoError(t, err)
	assert.Equal(t, `[{"topic":"AI/LLM-res","count":"18"}]`, string(b))
}

func TestNamedDataTable(t *testing.T) {
	got := NamedDataTable("research top10", []yaml.MapSlice{
		rec("topic", "AI/LLM-res", "count", "18"),
	}, false)
	assert.Contains(t, got, "## research top10")
	assert.Contains(t, got, "┌")
	assert.Contains(t, got, "AI/LLM-res")
}

func TestNamedDataTableMD(t *testing.T) {
	got := NamedDataTable("research top10", []yaml.MapSlice{
		rec("topic", "AI/LLM-res", "count", "18"),
	}, true)
	assert.Contains(t, got, "## research top10")
	assert.Contains(t, got, "| topic |")
	assert.Contains(t, got, "AI/LLM-res")
}
