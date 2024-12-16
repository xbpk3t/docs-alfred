package utils

import (
	"bytes"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// Parser YAML配置解析器
type Parser struct {
	decoder *yaml.Decoder
}

// NewParser 创建新的解析器
func NewParser(data []byte) *Parser {
	return &Parser{
		decoder: yaml.NewDecoder(bytes.NewReader(data)),
	}
}

// Parse 解析配置
func Parse[T any](data []byte) ([]T, error) {
	var result []T
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	for {
		var item T
		if err := decoder.Decode(&item); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("解析配置失败: %w", err)
		}
		result = append(result, item)
	}
	return result, nil
}
