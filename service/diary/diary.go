package diary

import (
	"github.com/xbpk3t/docs-alfred/pkg/parser"
)

// Entry 日记条目.
type Entry struct {
	Date   string `json:"date,omitempty"   yaml:"date,omitempty"`
	Des    string `json:"des,omitempty"    yaml:"des,omitempty"`
	Review string `json:"review,omitempty" yaml:"review,omitempty"`
	URL    string `json:"url,omitempty"    yaml:"url,omitempty"`
	Score  int    `json:"score,omitempty"  yaml:"score,omitempty"`
	Week   int    `json:"week,omitempty"   yaml:"week,omitempty"`
}

// ParseConfig 解析日记配置文件.
func ParseConfig(data []byte) ([]Entry, error) {
	return parser.NewParser[Entry](data).ParseFlatten()
}
