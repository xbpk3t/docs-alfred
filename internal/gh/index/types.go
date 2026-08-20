package ghindex

import (
	"strings"

	"github.com/xbpk3t/docs-alfred/internal/gh/model/gh"
	"github.com/xbpk3t/docs-alfred/pkg/urlutil"
)

const GhURL = "https://github.com/"

// Repo is the enriched repository type: the schema-generated data model
// (gh.Repo) plus runtime provenance fields set during indexing (which
// config/topic/rel a repo came from). Provenance is index-layer only; it is
// not part of the gh JSON Schema, so it lives outside the generated gh.
type Repo struct {
	gh.Repo       `yaml:",inline"`
	Tag           string `yaml:"tag,omitempty"      json:"tag,omitempty"`
	Type          string `yaml:"type,omitempty"     json:"type,omitempty"`
	TopicName     string `yaml:"-"                  json:"-"`
	MainRepo      string `yaml:"-"                  json:"-"`
	IsRelatedRepo bool   `yaml:"-"                  json:"-"`
}

// Repos is a list of enriched repositories.
type Repos []*Repo

// Topics is the schema-generated topic list. Topics carry no provenance (the
// index adds none today), so they are used directly from the generated gh.
// The alias keeps the composite-literal shape (`Topics{{...}}`) used by callers.
type Topics = []gh.Topic

// ConfigRepo defines configuration repository structure.
type ConfigRepo struct {
	IsDotfiles *bool  `yaml:"isDotfiles,omitempty"`
	Type       string `yaml:"type"`
	Tag        string `yaml:"tag"`
	Repos      Repos  `yaml:"repo"`
	Topics     Topics `json:"topics,omitempty" yaml:"topics,omitempty"`
}

type ConfigRepos []*ConfigRepo

// Config represents the complete gh configuration (for remote gh.yml).
type Config struct {
	ConfigRepos ConfigRepos `yaml:"config"`
}

func FullName(repo *Repo) string {
	if repo == nil {
		return ""
	}
	r, ok := urlutil.GitHubOwnerRepo(repo.URL)
	if !ok {
		return ""
	}

	return r.Owner + "/" + r.Name
}

func GetDes(repo *Repo) string {
	if repo == nil || repo.Des == nil {
		return ""
	}

	return *repo.Des
}

func GetURL(repo *Repo) string {
	if repo == nil {
		return ""
	}

	return repo.URL
}

func HasNix(repo *Repo) bool {
	if repo == nil || repo.Nix == nil {
		return false
	}

	return strings.TrimSpace(*repo.Nix) != ""
}
