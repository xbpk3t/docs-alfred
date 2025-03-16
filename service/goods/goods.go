package goods

import (
	"github.com/xbpk3t/docs-alfred/pkg/parser"
	"github.com/xbpk3t/docs-alfred/service/gh"
)

// Goods 定义商品配置结构
type Goods struct {
	Tag        string `yaml:"tag"`
	GoodsItems []struct {
		Type   string     `yaml:"type"`
		Des    string     `yaml:"des,omitempty"`
		Topics []gh.Topic `yaml:"tpcs,omitempty"`
		Using  []Item     `yaml:"using"`
		Item   []Item     `yaml:"item"`
		Score  int        `yaml:"score"`
	} `yaml:"item"`
}

// Item 定义单个商品项
type Item struct {
	Name   string   `yaml:"name"`
	Param  string   `yaml:"param,omitempty"`
	Price  string   `yaml:"price,omitempty"`
	Des    string   `yaml:"des,omitempty"`
	URL    string   `yaml:"url,omitempty"`
	Record []string `yaml:"record,omitempty"`
	Use    bool     `yaml:"use,omitempty"`
}

// ParseConfig 解析配置文件
func ParseConfig(data []byte) ([]Goods, error) {
	return parser.NewParser[Goods](data).ParseFlatten()
}
