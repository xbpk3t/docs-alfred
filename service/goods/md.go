package goods

import (
	"fmt"
	"strings"

	"github.com/xbpk3t/docs-alfred/pkg/render"
)

// GoodsMarkdownRender Markdown渲染器
type GoodsMarkdownRender struct {
	seenTags map[string]bool
	renderer render.MarkdownRenderer
}

// NewGoodsMarkdownRenderer 创建新的渲染器
func NewGoodsMarkdownRenderer() *GoodsMarkdownRender {
	return &GoodsMarkdownRender{
		seenTags: make(map[string]bool),
		renderer: render.NewMarkdownRenderer(),
	}
}

// TODO 类似 wiki对qs的问题

// Render 渲染商品数据
//func (r *GoodsMarkdownRender) Render(data []byte) (string, error) {
//	goods, err := ParseConfig(data)
//	if err != nil {
//		return "", errcode.WithError(errcode.ErrParseConfig, err)
//	}
//
//	for _, item := range goods {
//		// 渲染标签标题
//		if !r.seenTags[item.Tag] {
//			r.RenderHeader(render.HeadingLevel2, item.Tag)
//			r.seenTags[item.Tag] = true
//		}
//
//		// 渲染类型标题
//		r.RenderHeader(render.HeadingLevel3, item.Type)
//
//		// 渲染商品内容
//		r.Write(item.RenderMarkdown())
//	}
//
//	return r.String(), nil
//}

// RenderMarkdown 渲染为 Markdown 格式
//func (g *Goods) RenderMarkdown() string {
//	var content strings.Builder
//
//	// 渲染商品描述
//	if g.Des != "" {
//		content.WriteString(fmt.Sprintf("%s\n\n", g.Des))
//	}
//
//	// 渲染商品列表
//	content.WriteString(g.renderItems())
//
//	// 渲染问答部分
//	if qaContent := g.renderQA(); qaContent != "" {
//		content.WriteString(qaContent)
//	}
//
//	return content.String()
//}

// renderItems 渲染商品项
//func (g *Goods) renderItems() string {
//	var content strings.Builder
//	for _, item := range g.Item {
//		summary := item.formatSummary()
//		if details := item.formatDetails(); details != "" {
//			content.WriteString(fmt.Sprintf("\n<details>\n<summary>%s</summary>\n\n%s\n\n</details>\n\n",
//				summary, details))
//		} else {
//			content.WriteString(fmt.Sprintf("- %s\n", summary))
//		}
//	}
//	return content.String()
//}

// formatSummary 格式化商品摘要
func (i *Item) formatSummary() string {
	mark := "~~"
	if i.Use {
		mark = "***"
	}

	if i.URL != "" {
		return fmt.Sprintf("%s[%s](%s)%s", mark, i.Name, i.URL, mark)
	}
	return fmt.Sprintf("%s%s%s", mark, i.Name, mark)
}

// formatDetails 格式化商品详情
func (i *Item) formatDetails() string {
	var details []string

	// 添加商品参数
	if i.Param != "" {
		details = append(details, fmt.Sprintf("- 参数: %s", i.Param))
	}
	if i.Price != "" {
		details = append(details, fmt.Sprintf("- 价格: %s", i.Price))
	}

	// 添加商品描述
	if i.Des != "" {
		if len(details) > 0 {
			details = append(details, "---")
		}
		details = append(details, i.Des)
	}

	return strings.Join(details, "\n")
}

//// renderQA 渲染问答部分
//func (g *Goods) renderQA() string {
//	if len(g.QA) == 0 {
//		return ""
//	}
//
//	var content strings.Builder
//	for _, qa := range g.QA {
//		if details := qa.formatContent(); details != "" {
//			content.WriteString(fmt.Sprintf("\n<details>\n<summary>%s</summary>\n\n%s\n\n</details>\n\n",
//				qa.Question, details))
//		} else {
//			content.WriteString(fmt.Sprintf("- %s\n", qa.Question))
//		}
//	}
//
//	return fmt.Sprintf("\n---\n:::%s[%s]\n\n%s\n\n:::\n\n",
//		render.AdmonitionTip, "常见问题", content.String())
//}

// Write implements writing content
func (r *GoodsMarkdownRender) Write(s string) {
	r.renderer.Write(s)
}

// String implements getting result
func (r *GoodsMarkdownRender) String() string {
	return r.renderer.String()
}

// RenderHeader implements rendering header
func (r *GoodsMarkdownRender) RenderHeader(level int, text string) {
	r.renderer.RenderHeader(level, text)
}

// RenderFold implements rendering fold content
func (r *GoodsMarkdownRender) RenderFold(summary, details string) {
	r.renderer.RenderFold(summary, details)
}

// RenderListItem implements rendering list item
func (r *GoodsMarkdownRender) RenderListItem(text string) {
	r.renderer.RenderListItem(text)
}
