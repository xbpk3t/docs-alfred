package goods

import (
	yaml "github.com/goccy/go-yaml"
	"github.com/xbpk3t/docs-alfred/pkg/parser"
	"github.com/xbpk3t/docs-alfred/service/gh"
)

// Goods 定义商品配置结构
type Goods struct {
	Tag    string          `yaml:"tag" json:"tag"`
	Type   string          `yaml:"type" json:"type"`
	Des    string          `yaml:"des,omitempty" json:"des"`
	Using  Item            `yaml:"using" json:"using"`
	Topics []gh.Topic      `yaml:"topics,omitempty" json:"topics"`
	Item   []yaml.MapSlice `yaml:"item" json:"item"`
	Record []string        `yaml:"record,omitempty" json:"record"`
	Score  int             `yaml:"score" json:"score"`
}

// Item 定义单个商品项
type Item struct {
	Name  string `yaml:"name" json:"name"`
	Param string `yaml:"param,omitempty" json:"param"`
	Price string `yaml:"price,omitempty" json:"price"`
	Date  string `yaml:"date,omitempty" json:"date"`
	Des   string `yaml:"des,omitempty" json:"des"`
	URL   string `yaml:"url,omitempty" json:"url"`
	Use   bool   `yaml:"use,omitempty" json:"use"`
}

// ParseConfig 解析配置文件
func ParseConfig(data []byte) ([]Goods, error) {
	return parser.NewParser[Goods](data).ParseFlatten()
}
