package parser

import (
	"os"

	"github.com/xbpk3t/docs-alfred/pkg/errcode"
	"gopkg.in/yaml.v3"
)

// Parser YAML解析器
type Parser[T any] struct {
	data []byte
}

// NewParser 创建新的解析器
func NewParser[T any](data []byte) *Parser[T] {
	return &Parser[T]{
		data: data,
	}
}

// ParseSingle 解析单个配置
func (p *Parser[T]) ParseSingle() (T, error) {
	var result T
	if err := yaml.Unmarshal(p.data, &result); err != nil {
		return result, errcode.WithError(errcode.ErrParseConfig, err)
	}
	return result, nil
}

// ParseFlatten 解析并展平配置
func (p *Parser[T]) ParseFlatten() ([]T, error) {
	var result []T
	if err := yaml.Unmarshal(p.data, &result); err != nil {
		return nil, errcode.WithError(errcode.ErrParseConfig, err)
	}
	return result, nil
}

// ParseFromFile 从文件解析
func (p *Parser[T]) ParseFromFile(file string) ([]T, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, errcode.WithError(errcode.ErrReadFile, err)
	}

	var result []T
	if err := yaml.Unmarshal(data, &result); err != nil {
		return nil, errcode.WithError(errcode.ErrParseConfig, err)
	}
	return result, nil
}
