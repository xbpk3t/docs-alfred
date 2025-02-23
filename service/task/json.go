package task

import (
	"github.com/bytedance/sonic"
	"github.com/xbpk3t/docs-alfred/pkg/parser"
	"github.com/xbpk3t/docs-alfred/pkg/render"
)

// TaskJSONRender 日记渲染器
type TaskJSONRender struct {
	currentFile string
	renderer    render.JSONRenderer
}

// NewTaskJSONRender 创建日记渲染器
func NewTaskJSONRender() *TaskJSONRender {
	return &TaskJSONRender{
		renderer: render.JSONRenderer{
			PrettyPrint: true,
			ParseMode:   render.ParseFlatten,
		},
	}
}

// GetCurrentFileName 获取当前处理的文件名
func (djr *TaskJSONRender) GetCurrentFileName() string {
	return djr.currentFile
}

// SetCurrentFile 设置当前处理的文件名
func (djr *TaskJSONRender) SetCurrentFile(filename string) {
	djr.currentFile = filename
}

func ParseConfig(data []byte) ([]Task, error) {
	return parser.NewParser[Task](data).ParseFlatten()
}

func (djr *TaskJSONRender) Render(data []byte) (string, error) {
	// 使用 Tasks 类型来解析 YAML，使用 ParseFlatten 来处理多文档
	tasks, err := ParseConfig(data)
	if err != nil {
		return "", err
	}

	// 使用 sonic 进行 JSON 序列化，带缩进美化输出
	result, err := sonic.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return "", err
	}

	return string(result), nil
}
