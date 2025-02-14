package render

import (
	"github.com/bytedance/sonic"
	"github.com/xbpk3t/docs-alfred/pkg/parser"
)

// ParseMode 解析模式
type ParseMode int

const (
	ParseSingle ParseMode = iota
	ParseMulti
	ParseFlatten
)

// JSONRenderer JSON渲染器
type JSONRenderer struct {
	Cmd         string
	PrettyPrint bool
	ParseMode   ParseMode
}

// NewJSONRenderer 创建新的JSON渲染器
func NewJSONRenderer(cmd string, prettyPrint bool) *JSONRenderer {
	return &JSONRenderer{
		PrettyPrint: prettyPrint,
		Cmd:         cmd,
		ParseMode:   ParseSingle, // 默认使用单文档模式
	}
}

// WithParseMode 设置解析模式
func (j *JSONRenderer) WithParseMode(mode ParseMode) *JSONRenderer {
	j.ParseMode = mode
	return j
}

// Render 实现 Renderer 接口
func (j *JSONRenderer) Render(data []byte) (string, error) {
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

	// 转换为JSON
	var result []byte
	if j.PrettyPrint {
		result, err = sonic.MarshalIndent(dataToEncode, "", "  ")
	} else {
		result, err = sonic.Marshal(dataToEncode)
	}

	if err != nil {
		return "", err
	}

	return string(result), nil
}
