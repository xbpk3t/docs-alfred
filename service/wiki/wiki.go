package wiki

import (
	"github.com/xbpk3t/docs-alfred/pkg/parser"
	"github.com/xbpk3t/docs-alfred/service/gh"
)

// Doc 定义文档结构
type Doc struct {
	Type string  `yaml:"type"`
	Tag  string  `yaml:"tag"`
	Qs   []gh.QA `yaml:"qs"`
}

// Docs 文档集合
type Docs []Doc

// ParseConfig 解析配置文件
func ParseConfig(data []byte) ([]Docs, error) {
	return parser.NewParser[Docs](data).ParseMulti()
}
