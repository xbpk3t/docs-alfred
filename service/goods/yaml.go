package goods

import (
	yaml "github.com/goccy/go-yaml"
	"github.com/xbpk3t/docs-alfred/pkg/parser"
	"github.com/xbpk3t/docs-alfred/pkg/render"
	"github.com/xbpk3t/docs-alfred/service"
)

// GoodsYAMLRender 商品 YAML 渲染器.
type GoodsYAMLRender struct {
	*render.YAMLRenderer
}

// NewGoodsYAMLRender 创建新的商品 YAML 渲染器.
func NewGoodsYAMLRender() *GoodsYAMLRender {
	return &GoodsYAMLRender{
		YAMLRenderer: render.NewYAMLRenderer(string(service.ServiceGoods), true),
	}
}

// Render 渲染商品数据.
func (g *GoodsYAMLRender) Render(data []byte) (string, error) {
	// 解析YAML数据为Goods类型
	goods, err := parser.NewParser[Goods](data).ParseFlatten()
	if err != nil {
		return "", err
	}

	// 将数据编码为YAML格式
	result, err := yaml.Marshal(goods)
	if err != nil {
		return "", err
	}

	return string(result), nil
}
