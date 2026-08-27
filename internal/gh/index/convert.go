package ghindex

import (
	"github.com/xbpk3t/docs-alfred/internal/gh/model/gh"
	"github.com/xbpk3t/docs-alfred/pkg/urlutil"
)

// ToRepos converts ConfigRepos to flat Repos list.
func (cr ConfigRepos) ToRepos() Repos {
	var repos Repos

	for _, config := range cr {
		// repos now live on topics only; there's no type-level repo list.
		for i := range config.Topics {
			repos = append(repos, processTopicRepos(&config.Topics[i], config)...)
		}
	}

	return repos
}

// processTopicRepos processes repos inside a topic, carrying provenance
// (tag/type/source file) onto each flattened Repo.
func processTopicRepos(topic *gh.Topic, config *ConfigRepo) Repos {
	var repos Repos

	for i := range topic.Repo {
		// topic.repo entries are pure data-model repos; enrich them with provenance.
		repo := &Repo{Repo: topic.Repo[i], Tag: config.Tag, Type: config.Type, TopicName: topic.Topic, File: config.File}
		repos = append(repos, processRepo(repo, config.Type)...)
	}

	return repos
}

// processRepo processes a repository and its sub-repos.
func processRepo(repo *Repo, configType string) Repos {
	var repos Repos
	if mainRepo := processMainRepo(repo, configType); mainRepo != nil {
		repos = append(repos, mainRepo)
	}
	repos = append(repos, processAllSubRepos(repo)...)

	return repos
}

func processMainRepo(repo *Repo, configType string) *Repo {
	if !isValidSourceRepoURL(repo.URL) {
		return nil
	}
	repo.Type = configType

	return repo
}

func processAllSubRepos(repo *Repo) Repos {
	var repos Repos

	for i := range repo.Rel {
		rel := &Repo{
			Repo:          repo.Rel[i],
			IsRelatedRepo: true,
			Type:          repo.Type,
			Tag:           repo.Tag,
			MainRepo:      FullName(repo),
			File:          repo.File,
		}
		repos = append(repos, processRepo(rel, repo.Type)...)
	}

	return repos
}

func isValidSourceRepoURL(url string) bool {
	return urlutil.IsSourceRepo(url)
}
