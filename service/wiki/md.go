package wiki

import (
	"strings"

	"github.com/xbpk3t/docs-alfred/pkg/render"

	"github.com/samber/lo"
)

// WorkRenderer Markdown渲染器
type WorkRenderer struct {
	seenTags map[string]bool
	renderer render.MarkdownRenderer
}

// NewWorkRenderer 创建新的渲染器
func NewWorkRenderer() *WorkRenderer {
	return &WorkRenderer{
		seenTags: make(map[string]bool),
		renderer: render.NewMarkdownRenderer(),
	}
}

// Render 渲染文档
func (r *WorkRenderer) Render(data []byte) (string, error) {
	docs, err := ParseConfig(data)
	if err != nil {
		return "", err
	}

	for _, doc := range docs {
		for _, d := range doc {
			if !r.seenTags[d.Tag] {
				r.RenderHeader(render.HeadingLevel2, d.Tag)
				r.seenTags[d.Tag] = true
			}

			if d.Tag != d.Type {
				r.RenderHeader(render.HeadingLevel3, d.Type)
			}

			r.Write(d.RenderContent())
		}
	}

	return r.String(), nil
}

// RenderContent 渲染文档内容
func (d *Doc) RenderContent() string {
	if len(d.Topics) == 0 {
		return ""
	}

	var content strings.Builder
	// TODO wiki里的qs直接复用gh的qs之后，导致之前渲染qs的代码已经不可用了。所以全部移除掉了，之后这部分代码都直接调用gh里的渲染qs代码。
	//for _, qa := range d.Topics {
	//	content.WriteString(qa.Render())
	//}
	return content.String()
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
		for _, qa := range doc.Topics {
			if strings.Contains(strings.ToLower(qa.Topic), query) {
				results = append(results, qa.Topic)
			}
		}
	}

	return results
}

// Write implements writing content
func (r *WorkRenderer) Write(s string) {
	r.renderer.Write(s)
}

// String implements getting result
func (r *WorkRenderer) String() string {
	return r.renderer.String()
}

// RenderHeader implements rendering header
func (r *WorkRenderer) RenderHeader(level int, text string) {
	r.renderer.RenderHeader(level, text)
}

// RenderFold implements rendering fold content
func (r *WorkRenderer) RenderFold(summary, details string) {
	r.renderer.RenderFold(summary, details)
}

// RenderListItem implements rendering list item
func (r *WorkRenderer) RenderListItem(text string) {
	r.renderer.RenderListItem(text)
}
