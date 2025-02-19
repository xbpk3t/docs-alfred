package render

import (
	"github.com/goccy/go-yaml"
	"github.com/xbpk3t/docs-alfred/pkg/parser"
)

// YAMLRenderer JSON渲染器
type YAMLRenderer struct {
	Cmd         string
	PrettyPrint bool
	ParseMode   ParseMode
}

// NewJSONRenderer 创建新的JSON渲染器
func NewYAMLRenderer(cmd string, prettyPrint bool) *YAMLRenderer {
	return &YAMLRenderer{
		PrettyPrint: prettyPrint,
		Cmd:         cmd,
		ParseMode:   ParseSingle, // 默认使用单文档模式
	}
}

// WithParseMode 设置解析模式
func (j *YAMLRenderer) WithParseMode(mode ParseMode) *YAMLRenderer {
	j.ParseMode = mode
	return j
}

// Render 实现 Renderer 接口
func (j *YAMLRenderer) Render(data []byte) (string, error) {
	var dataToEncode interface{}

	// 根据解析模式选择不同的解析方法
	ps := parser.NewParser[interface{}](data)
	var err error

	switch j.ParseMode {
	case ParseMulti:
		dataToEncode, err = ps.ParseMulti()
	case ParseFlatten:
		dataToEncode, err = ps.ParseFlatten()
	default: // ParseSingle
		dataToEncode, err = ps.ParseSingle()
	}

	if err != nil {
		return "", err
	}

	// 转换为YAML
	var result []byte
	result, err = yaml.Marshal(dataToEncode)
	if err != nil {
		return "", err
	}

	return string(result), nil
}
