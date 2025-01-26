package render

import (
	"github.com/bytedance/sonic"
)

// JSONRenderer JSON渲染器
type JSONRenderer struct {
	// 可以添加配置选项
	PrettyPrint bool
}

// NewJSONRenderer 创建新的JSON渲染器
func NewJSONRenderer(prettyPrint bool) *JSONRenderer {
	return &JSONRenderer{
		PrettyPrint: prettyPrint,
	}
}

// Render 实现 Renderer 接口
func (j *JSONRenderer) Render(data []byte) (string, error) {
	// 首先解析YAML数据到通用接口
	var parsedData interface{}
	if err := sonic.Unmarshal(data, &parsedData); err != nil {
		return "", err
	}

	// 根据配置选择是否美化输出
	var result []byte
	var err error
	if j.PrettyPrint {
		result, err = sonic.MarshalIndent(parsedData, "", "  ")
	} else {
		result, err = sonic.Marshal(parsedData)
	}

	if err != nil {
		return "", err
	}

	return string(result), nil
}

//type JSONWriter interface {
//	Render(data []byte) (string, error)
//}
//
//type JSONBuilder struct {
//	builder strings.Builder
//}
