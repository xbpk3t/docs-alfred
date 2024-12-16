package goods

import (
	"github.com/xbpk3t/docs-alfred/utils"
)

// GoodsRenderer 商品渲染器
type GoodsRenderer struct {
	utils.MarkdownRenderer
	Config ConfigGoods
}

func (g *GoodsRenderer) Render(data []byte) (string, error) {
	f := NewConfigGoods(data)
	seenTags := make(map[string]bool)

	for _, item := range f {
		if !seenTags[item.Tag] {
			g.RenderHeader(2, item.Tag)
			seenTags[item.Tag] = true
		}

		g.RenderHeader(3, item.Type)
		g.renderGoodsContent(item)
	}
	return g.String(), nil
}

func (g *GoodsRenderer) renderGoodsContent(item ConfigGoodsX) {
	g.Write(AddMarkdownFormat(item))
	g.Write(AddTypeQs(item))
}
