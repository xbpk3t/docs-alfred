package movie

import (
	"github.com/xbpk3t/docs-alfred/pkg/parser"
	"github.com/xbpk3t/docs-alfred/service/gh"
)

// Movie 电影/剧集条目.
type Movie struct {
	URL       string    `json:"url,omitempty"       yaml:"url,omitempty"`
	Alias     string    `json:"alias,omitempty"     yaml:"alias,omitempty"`
	Author    string    `json:"author,omitempty"    yaml:"author,omitempty"`
	Name      string    `json:"name"                yaml:"name"`
	ReadAt    string    `json:"readAt,omitempty"    yaml:"readAt,omitempty"`
	PublishAt string    `json:"publishAt,omitempty" yaml:"publishAt,omitempty"`
	Des       string    `json:"des,omitempty"       yaml:"des,omitempty"`
	Date      string    `json:"date,omitempty"      yaml:"date,omitempty"`
	Tags      []string  `json:"tags,omitempty"      yaml:"tags,omitempty"`
	Topics    gh.Topics `json:"topics,omitempty"    yaml:"topics,omitempty"`
	Record    []Record  `json:"record,omitempty"    yaml:"record,omitempty"`
	Sub       []Movie   `json:"sub,omitempty"       yaml:"sub,omitempty"`
	Score     int       `json:"score,omitempty"     yaml:"score,omitempty"`
}

// Record 观看记录.
type Record struct {
	Date string `json:"date,omitempty" yaml:"date,omitempty"`
	Des  string `json:"des,omitempty"  yaml:"des,omitempty"`
}

// ParseConfig 解析影视配置文件.
func ParseConfig(data []byte) ([]Movie, error) {
	return parser.NewParser[Movie](data).ParseFlatten()
}
