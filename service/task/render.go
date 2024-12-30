package task

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

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
	// 获取目录路径
	dirPath := string(data)
	dirPath = strings.TrimSpace(dirPath)

	// 读取目录下的所有文件
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return "", err
	}

	// 收集所有 yml 文件
	var ymlFiles []string
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".yml" {
			continue
		}
		ymlFiles = append(ymlFiles, file.Name())
	}
	sort.Strings(ymlFiles)

	var content strings.Builder
	// 添加头部内容
	content.WriteString("---\nslug: /\n---\n\n")

	for _, file := range ymlFiles {
		name := strings.TrimSuffix(file, ".yml")
		// 移除 task- 前缀
		name = strings.TrimPrefix(name, "task-")

		fmt.Fprintf(&content, "## %s\n", name)
		fmt.Fprintf(&content, "import %s from '!!raw-loader!../task/%s';\n\n", name, file)
		fmt.Fprintf(&content, "<CodeBlock language=\"yaml\">{%s}</CodeBlock>\n\n", name)
	}

	return content.String(), nil
}
