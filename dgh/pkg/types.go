package gh

import (
	"strings"

	yaml "github.com/goccy/go-yaml"
)

const GhURL = "https://github.com/"

// Repository defines repository structure.
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

// Topic defines topic structure.
type Topic struct {
	Topic    string          `json:"topic"            yaml:"topic"`
	Des      string          `json:"des,omitempty"    yaml:"des,omitempty"`
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
	IsX      bool            `json:"isX,omitempty"    yaml:"isX,omitempty"`
}

type Topics []Topic

type Table struct {
	Name  string          `json:"name,omitempty"  yaml:"name,omitempty"`
	URL   string          `json:"url,omitempty"   yaml:"url,omitempty"`
	Table []yaml.MapSlice `json:"table,omitempty" yaml:"table,omitempty"`
}

type Tables []Table

// Config represents the complete dgh configuration.
type Config struct {
	ConfigRepos ConfigRepos `yaml:"config"`
}

// IsValid checks if repository has a valid GitHub URL.
func (r *Repository) IsValid() bool {
	return strings.Contains(r.URL, GhURL)
}

// FullName returns the repository full name (owner/repo).
func (r *Repository) FullName() string {
	if !r.IsValid() {
		return ""
	}
	if sx, found := strings.CutPrefix(r.URL, GhURL); found {
		return sx
	}

	return ""
}

// GetDes returns repository description.
func (r *Repository) GetDes() string {
	return r.Des
}

// GetURL returns repository URL.
func (r *Repository) GetURL() string {
	return r.URL
}

// HasQs checks if repository has topics.
func (r *Repository) HasQs() bool {
	return len(r.Topics) > 0
}

// HasSubRepos checks if repository has sub/replaced/related repos.
func (r *Repository) HasSubRepos() bool {
	return len(r.SubRepos) > 0 || len(r.ReplacedRepos) > 0 || len(r.RelatedRepos) > 0
}

// IsSubOrDepOrRelRepo checks if it's a sub/replaced/related repo.
func (r *Repository) IsSubOrDepOrRelRepo() bool {
	return r.IsSubRepo || r.IsReplacedRepo || r.IsRelatedRepo
}

// ToRepos converts ConfigRepos to flat Repos list.
func (cr ConfigRepos) ToRepos() Repos {
	var repos Repos

	for _, config := range cr {
		// Process using repo
		config.Using.Tag = config.Tag
		repos = append(repos, processRepo(&config.Using, config.Type)...)

		// Process all repos in this config
		for i := range config.Repos {
			config.Repos[i].Tag = config.Tag
			repos = append(repos, processRepo(config.Repos[i], config.Type)...)
		}
	}

	return repos
}

// processRepo processes a repository and its sub-repos.
func processRepo(repo *Repository, configType string) Repos {
	var repos Repos

	// Process main repo
	if mainRepo := processMainRepo(repo, configType); mainRepo != nil {
		repos = append(repos, mainRepo)
	}

	// Process all sub-repos
	repos = append(repos, processAllSubRepos(repo)...)

	return repos
}

// processMainRepo processes main repository.
func processMainRepo(repo *Repository, configType string) *Repository {
	if !isValidGithubURL(repo.URL) {
		return nil
	}
	repo.Type = configType

	return repo
}

// processAllSubRepos processes all types of sub-repos.
func processAllSubRepos(repo *Repository) Repos {
	var repos Repos

	// Process sub repos
	for i := range repo.SubRepos {
		repo.SubRepos[i].IsSubRepo = true
		repo.SubRepos[i].Type = repo.Type
		repo.SubRepos[i].Tag = repo.Tag
		repo.SubRepos[i].MainRepo = repo.FullName()
		repos = append(repos, processRepo(repo.SubRepos[i], repo.Type)...)
	}

	// Process replaced repos
	for i := range repo.ReplacedRepos {
		repo.ReplacedRepos[i].IsReplacedRepo = true
		repo.ReplacedRepos[i].Type = repo.Type
		repo.ReplacedRepos[i].Tag = repo.Tag
		repo.ReplacedRepos[i].MainRepo = repo.FullName()
		repos = append(repos, processRepo(repo.ReplacedRepos[i], repo.Type)...)
	}

	// Process related repos
	for i := range repo.RelatedRepos {
		repo.RelatedRepos[i].IsRelatedRepo = true
		repo.RelatedRepos[i].Type = repo.Type
		repo.RelatedRepos[i].Tag = repo.Tag
		repo.RelatedRepos[i].MainRepo = repo.FullName()
		repos = append(repos, processRepo(repo.RelatedRepos[i], repo.Type)...)
	}

	return repos
}

// isValidGithubURL checks if URL is a valid GitHub URL.
func isValidGithubURL(url string) bool {
	return strings.Contains(url, GhURL)
}

// Filter filters repositories by query string.
func (r Repos) Filter(query string) Repos {
	if query == "" {
		return r
	}

	query = strings.ToLower(query)
	var filtered Repos

	for _, repo := range r {
		// Search in URL, description, type, tag
		if strings.Contains(strings.ToLower(repo.URL), query) ||
			strings.Contains(strings.ToLower(repo.Des), query) ||
			strings.Contains(strings.ToLower(repo.Type), query) ||
			strings.Contains(strings.ToLower(repo.Tag), query) {
			filtered = append(filtered, repo)
		}
	}

	return filtered
}

// ExtractTags extracts unique tags from repositories.
func (r Repos) ExtractTags() []string {
	tagMap := make(map[string]struct{})
	for _, repo := range r {
		if repo.Tag != "" {
			tagMap[repo.Tag] = struct{}{}
		}
	}

	tags := make([]string, 0, len(tagMap))
	for tag := range tagMap {
		tags = append(tags, tag)
	}

	return tags
}

// ExtractTypesByTag returns all types for a given tag.
func (r Repos) ExtractTypesByTag(tag string) []string {
	types := make(map[string]bool)
	for _, repo := range r {
		if repo.Tag == tag {
			types[repo.Type] = true
		}
	}

	result := make([]string, 0, len(types))
	for t := range types {
		if t != "" {
			result = append(result, t)
		}
	}

	return result
}
