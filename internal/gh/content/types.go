package content

import (
	"encoding/json"
)

// Repo defines a repository entry in data/gh YAML.
type Repo struct {
	IsDotfiles    *bool  `json:"isDotfiles,omitempty" yaml:"isDotfiles,omitempty"`
	URL           string `json:"url"              yaml:"url"`
	Des           string `json:"des,omitempty"    yaml:"des,omitempty"`
	NixURL        string `json:"nix,omitempty"    yaml:"nix,omitempty"`
	Doc           string `json:"doc,omitempty"    yaml:"doc,omitempty"`
	Tag           string `json:"tag,omitempty"    yaml:"tag,omitempty"`
	Type          string `json:"type,omitempty"   yaml:"type,omitempty"`
	TopicName     string `json:"-"                yaml:"-"`
	MainRepo      string `json:"-"                yaml:"-"`
	RelatedRepos  Repos  `json:"rel,omitempty"    yaml:"rel,omitempty"`
	Score         int    `json:"score,omitempty"  yaml:"score,omitempty"`
	IsRelatedRepo bool   `json:"-"                yaml:"-"`
}

type Repos []*Repo

// Topic defines a reusable content topic structure shared by multiple domains.
type Topic struct {
	Topic string                   `json:"topic"           yaml:"topic"`
	What  []string                 `json:"what,omitempty"  yaml:"what,omitempty"`
	HTO   []string                 `json:"hto,omitempty"   yaml:"hto,omitempty"`
	Repos Repos                    `json:"repos,omitempty" yaml:"repo,omitempty"`
	Table []map[string]interface{} `json:"table,omitempty" yaml:"table,omitempty"`
	Qs    []string                 `json:"qs,omitempty"    yaml:"qs,omitempty"`
	Why   []string                 `json:"why,omitempty"   yaml:"why,omitempty"`
	WW    []string                 `json:"ww,omitempty"    yaml:"ww,omitempty"`
	HTU   []string                 `json:"htu,omitempty"   yaml:"htu,omitempty"`
	HTI   []string                 `json:"hti,omitempty"   yaml:"hti,omitempty"`
	Score int                      `json:"score,omitempty" yaml:"score,omitempty"`
}

type Topics []Topic

func (t *Topic) MarshalJSON() ([]byte, error) {
	type topic Topic

	return json.Marshal((*topic)(t))
}

// DirName returns the directory name implied by a topic.
func (t *Topic) DirName() string {
	return t.Topic
}
