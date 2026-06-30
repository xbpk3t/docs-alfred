package ghindex

import (
	"strings"

	"github.com/xbpk3t/docs-alfred/internal/gh/content"
	"github.com/xbpk3t/docs-alfred/pkg/urlutil"
)

const GhURL = "https://github.com/"

// Repository defines repository structure (used by both render pipeline and remote search).
type Repository struct {
	MainRepo       string         `yaml:"-"`
	Des            string         `yaml:"des,omitempty"`
	URL            string         `yaml:"url"`
	NixURL         string         `yaml:"nix,omitempty"`
	Tag            string         `yaml:"tag,omitempty"`
	Type           string         `yaml:"type"`
	Doc            string         `yaml:"doc,omitempty"`
	Topics         content.Topics `json:"topics,omitempty" yaml:"topics,omitempty"`
	SubRepos       Repos          `yaml:"sub,omitempty"`
	ReplacedRepos  Repos          `yaml:"rep,omitempty"`
	RelatedRepos   Repos          `yaml:"rel,omitempty"`
	Cmd            []string       `yaml:"cmd,omitempty"`
	Score          int            `yaml:"score,omitempty"`
	IsDotfiles     bool           `yaml:"isDotfiles,omitempty"`
	IsSubRepo      bool           `yaml:"-"`
	IsReplacedRepo bool           `yaml:"-"`
	IsRelatedRepo  bool           `yaml:"-"`
}

type Repos []*Repository

// ConfigRepo defines configuration repository structure.
type ConfigRepo struct {
	Type       string         `yaml:"type"`
	Tag        string         `yaml:"tag"`
	Repos      Repos          `yaml:"repo"`
	Topics     content.Topics `json:"topics,omitempty" yaml:"topics,omitempty"`
	Using      Repository     `yaml:"using,omitempty"`
	Score      int            `yaml:"score,omitempty"`
	IsDotfiles bool           `yaml:"isDotfiles,omitempty"`
}

type ConfigRepos []*ConfigRepo

// Config represents the complete gh configuration (for remote gh.yml).
type Config struct {
	ConfigRepos ConfigRepos `yaml:"config"`
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

func (r *Repository) HasNix() bool {
	return strings.TrimSpace(r.NixURL) != ""
}

func (r *Repository) HasSubRepos() bool {
	return len(r.SubRepos) > 0 || len(r.ReplacedRepos) > 0 || len(r.RelatedRepos) > 0
}

func (r *Repository) IsSubOrDepOrRelRepo() bool {
	return r.IsSubRepo || r.IsReplacedRepo || r.IsRelatedRepo
}
