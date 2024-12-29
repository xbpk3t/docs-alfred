package task

import (
	"path/filepath"

	"github.com/xbpk3t/docs-alfred/pkg/render"
)

// TaskRenderer Markdown渲染器
type TaskRenderer struct {
	render.MarkdownRenderer
}

// NewTaskRenderer 创建新的渲染器
func NewTaskRenderer() *TaskRenderer {
	return &TaskRenderer{
		MarkdownRenderer: render.NewMarkdownRenderer(),
	}
}

// Render 渲染文档
func (r *TaskRenderer) Render(data []byte) (string, error) {
	// 写入头部内容
	r.RenderMetadata(map[string]string{
		"slug": "/",
	})

	// 渲染任务内容
	r.RenderHeader(render.HeadingLevel2, "Task")
	r.RenderDocusaurusRawLoader("task", filepath.Join("..", "task.yml"))

	return r.String(), nil
}
