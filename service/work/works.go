package work

import (
	"fmt"
	"github.com/xbpk3t/docs-alfred/pkg/render"
	"strings"

	"github.com/samber/lo"
	"github.com/xbpk3t/docs-alfred/pkg"
)

// Doc 定义文档结构
type Doc struct {
	Type string `yaml:"type"`
	Tag  string `yaml:"tag"`
	Qs   []QA   `yaml:"qs"`
}

// QA 定义问答结构
type QA struct {
	Question string   `yaml:"q"` // 问题
	Answer   string   `yaml:"x"` // 答案
	URL      string   `yaml:"u"` // 链接
	Pictures []string `yaml:"p"` // 图片
	SubQs    []string `yaml:"s"` // 子问题
}

// Docs 文档集合
type Docs []Doc

// WorkRenderer Markdown渲染器
type WorkRenderer struct {
	render.MarkdownRenderer
	seenTags map[string]bool
}

// NewWorkRenderer 创建新的渲染器
func NewWorkRenderer() *WorkRenderer {
	return &WorkRenderer{
		seenTags: make(map[string]bool),
	}
}

// Render 渲染文档
func (r *WorkRenderer) Render(data []byte) (string, error) {
	docs, err := pkg.Parse[Doc](data)
	if err != nil {
		return "", err
	}

	for _, doc := range docs {
		if !r.seenTags[doc.Tag] {
			r.RenderHeader(2, doc.Tag)
			r.seenTags[doc.Tag] = true
		}

		if doc.Tag != doc.Type {
			r.RenderHeader(3, doc.Type)
		}

		r.Write(doc.RenderContent())
	}

	return r.String(), nil
}

// RenderContent 渲染文档内容
func (d *Doc) RenderContent() string {
	if len(d.Qs) == 0 {
		return ""
	}

	var content strings.Builder
	for _, qa := range d.Qs {
		content.WriteString(qa.Render())
	}
	return content.String()
}

// Render 渲染问答内容
func (qa *QA) Render() string {
	summary := qa.formatSummary()
	if details := qa.formatDetails(); details != "" {
		return fmt.Sprintf("\n<details>\n<summary>%s</summary>\n\n%s\n\n</details>\n\n",
			summary, details)
	}
	return fmt.Sprintf("- %s\n", summary)
}

// formatSummary 格式化问答摘要
func (qa *QA) formatSummary() string {
	if qa.URL != "" {
		return fmt.Sprintf("[%s](%s)", qa.Question, qa.URL)
	}
	return qa.Question
}

// formatDetails 格式化问答详情
func (qa *QA) formatDetails() string {
	var parts []string
	renderer := render.NewMarkdownRenderer()

	// 处理图片
	if len(qa.Pictures) > 0 {
		var pictures strings.Builder
		for _, pic := range qa.Pictures {
			renderer.RenderImageWithFigcaption(pic)
			pictures.WriteString(renderer.String())
		}
		parts = append(parts, pictures.String())
	}

	// 处理子问题
	if len(qa.SubQs) > 0 {
		var steps strings.Builder
		for _, subQ := range qa.SubQs {
			steps.WriteString(fmt.Sprintf("- %s\n", subQ))
		}
		parts = append(parts, steps.String())
	}

	// 处理答案
	if qa.Answer != "" {
		if len(parts) > 0 {
			parts = append(parts, "---")
		}
		parts = append(parts, qa.Answer)
	}

	return strings.Join(parts, "\n\n")
}

// GetTypes 获取所有类型
func (docs Docs) GetTypes() []string {
	return lo.Uniq(lo.Map(docs, func(d Doc, _ int) string {
		return d.Type
	}))
}

// GetTypesByTag 根据标签获取类型
func (docs Docs) GetTypesByTag(tag string) []string {
	filtered := lo.Filter(docs, func(d Doc, _ int) bool {
		return d.Tag == tag
	})
	return lo.Map(filtered, func(d Doc, _ int) string {
		return d.Type
	})
}

// ContainsType 检查是否包含指定类型
func (docs Docs) ContainsType(query string) bool {
	return lo.ContainsBy(docs.GetTypes(), func(t string) bool {
		return strings.EqualFold(t, query)
	})
}

// SearchQuestions 搜索问题
func (docs Docs) SearchQuestions(query string) []string {
	query = strings.ToLower(query)
	var results []string

	for _, doc := range docs {
		for _, qa := range doc.Qs {
			if strings.Contains(strings.ToLower(qa.Question), query) {
				results = append(results, qa.Question)
			}
		}
	}

	return results
}
