package ghindex

import (
	"strings"

	"github.com/xbpk3t/docs-alfred/internal/gh/content"
	"github.com/xbpk3t/docs-alfred/pkg/urlutil"
)

const GhURL = "https://github.com/"

// Repository is an alias for content.Repo.
type Repository = content.Repo

// Repos is an alias for content.Repos.
type Repos = content.Repos

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

func IsValid(repo *content.Repo) bool {
	_, ok := urlutil.GitHubOwnerRepo(repo.URL)

	return ok
}

func FullName(repo *content.Repo) string {
	r, ok := urlutil.GitHubOwnerRepo(repo.URL)
	if !ok {
		return ""
	}

	return r.Owner + "/" + r.Name
}

func GetDes(repo *content.Repo) string {
	return repo.Des
}

func GetURL(repo *content.Repo) string {
	return repo.URL
}

func HasQs(repo *content.Repo) bool {
	return len(repo.Topics) > 0
}

func HasNix(repo *content.Repo) bool {
	return strings.TrimSpace(repo.NixURL) != ""
}

func HasSubRepos(repo *content.Repo) bool {
	return len(repo.SubRepos) > 0 || len(repo.ReplacedRepos) > 0 || len(repo.RelatedRepos) > 0
}

func IsSubOrDepOrRelRepo(repo *content.Repo) bool {
	return repo.IsSubRepo || repo.IsReplacedRepo || repo.IsRelatedRepo
}
