package cmd

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// usingType mirrors goods.UsingType for CLI-level JSON assertions.
// The tag grouping layer was removed; output is type → topic directly.
type usingType struct {
	Type   string       `json:"type"`
	Topics []usingTopic `json:"topics"`
}

type usingTopic struct {
	Topic string      `json:"topic"`
	Items []usingItem `json:"items"`
}

type usingItem struct {
	Name  string `json:"name"`
	Brand string `json:"brand,omitempty"`
}

func decodeUsing(t *testing.T, raw string) []usingType {
	t.Helper()
	var result []usingType
	require.NoError(t, json.Unmarshal([]byte(raw), &result))

	return result
}

func TestNewGoodsUsingCmd_JSONShape(t *testing.T) {
	goodsDir := writeGhFiles(t, map[string]string{
		"goods.test.yml": `---
- type: 耐用品
  topics:
    - topic: 收纳袋
      table:
        - name: 抽绳束口#防水#收纳袋（15D尼龙涂硅）
          brand: 三峰出
          price: "¥13"
          isUsing: true
        - name: 天纵被子收纳袋
          price: ¥84

    - topic: 速干浴巾
      table:
        - name: 速干浴巾 NH19Y001-J
          brand: 挪客
          isUsing: true
`,
	})

	out, err := captureStdout(t, func() error {
		cmd := newRootCmd()
		cmd.SetArgs([]string{"goods", "using", "--path", goodsDir})
		return cmd.Execute()
	})
	require.NoError(t, err)

	types := decodeUsing(t, out)
	require.Len(t, types, 1)
	assert.Equal(t, "耐用品", types[0].Type)
	require.Len(t, types[0].Topics, 2)
	assert.Equal(t, "收纳袋", types[0].Topics[0].Topic)
	require.Len(t, types[0].Topics[0].Items, 1)
	assert.Equal(t, "抽绳束口#防水#收纳袋（15D尼龙涂硅）", types[0].Topics[0].Items[0].Name)
	assert.Equal(t, "三峰出", types[0].Topics[0].Items[0].Brand)
	// 未标记 isUsing 的不出现
	assert.Len(t, types[0].Topics[1].Items, 1)
	// schema 严格枚举 goods 行 key，输出不含 extra 字段
}

func TestNewGoodsUsingCmd_EmptyResult(t *testing.T) {
	emptyDir := t.TempDir()

	out, err := captureStdout(t, func() error {
		cmd := newRootCmd()
		cmd.SetArgs([]string{"goods", "using", "--path", emptyDir})
		return cmd.Execute()
	})
	require.NoError(t, err)

	tags := decodeUsing(t, out)
	assert.Empty(t, tags)
}

func TestNewGoodsUsingCmd_ErrorOnMissingDir(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"goods", "using", "--path", filepath.Join(t.TempDir(), "missing")})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list goods files")
}
