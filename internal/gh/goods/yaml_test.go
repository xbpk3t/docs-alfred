package goods

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGoodsYAMLRender(t *testing.T) {
	r := NewGoodsYAMLRender()
	require.NotNil(t, r)
	require.NotNil(t, r.YAMLRenderer)
}

func TestGoodsYAMLRender_Render(t *testing.T) {
	r := NewGoodsYAMLRender()
	data := []byte(`---
- type: 耳机
  tag: EDC
  score: 4
  using:
    name: AirPods Pro
    price: ¥1799
  item:
    - name: C50
      price: ¥179
      date: 2023-04-29
`)
	result, err := r.Render(data)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "耳机")
	assert.Contains(t, result, "EDC")
	assert.Contains(t, result, "C50")
}

func TestGoodsYAMLRender_RenderInvalidYAML(t *testing.T) {
	r := NewGoodsYAMLRender()
	data := []byte(`invalid: [yaml: broken`)
	_, err := r.Render(data)
	require.Error(t, err)
}
