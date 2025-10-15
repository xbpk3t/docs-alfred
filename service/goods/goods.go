package goods

import (
	yaml "github.com/goccy/go-yaml"
	"github.com/xbpk3t/docs-alfred/pkg/parser"
	"github.com/xbpk3t/docs-alfred/service/gh"
)

// Goods 定义商品配置结构.
type Goods struct {
	Tag    string          `json:"tag"    yaml:"tag"`
	Type   string          `json:"type"   yaml:"type"`
	Des    string          `json:"des"    yaml:"des,omitempty"`
	Using  Item            `json:"using"  yaml:"using"`
	Topics []gh.Topic      `json:"topics" yaml:"topics,omitempty"`
	Item   []yaml.MapSlice `json:"item"   yaml:"item"`
	Record []string        `json:"record" yaml:"record,omitempty"`
	Score  int             `json:"score"  yaml:"score"`
}

// Item 定义单个商品项.
type Item struct {
	Name  string `json:"name"  yaml:"name"`
	Param string `json:"param" yaml:"param,omitempty"`
	Price string `json:"price" yaml:"price,omitempty"`
	Date  string `json:"date"  yaml:"date,omitempty"`
	Des   string `json:"des"   yaml:"des,omitempty"`
	URL   string `json:"url"   yaml:"url,omitempty"`
	Use   bool   `json:"use"   yaml:"use,omitempty"`
}

// ParseConfig 解析配置文件.
func ParseConfig(data []byte) ([]Goods, error) {
	return parser.NewParser[Goods](data).ParseFlatten()
}
