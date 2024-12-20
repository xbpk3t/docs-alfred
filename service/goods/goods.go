package goods

import (
	"fmt"
	"github.com/xbpk3t/docs-alfred/pkg/parser"
	"github.com/xbpk3t/docs-alfred/pkg/render"
	"strings"
)

// Goods 定义商品配置结构
type Goods struct {
	Type  string `yaml:"type"`
	Tag   string `yaml:"tag"`
	Items []Item `yaml:"goods"`
	Des   string `yaml:"des,omitempty"`
	QA    []QA   `yaml:"qs,omitempty"`
}

// Item 定义单个商品项
type Item struct {
	Name  string   `yaml:"name"`
	Param string   `yaml:"param,omitempty"`
	Price string   `yaml:"price,omitempty"`
	Des   string   `yaml:"des,omitempty"`
	URL   string   `yaml:"url,omitempty"`
	Date  []string `yaml:"date,omitempty"`
	Use   bool     `yaml:"use,omitempty"`
}

// QA 定义问答结构
type QA struct {
	Question string   `yaml:"q"`
	Answer   string   `yaml:"x"`
	Steps    []string `yaml:"s"`
}

// GoodsRenderer Markdown渲染器
type GoodsRenderer struct {
	render.MarkdownRenderer
	seenTags map[string]bool
}

// NewGoodsRenderer 创建新的渲染器
func NewGoodsRenderer() *GoodsRenderer {
	return &GoodsRenderer{
		seenTags: make(map[string]bool),
	}
}

// Render 渲染商品数据
func (r *GoodsRenderer) Render(data []byte) (string, error) {
	goods, err := ParseConfig(data)
	if err != nil {
		return "", fmt.Errorf("解析配置失败: %w", err)
	}

	for _, item := range goods {
		// 渲染标签标题
		if !r.seenTags[item.Tag] {
			r.RenderHeader(2, item.Tag)
			r.seenTags[item.Tag] = true
		}

		// 渲染类型标题
		r.RenderHeader(3, item.Type)

		// 渲染商品内容
		r.Write(item.RenderMarkdown())
	}

	return r.String(), nil
}

// ParseConfig 解析配置文件
func ParseConfig(data []byte) ([]Goods, error) {
	return parser.NewParser[Goods](data).ParseMulti()
}

// RenderMarkdown 渲染为 Markdown 格式
func (g *Goods) RenderMarkdown() string {
	var content strings.Builder

	// 渲染商品描述
	if g.Des != "" {
		content.WriteString(fmt.Sprintf("%s\n\n", g.Des))
	}

	// 渲染商品列表
	content.WriteString(g.renderItems())

	// 渲染问答部分
	if qaContent := g.renderQA(); qaContent != "" {
		content.WriteString(qaContent)
	}

	return content.String()
}

// renderItems 渲染商品项
func (g *Goods) renderItems() string {
	var content strings.Builder
	for _, item := range g.Items {
		summary := item.formatSummary()
		if details := item.formatDetails(); details != "" {
			content.WriteString(fmt.Sprintf("\n<details>\n<summary>%s</summary>\n\n%s\n\n</details>\n\n",
				summary, details))
		} else {
			content.WriteString(fmt.Sprintf("- %s\n", summary))
		}
	}
	return content.String()
}

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
	if len(i.Date) > 0 {
		details = append(details, fmt.Sprintf("- 购买时间: %s", strings.Join(i.Date, ", ")))
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

// renderQA 渲染问答部分
func (g *Goods) renderQA() string {
	if len(g.QA) == 0 {
		return ""
	}

	var content strings.Builder
	for _, qa := range g.QA {
		if details := qa.formatContent(); details != "" {
			content.WriteString(fmt.Sprintf("\n<details>\n<summary>%s</summary>\n\n%s\n\n</details>\n\n",
				qa.Question, details))
		} else {
			content.WriteString(fmt.Sprintf("- %s\n", qa.Question))
		}
	}

	return fmt.Sprintf("\n---\n:::%s[%s]\n\n%s\n\n:::\n\n",
		render.AdmonitionTip, "常见问题", content.String())
}

// formatContent 格式化问答内容
func (qa *QA) formatContent() string {
	var parts []string

	// 添加步骤
	if len(qa.Steps) > 0 {
		var steps strings.Builder
		for _, step := range qa.Steps {
			steps.WriteString(fmt.Sprintf("- %s\n", step))
		}
		parts = append(parts, steps.String())
	}

	// 添加分隔符
	if len(qa.Steps) > 0 && qa.Answer != "" {
		parts = append(parts, "---")
	}

	// 添加答案
	if qa.Answer != "" {
		parts = append(parts, qa.Answer)
	}

	return strings.Join(parts, "\n\n")
}
