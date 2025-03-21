package render

import (
	"github.com/xbpk3t/docs-alfred/pkg/parser"
	"sigs.k8s.io/yaml"
)

// ParseMode 解析模式
type ParseMode int

const (
	ParseSingle ParseMode = iota
	ParseMulti
	ParseFlatten
)

// YAMLRenderer YAML渲染器
type YAMLRenderer struct {
	Cmd         string
	PrettyPrint bool
	ParseMode   ParseMode
}

// NewYAMLRenderer 创建新的YAML渲染器
func NewYAMLRenderer(cmd string, prettyPrint bool) *YAMLRenderer {
	return &YAMLRenderer{
		PrettyPrint: prettyPrint,
		Cmd:         cmd,
		ParseMode:   ParseSingle, // 默认使用单文档模式
	}
}

// WithParseMode 设置解析模式
func (j *YAMLRenderer) WithParseMode(mode ParseMode) {
	j.ParseMode = mode
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
	result, err := yaml.Marshal(dataToEncode)
	if err != nil {
		return "", err
	}

	return string(result), nil
}
