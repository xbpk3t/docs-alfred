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
- topic: 收纳袋
  score: 5
  table:
    - name: 抽绳束口收纳袋
      brand: 三峰出
      price: ¥13
      isUsing: true
`)
	result, err := r.Render(data)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "收纳袋")
	assert.Contains(t, result, "抽绳束口收纳袋")
	assert.Contains(t, result, "isUsing")
}

func TestGoodsYAMLRender_RenderInvalidYAML(t *testing.T) {
	r := NewGoodsYAMLRender()
	data := []byte(`invalid: [yaml: broken`)
	_, err := r.Render(data)
	require.Error(t, err)
}
