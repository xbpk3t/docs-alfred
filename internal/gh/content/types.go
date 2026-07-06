package content

import (
	"encoding/json"

	yaml "github.com/goccy/go-yaml"
)

// Repo defines a repository entry in data/gh YAML.
type Repo struct {
	IsDotfiles     *bool    `json:"isDotfiles,omitempty" yaml:"isDotfiles,omitempty"`
	URL            string   `json:"url"              yaml:"url"`
	Des            string   `json:"des,omitempty"    yaml:"des,omitempty"`
	Zk             string   `json:"zk,omitempty"     yaml:"zk,omitempty"`
	NixURL         string   `json:"nix,omitempty"    yaml:"nix,omitempty"`
	Doc            string   `json:"doc,omitempty"    yaml:"doc,omitempty"`
	Tag            string   `json:"tag,omitempty"    yaml:"tag,omitempty"`
	Type           string   `json:"type,omitempty"   yaml:"type,omitempty"`
	TopicName      string   `json:"-"                yaml:"-"`
	MainRepo       string   `json:"-"                yaml:"-"`
	Topics         Topics   `json:"topics,omitempty" yaml:"topics,omitempty"`
	RelatedRepos   Repos    `json:"rel,omitempty"    yaml:"rel,omitempty"`
	Cmd            []string `json:"cmd,omitempty"   yaml:"cmd,omitempty"`
	Record         []Record `json:"record,omitempty" yaml:"record,omitempty"`
	Score          int      `json:"score,omitempty"  yaml:"score,omitempty"`
	IsRelatedRepo  bool     `json:"-"                yaml:"-"`
	HasRecord      bool     `json:"-"                yaml:"-"`
	RecordValid    bool     `json:"-"                yaml:"-"`
}

type Repos []*Repo

// Record is a dated note attached to a repo or topic.
type Record struct {
	Date string `json:"date" yaml:"date"`
	Des  string `json:"des,omitempty" yaml:"des,omitempty"`
}

// Topic defines a reusable content topic structure shared by multiple domains.
type Topic struct {
	Meta        *TopicMeta      `json:"-"                yaml:"meta,omitempty"`
	PicDir      string          `json:"picDir,omitempty" yaml:"picDir,omitempty"`
	Des         string          `json:"des,omitempty"    yaml:"des,omitempty"`
	Topic       string          `json:"topic"            yaml:"topic"`
	URLs        string          `json:"url,omitempty"    yaml:"url,omitempty"`
	What        []string        `json:"what,omitempty"   yaml:"what,omitempty"`
	HTO         []string        `json:"hto,omitempty"    yaml:"hto,omitempty"`
	Repos       Repos           `json:"repos,omitempty"  yaml:"repo,omitempty"`
	Qs          []string        `json:"qs,omitempty"     yaml:"qs,omitempty"`
	Why         []string        `json:"why,omitempty"    yaml:"why,omitempty"`
	Sub         Topics          `json:"sub,omitempty"    yaml:"sub,omitempty"`
	WW          []string        `json:"ww,omitempty"     yaml:"ww,omitempty"`
	HTU         []string        `json:"htu,omitempty"    yaml:"htu,omitempty"`
	HTI         []string        `json:"hti,omitempty"    yaml:"hti,omitempty"`
	Pictures    []string        `json:"pic,omitempty"    yaml:"pic,omitempty"`
	Table       []yaml.MapSlice `json:"table,omitempty"  yaml:"table,omitempty"`
	Tables      Tables          `json:"tables,omitempty" yaml:"tables,omitempty"`
	Record      []Record        `json:"record,omitempty" yaml:"record,omitempty"`
	IsX         bool            `json:"isX,omitempty"    yaml:"isX,omitempty"`
	HasPic      bool            `json:"hasPic,omitempty" yaml:"hasPic,omitempty"`
	HasRecord   bool            `json:"-"                yaml:"-"`
	RecordValid bool            `json:"-"                yaml:"-"`
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

// DirName returns the directory name implied by a topic.
func (t *Topic) DirName() string {
	if t.Meta != nil && t.Meta.Slug != "" {
		return t.Meta.Slug
	}

	return t.Topic
}

// HasPicture reports whether a topic expects an image directory.
func (t *Topic) HasPicture() bool {
	return (t.Meta != nil && t.Meta.HasPic) || t.HasPic
}
