package music

import (
	"github.com/xbpk3t/docs-alfred/pkg/parser"
	"github.com/xbpk3t/docs-alfred/service/gh"
)

// Music 音乐条目.
type Music struct {
	Name      string    `json:"name,omitempty"      yaml:"name,omitempty"`
	Author    string    `json:"author,omitempty"    yaml:"author,omitempty"`
	PublishAt string    `json:"publishAt,omitempty" yaml:"publishAt,omitempty"`
	Des       string    `json:"des,omitempty"       yaml:"des,omitempty"`
	URL       string    `json:"url,omitempty"       yaml:"url,omitempty"`
	Perf      string    `json:"perf,omitempty"      yaml:"perf,omitempty"`
	Label     string    `json:"label,omitempty"     yaml:"label,omitempty"`
	Conductor string    `json:"conductor,omitempty" yaml:"conductor,omitempty"`
	Tags      []string  `json:"tags,omitempty"      yaml:"tags,omitempty"`
	Topics    gh.Topics `json:"topics,omitempty"    yaml:"topics,omitempty"`
	Record    []Record  `json:"record,omitempty"    yaml:"record,omitempty"`
	Score     int       `json:"score,omitempty"     yaml:"score,omitempty"`
}

// Record 听歌记录.
type Record struct {
	Date string `json:"date,omitempty" yaml:"date,omitempty"`
	Des  string `json:"des,omitempty"  yaml:"des,omitempty"`
}

// ParseConfig 解析音乐配置文件.
func ParseConfig(data []byte) ([]Music, error) {
	return parser.NewParser[Music](data).ParseFlatten()
}
