package diary

import (
	"github.com/bytedance/sonic"
	"github.com/xbpk3t/docs-alfred/pkg/parser"
	"github.com/xbpk3t/docs-alfred/pkg/render"
)

// DiaryJSONRender 日记渲染器
type DiaryJSONRender struct {
	currentFile string
	renderer    render.JSONRenderer
}

// NewDiaryJSONRender 创建日记渲染器
func NewDiaryJSONRender() *DiaryJSONRender {
	return &DiaryJSONRender{
		renderer: render.JSONRenderer{
			PrettyPrint: true,
			ParseMode:   render.ParseFlatten,
		},
	}
}

// GetCurrentFileName 获取当前处理的文件名
func (djr *DiaryJSONRender) GetCurrentFileName() string {
	return djr.currentFile
}

// SetCurrentFile 设置当前处理的文件名
func (djr *DiaryJSONRender) SetCurrentFile(filename string) {
	djr.currentFile = filename
}

func ParseConfig(data []byte) ([]Task, error) {
	return parser.NewParser[Task](data).ParseFlatten()
}

func (djr *DiaryJSONRender) Render(data []byte) (string, error) {
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
