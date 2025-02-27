package wiki

import "github.com/xbpk3t/docs-alfred/pkg/parser"

// Doc 定义文档结构
type Doc struct {
	Type string `yaml:"type"`
	Tag  string `yaml:"tag"`
	Qs   []QA   `yaml:"qs"`
}

// QA 定义问答结构
type QA struct {
	Question     string   `yaml:"q"` // 问题
	Answer       string   `yaml:"x"` // 答案
	URL          string   `yaml:"u"` // 链接
	Pictures     []string `yaml:"p"` // 图片
	SubQuestions []string `yaml:"s"` // 子问题
}

// Docs 文档集合
type Docs []Doc

// ParseConfig 解析配置文件
func ParseConfig(data []byte) ([]Docs, error) {
	return parser.NewParser[Docs](data).ParseMulti()
}
