package goods

import "github.com/xbpk3t/docs-alfred/pkg/parser"

// Goods 定义商品配置结构
type Goods struct {
	Type  string `yaml:"type"`
	Tag   string `yaml:"tag"`
	Using []Item `yaml:"using"`
	Item  []Item `yaml:"item"`
	Des   string `yaml:"des,omitempty"`
	QA    []QA   `yaml:"qs,omitempty"`
	Score int    `yaml:"score"`
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

// QA 定义问答结构
type QA struct {
	Question     string   `yaml:"q"`
	Answer       string   `yaml:"x"`
	SubQuestions []string `yaml:"s"`
}

// ParseConfig 解析配置文件
func ParseConfig(data []byte) ([]Goods, error) {
	return parser.NewParser[Goods](data).ParseFlatten()
}
