package books

import (
	"github.com/xbpk3t/docs-alfred/pkg/parser"
	"github.com/xbpk3t/docs-alfred/service/gh"
)

// Book 书籍条目.
type Book struct {
	URL       string    `json:"url,omitempty"       yaml:"url,omitempty"`
	Source    string    `json:"source,omitempty"    yaml:"source,omitempty"`
	Author    string    `json:"author,omitempty"    yaml:"author,omitempty"`
	Cast      string    `json:"cast,omitempty"      yaml:"cast,omitempty"`
	ReadAt    string    `json:"readAt,omitempty"    yaml:"readAt,omitempty"`
	ReadTime  string    `json:"readTime,omitempty"  yaml:"readTime,omitempty"`
	PublishAt string    `json:"publishAt,omitempty" yaml:"publishAt,omitempty"`
	Des       string    `json:"des,omitempty"       yaml:"des,omitempty"`
	Alias     string    `json:"alias,omitempty"     yaml:"alias,omitempty"`
	Name      string    `json:"name"                yaml:"name"`
	Dict      string    `json:"dict,omitempty"      yaml:"dict,omitempty"`
	Content   string    `json:"content,omitempty"   yaml:"content,omitempty"`
	Tags      []string  `json:"tags,omitempty"      yaml:"tags,omitempty"`
	Topics    gh.Topics `json:"topics,omitempty"    yaml:"topics,omitempty"`
	Record    []Record  `json:"record,omitempty"    yaml:"record,omitempty"`
	Sub       []Book    `json:"sub,omitempty"       yaml:"sub,omitempty"`
	Qs        []string  `json:"qs,omitempty"        yaml:"qs,omitempty"`
	Score     int       `json:"score,omitempty"     yaml:"score,omitempty"`
}

// Record 阅读记录.
type Record struct {
	Date string `json:"date,omitempty" yaml:"date,omitempty"`
	Des  string `json:"des,omitempty"  yaml:"des,omitempty"`
	URL  string `json:"url,omitempty"  yaml:"url,omitempty"`
}

// ParseConfig 解析书籍配置文件.
func ParseConfig(data []byte) ([]Book, error) {
	return parser.NewParser[Book](data).ParseFlatten()
}
