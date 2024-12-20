package parser

import (
	"bytes"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// Parser YAML配置解析器
type Parser[T any] struct {
	data []byte
}

// NewParser 创建解析器
func NewParser[T any](data []byte) *Parser[T] {
	return &Parser[T]{
		data: data,
	}
}

// ParseSingle 解析单个YAML文档
func (p *Parser[T]) ParseSingle() (T, error) {
	var result T
	decoder := yaml.NewDecoder(bytes.NewReader(p.data))
	if err := decoder.Decode(&result); err != nil {
		return result, fmt.Errorf("解析配置失败: %w", err)
	}
	return result, nil
}

// ParseMulti 解析多文档YAML
func (p *Parser[T]) ParseMulti() ([]T, error) {
	var results []T
	decoder := yaml.NewDecoder(bytes.NewReader(p.data))

	for {
		var item T
		err := decoder.Decode(&item)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("解析配置失败: %w", err)
		}
		results = append(results, item)
	}
	return results, nil
}
