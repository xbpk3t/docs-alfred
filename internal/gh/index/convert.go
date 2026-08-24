package ghindex

import (
	"github.com/xbpk3t/docs-alfred/internal/gh/model/gh"
	"github.com/xbpk3t/docs-alfred/pkg/urlutil"
)

// ToRepos converts ConfigRepos to flat Repos list.
func (cr ConfigRepos) ToRepos() Repos {
	var repos Repos

	for _, config := range cr {
		// repos now live on topics only; there is no type-level repo list.
		for i := range config.Topics {
			repos = append(repos, processTopicRepos(&config.Topics[i], config.Tag, config.Type)...)
		}
	}

	return repos
}

// processTopicRepos processes repos inside a topic.
func processTopicRepos(topic *gh.Topic, tag, typeName string) Repos {
	var repos Repos

	for i := range topic.Repo {
		// topic.repo entries are pure data-model repos; enrich them with provenance.
		repo := &Repo{Repo: topic.Repo[i], Tag: tag, Type: typeName, TopicName: topic.Topic}
		repos = append(repos, processRepo(repo, typeName)...)
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
		}
		repos = append(repos, processRepo(rel, repo.Type)...)
	}

	return repos
}

func isValidSourceRepoURL(url string) bool {
	return urlutil.IsSourceRepo(url)
}
