package render

import (
	"bytes"

	"github.com/bytedance/sonic"
	"gopkg.in/yaml.v3"
)

// JSONRenderer JSON渲染器
type JSONRenderer struct {
	Cmd         string
	PrettyPrint bool
}

// NewJSONRenderer 创建新的JSON渲染器
func NewJSONRenderer(cmd string, prettyPrint bool) *JSONRenderer {
	return &JSONRenderer{
		PrettyPrint: prettyPrint,
		Cmd:         cmd,
	}
}

// Render 实现 Renderer 接口
func (j *JSONRenderer) Render(data []byte) (string, error) {
	// 分割多个YAML文档
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var documents []interface{}

	for {
		var doc interface{}
		err := decoder.Decode(&doc)
		if err != nil {
			break // 到达文件末尾
		}
		if doc != nil {
			documents = append(documents, doc)
		}
	}

	// 如果只有一个文档，直接返回它
	var dataToEncode interface{} = documents
	if len(documents) == 1 {
		dataToEncode = documents[0]
	}

	// 转换为JSON
	var result []byte
	var err error
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

//type JSONWriter interface {
//	Render(data []byte) (string, error)
//}
//
//type JSONBuilder struct {
//	builder strings.Builder
//}
