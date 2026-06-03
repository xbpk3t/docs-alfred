package ntl

import (
	"github.com/xbpk3t/docs-alfred/pkg/parser"
	"github.com/xbpk3t/docs-alfred/service/gh"
)

// Jav 作品条目.
type Jav struct {
	Name      string    `json:"name,omitempty"      yaml:"name,omitempty"`
	URL       string    `json:"url,omitempty"       yaml:"url,omitempty"`
	Cast      string    `json:"cast,omitempty"      yaml:"cast,omitempty"`
	Des       string    `json:"des,omitempty"       yaml:"des,omitempty"`
	PublishAt string    `json:"publishAt,omitempty" yaml:"publishAt,omitempty"`
	Label     string    `json:"label,omitempty"     yaml:"label,omitempty"`
	Tags      []string  `json:"tags,omitempty"      yaml:"tags,omitempty"`
	Topics    gh.Topics `json:"topics,omitempty"    yaml:"topics,omitempty"`
	Sub       []Jav     `json:"sub,omitempty"       yaml:"sub,omitempty"`
	Record    []Record  `json:"record,omitempty"    yaml:"record,omitempty"`
	Score     int       `json:"score,omitempty"     yaml:"score,omitempty"`
}

// VG 游戏条目.
type VG struct {
	Genre     string    `json:"genre,omitempty"     yaml:"genre,omitempty"`
	Status    string    `json:"status,omitempty"    yaml:"status,omitempty"`
	Developer string    `json:"developer,omitempty" yaml:"developer,omitempty"`
	Price     string    `json:"price,omitempty"     yaml:"price,omitempty"`
	Des       string    `json:"des,omitempty"       yaml:"des,omitempty"`
	PlayAt    string    `json:"playAt,omitempty"    yaml:"playAt,omitempty"`
	Alias     string    `json:"alias,omitempty"     yaml:"alias,omitempty"`
	URL       string    `json:"url,omitempty"       yaml:"url,omitempty"`
	Name      string    `json:"name,omitempty"      yaml:"name,omitempty"`
	PublishAt string    `json:"publishAt,omitempty" yaml:"publishAt,omitempty"`
	Platform  string    `json:"platform,omitempty"  yaml:"platform,omitempty"`
	Tags      []string  `json:"tags,omitempty"      yaml:"tags,omitempty"`
	Topics    gh.Topics `json:"topics,omitempty"    yaml:"topics,omitempty"`
	Sub       []VG      `json:"sub,omitempty"       yaml:"sub,omitempty"`
	Record    []Record  `json:"record,omitempty"    yaml:"record,omitempty"`
	Score     int       `json:"score,omitempty"     yaml:"score,omitempty"`
}

// Record 通用记录条目.
type Record struct {
	Date string `json:"date,omitempty" yaml:"date,omitempty"`
	Des  string `json:"des,omitempty"  yaml:"des,omitempty"`
}

// ParseJavConfig 解析 Jav 配置文件.
func ParseJavConfig(data []byte) ([]Jav, error) {
	return parser.NewParser[Jav](data).ParseFlatten()
}

// ParseVGConfig 解析 VG 配置文件.
func ParseVGConfig(data []byte) ([]VG, error) {
	return parser.NewParser[VG](data).ParseFlatten()
}
