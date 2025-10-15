package parser

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	yaml "github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/parser"
)

// Parser YAML配置解析器.
type Parser[T any] struct {
	fileName string
	data     []byte
}

// NewParser 创建解析器.
func NewParser[T any](data []byte) *Parser[T] {
	return &Parser[T]{
		data: data,
	}
}

// WithFileName 设置文件名.
func (p *Parser[T]) WithFileName(fileName string) *Parser[T] {
	p.fileName = fileName

	return p
}

// ParseSingle 解析单个YAML文档.
func (p *Parser[T]) ParseSingle() (T, error) {
	var result T
	decoder := yaml.NewDecoder(bytes.NewReader(p.data))
	if err := decoder.Decode(&result); err != nil {
		if p.fileName != "" {
			return result, fmt.Errorf("%s 解析配置失败: %w", p.fileName, err)
		}

		return result, fmt.Errorf("解析配置失败: %w", err)
	}

	return result, nil
}

// ParseMulti 解析多文档 Markdown.
func (p *Parser[T]) ParseMulti() ([]T, error) {
	var results []T
	decoder := yaml.NewDecoder(bytes.NewReader(p.data))

	for {
		var item T
		err := decoder.Decode(&item)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("解析配置失败: %w", err)
		}
		results = append(results, item)
	}

	return results, nil
}

// ParseFlatten 解析多文档 Markdown 并展开结果
// 适用于每个文档都是 slice 且需要合并的情况.
func (p *Parser[T]) ParseFlatten() ([]T, error) {
	var results []T
	decoder := yaml.NewDecoder(bytes.NewReader(p.data))

	for {
		var item []T
		err := decoder.Decode(&item)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("解析配置失败: %w", err)
		}
		results = append(results, item...)
	}

	return results, nil
}

// IsMultiDocument checks if the YAML content contains multiple documents.
func (p *Parser[T]) IsMultiDocument() (bool, error) {
	file, err := parser.ParseBytes(p.data, parser.ParseComments)
	if err != nil {
		return false, fmt.Errorf("解析YAML失败: %w", err)
	}

	return len(file.Docs) > 1, nil
}
