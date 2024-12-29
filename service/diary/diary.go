package diary

import (
	"path/filepath"

	"github.com/xbpk3t/docs-alfred/pkg/render"
)

// DiaryRenderer Markdown渲染器
type DiaryRenderer struct {
	render.MarkdownRenderer
}

// NewDiaryRenderer 创建新的渲染器
func NewDiaryRenderer() *DiaryRenderer {
	return &DiaryRenderer{
		MarkdownRenderer: render.NewMarkdownRenderer(),
	}
}

// Render 渲染文档
func (r *DiaryRenderer) Render(data []byte) (string, error) {
	// 写入头部内容
	r.RenderMetadata(map[string]string{
		"slug": "/",
	})

	// 渲染日记内容
	r.RenderHeader(render.HeadingLevel2, "Diary")
	r.RenderDocusaurusRawLoader("diary", filepath.Join("..", "diary.yml"))

	return r.String(), nil
}
