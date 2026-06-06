package gh

import (
	yaml "github.com/goccy/go-yaml"
	"github.com/xbpk3t/docs-alfred/pkg/urlutil"
)

const GhURL = "https://github.com/"

// Repository defines repository structure (used by both render pipeline and remote search).
type Repository struct {
	Doc            string   `yaml:"doc,omitempty"`
	Des            string   `yaml:"des,omitempty"`
	URL            string   `yaml:"url"`
	Tag            string   `yaml:"tag,omitempty"`
	Type           string   `yaml:"type"`
	MainRepo       string   `yaml:"-"` // If it's a sub/replaced/related repo
	Topics         Topics   `json:"topics,omitempty" yaml:"topics,omitempty"`
	SubRepos       Repos    `yaml:"sub,omitempty"`
	ReplacedRepos  Repos    `yaml:"rep,omitempty"`
	RelatedRepos   Repos    `yaml:"rel,omitempty"`
	Cmd            []string `yaml:"cmd,omitempty"`
	IsSubRepo      bool     `yaml:"-"`
	IsReplacedRepo bool     `yaml:"-"`
	IsRelatedRepo  bool     `yaml:"-"`
	Score          int      `yaml:"score,omitempty"`
}

type Repos []*Repository

// ConfigRepo defines configuration repository structure.
type ConfigRepo struct {
	Type   string     `yaml:"type"`
	Tag    string     `yaml:"tag"`
	Repos  Repos      `yaml:"repo"`
	Topics Topics     `json:"topics,omitempty" yaml:"topics,omitempty"`
	Using  Repository `yaml:"using,omitempty"`
	Score  int        `yaml:"score,omitempty"`
}

type ConfigRepos []*ConfigRepo

// Config represents the complete gh configuration (for remote gh.yml).
type Config struct {
	ConfigRepos ConfigRepos `yaml:"config"`
}

// Topic defines topic structure.
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
	return yaml.Marshal(t)
}

func (r *Repository) IsValid() bool {
	_, ok := urlutil.GitHubOwnerRepo(r.URL)

	return ok
}

func (r *Repository) FullName() string {
	repo, ok := urlutil.GitHubOwnerRepo(r.URL)
	if !ok {
		return ""
	}

	return repo.Owner + "/" + repo.Name
}

func (r *Repository) GetDes() string {
	return r.Des
}

func (r *Repository) GetURL() string {
	return r.URL
}

func (r *Repository) HasQs() bool {
	return len(r.Topics) > 0
}

func (r *Repository) HasSubRepos() bool {
	return len(r.SubRepos) > 0 || len(r.ReplacedRepos) > 0 || len(r.RelatedRepos) > 0
}

func (r *Repository) IsSubOrDepOrRelRepo() bool {
	return r.IsSubRepo || r.IsReplacedRepo || r.IsRelatedRepo
}
