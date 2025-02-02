package books

import "github.com/xbpk3t/docs-alfred/pkg/parser"

type Books struct {
	Year      string
	BookTypes []BookType
}

type BookType struct {
	Tag   string `yaml:"tag"`
	Type  string `yaml:"type,omitempty"`
	Books []struct {
		Name   string     `yaml:"name"`
		Author string     `yaml:"author,omitempty"`
		URL    string     `yaml:"url,omitempty"`
		Des    string     `yaml:"des,omitempty"`
		Date   []struct{} `yaml:"date,omitempty"`
		Score  int        `yaml:"score,omitempty"`
	} `yaml:"books,omitempty"`
}

// ParseConfig 解析配置文件
func ParseConfig(data []byte) ([]Books, error) {
	return parser.NewParser[Books](data).ParseFlatten()
}
