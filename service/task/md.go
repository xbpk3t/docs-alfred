package task

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/xbpk3t/docs-alfred/pkg/render"
)

// TaskRenderer Markdown渲染器
type TaskRenderer struct {
	renderer render.MarkdownRenderer
}

// NewTaskRenderer 创建新的渲染器
func NewTaskRenderer() *TaskRenderer {
	return &TaskRenderer{
		renderer: render.NewMarkdownRenderer(),
	}
}

// Render 渲染文档
func (r *TaskRenderer) Render(data []byte) (string, error) {
	// 获取目录路径
	dirPath := string(data)
	dirPath = strings.TrimSpace(dirPath)

	// 获取相对路径
	parentDir := filepath.Base(dirPath) // 获取父目录名

	// 读取目录下的所有文件
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return "", err
	}

	// 收集所有 yml 文件
	var ymlFiles []string
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".yml" || !strings.HasPrefix(file.Name(), "task") {
			continue
		}
		ymlFiles = append(ymlFiles, file.Name())
	}
	sort.Strings(ymlFiles)

	// 添加头部内容
	r.renderer.RenderMetadata(map[string]string{
		"slug": "/",
	})

	for _, file := range ymlFiles {
		name := strings.TrimSuffix(file, ".yml")
		// 移除 task- 前缀
		name = strings.TrimPrefix(name, "task-")

		// 渲染标题
		r.renderer.RenderHeader(render.HeadingLevel2, name)

		// 渲染导入语句和代码块
		r.renderer.RenderImport(name, "../"+parentDir+"/"+file)
		r.renderer.RenderContainer("{"+name+"}", "yaml")
	}

	return r.renderer.String(), nil
}
