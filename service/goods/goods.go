package goods

import (
	yaml "github.com/goccy/go-yaml"
	"github.com/xbpk3t/docs-alfred/service/content"
)

// Goods 定义商品配置结构.
type Goods struct {
	Tag    string          `json:"tag"    yaml:"tag"`
	Type   string          `json:"type"   yaml:"type"`
	Des    string          `json:"des"    yaml:"des,omitempty"`
	Using  Item            `json:"using"  yaml:"using"`
	Topics []content.Topic `json:"topics" yaml:"topics,omitempty"`
	Item   []yaml.MapSlice `json:"item"   yaml:"item"`
	// Record []string        `json:"record" yaml:"record,omitempty"`
	Score int `json:"score" yaml:"score"`
}

// Item 定义单个商品项.
type Item struct {
	Name     string `json:"name"     yaml:"name"`
	Param    string `json:"param"    yaml:"param,omitempty"`
	Price    string `json:"price"    yaml:"price,omitempty"`
	Date     string `json:"date"     yaml:"date,omitempty"`
	EndDate  string `json:"endDate"  yaml:"endDate,omitempty"`
	EndPrice string `json:"endPrice" yaml:"endPrice,omitempty"`
	Des      string `json:"des"      yaml:"des,omitempty"`
	URL      string `json:"url"      yaml:"url,omitempty"`
	Use      bool   `json:"use"      yaml:"use,omitempty"`
}
