package content

import (
	"encoding/json"

	yaml "github.com/goccy/go-yaml"
)

// Topic defines a reusable content topic structure shared by multiple domains.
type Topic struct {
	Topic    string          `json:"topic"            yaml:"topic"`
	Des      string          `json:"des,omitempty"    yaml:"des,omitempty"`
	Meta     *TopicMeta      `json:"-"                yaml:"meta,omitempty"`
	Sub      Topics          `json:"sub,omitempty"    yaml:"sub,omitempty"`
	PicDir   string          `json:"picDir,omitempty" yaml:"picDir,omitempty"`
	Pictures []string        `json:"pic,omitempty"    yaml:"pic,omitempty"`
	URLs     string          `json:"url,omitempty"    yaml:"url,omitempty"`
	Qs       []string        `json:"qs,omitempty"     yaml:"qs,omitempty"`
	Why      []string        `json:"why,omitempty"    yaml:"why,omitempty"`
	What     []string        `json:"what,omitempty"   yaml:"what,omitempty"`
	WW       []string        `json:"ww,omitempty"     yaml:"ww,omitempty"`
	HTU      []string        `json:"htu,omitempty"    yaml:"htu,omitempty"`
	HTI      []string        `json:"hti,omitempty"    yaml:"hti,omitempty"`
	HTO      []string        `json:"hto,omitempty"    yaml:"hto,omitempty"`
	Table    []yaml.MapSlice `json:"table,omitempty"  yaml:"table,omitempty"`
	Tables   Tables          `json:"tables,omitempty" yaml:"tables,omitempty"`
	HasPic   bool            `json:"hasPic,omitempty" yaml:"hasPic,omitempty"`
	IsX      bool            `json:"isX,omitempty"    yaml:"isX,omitempty"`
}

type Topics []Topic

type TopicMeta struct {
	Slug   string `json:"slug,omitempty"   yaml:"slug,omitempty"`
	HasPic bool   `json:"hasPic,omitempty" yaml:"hasPic,omitempty"`
	IsX    bool   `json:"isX,omitempty"    yaml:"isX,omitempty"`
}

type Table struct {
	Name  string          `json:"name,omitempty"  yaml:"name,omitempty"`
	URL   string          `json:"url,omitempty"   yaml:"url,omitempty"`
	Table []yaml.MapSlice `json:"table,omitempty" yaml:"table,omitempty"`
}

type Tables []Table

func (t *Topic) MarshalJSON() ([]byte, error) {
	type topic Topic

	return json.Marshal((*topic)(t))
}
