package ws

import "github.com/xbpk3t/docs-alfred/pkg/parser"

// URL 定义单个URL结构
type URL struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
	Des  string `yaml:"des,omitempty"`
	Feed string `yaml:"feed"`
}

// WebStack 定义网站栈结构
type WebStack struct {
	Type string `yaml:"type"`
	URLs []URL  `yaml:"urls"`
}

// WebStacks WebStack集合
type WebStacks []WebStack

// ParseConfig 解析配置文件
func ParseConfig(data []byte) (WebStacks, error) {
	return parser.NewParser[[]WebStack](data).ParseSingle()
}
