package goods

import (
	yaml "github.com/goccy/go-yaml"
	modelgoods "github.com/xbpk3t/docs-alfred/internal/gh/model/goods"
	"github.com/xbpk3t/docs-alfred/pkg/parser"
	"github.com/xbpk3t/docs-alfred/pkg/render"
)

// GoodsYAMLRender 商品 YAML 渲染器.
type GoodsYAMLRender struct {
	*render.YAMLRenderer
}

// NewGoodsYAMLRender 创建新的商品 YAML 渲染器.
func NewGoodsYAMLRender() *GoodsYAMLRender {
	return &GoodsYAMLRender{
		YAMLRenderer: render.NewYAMLRenderer("goods", true),
	}
}

// Render 渲染商品数据.
func (g *GoodsYAMLRender) Render(data []byte) (string, error) {
	// 解析YAML数据为 schema 生成模型；新布局下每个文件根是扁平 topic 数组。
	topics, err := parser.NewParser[modelgoods.Topic](data).ParseFlatten()
	if err != nil {
		return "", err
	}

	// 将数据编码为YAML格式
	result, err := yaml.Marshal(topics)
	if err != nil {
		return "", err
	}

	return string(result), nil
}
