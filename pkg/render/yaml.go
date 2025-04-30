package render

import (
	"github.com/goccy/go-yaml"
	"github.com/xbpk3t/docs-alfred/pkg/parser"
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
	dataToEncode, err := j.ParseData(data)
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

// parseData 根据不同模式解析数据，并统一返回类型
func (j *YAMLRenderer) ParseData(data []byte) (interface{}, error) {
	ps := parser.NewParser[interface{}](data)

	switch j.ParseMode {
	case ParseMulti:
		// ParseMulti 返回 [][]interface{}
		return ps.ParseMulti()
	case ParseFlatten:
		// ParseFlatten 返回 []interface{}
		return ps.ParseFlatten()
	default:
		// ParseSingle 返回 map[string]interface{}
		return ps.ParseSingle()
	}
}
