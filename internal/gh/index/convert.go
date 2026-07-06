package ghindex

import (
	"github.com/xbpk3t/docs-alfred/internal/gh/content"
	"github.com/xbpk3t/docs-alfred/pkg/urlutil"
)

// ToRepos converts ConfigRepos to flat Repos list.
func (cr ConfigRepos) ToRepos() Repos {
	var repos Repos

	for _, config := range cr {
		// Propagate type-level isDotfiles to repos, but only when explicitly set.
		// nil means "not set" so we skip propagation, avoiding zero-value false
		// overriding repo-level values.
		config.Using.Tag = config.Tag
		if config.IsDotfiles != nil {
			config.Using.IsDotfiles = config.IsDotfiles
		}
		repos = append(repos, processRepo(&config.Using, config.Type)...)

		for i := range config.Repos {
			config.Repos[i].Tag = config.Tag
			if config.IsDotfiles != nil {
				config.Repos[i].IsDotfiles = config.IsDotfiles
			}
			repos = append(repos, processRepo(config.Repos[i], config.Type)...)
		}

		// 处理 topic 内的 repos
		for i := range config.Topics {
			repos = append(repos, processTopicRepos(&config.Topics[i], config.Tag, config.Type)...)
		}
	}

	return repos
}

// processTopicRepos processes repos inside a topic.
func processTopicRepos(topic *content.Topic, tag, typeName string) Repos {
	var repos Repos

	for i := range topic.Repos {
		topic.Repos[i].Tag = tag
		topic.Repos[i].Type = typeName
		topic.Repos[i].TopicName = topic.Topic // 设置 topic 名称
		repos = append(repos, processRepo(topic.Repos[i], typeName)...)
	}

	// 递归处理子 topics
	for i := range topic.Sub {
		repos = append(repos, processTopicRepos(&topic.Sub[i], tag, typeName)...)
	}

	return repos
}

// processRepo processes a repository and its sub-repos.
func processRepo(repo *Repository, configType string) Repos {
	var repos Repos
	if mainRepo := processMainRepo(repo, configType); mainRepo != nil {
		repos = append(repos, mainRepo)
	}
	repos = append(repos, processAllSubRepos(repo)...)

	return repos
}

func processMainRepo(repo *Repository, configType string) *Repository {
	if !isValidSourceRepoURL(repo.URL) {
		return nil
	}
	repo.Type = configType

	return repo
}

func processAllSubRepos(repo *Repository) Repos {
	var repos Repos

	for i := range repo.SubRepos {
		repo.SubRepos[i].IsSubRepo = true
		repo.SubRepos[i].Type = repo.Type
		repo.SubRepos[i].Tag = repo.Tag
		repo.SubRepos[i].MainRepo = FullName(repo)
		repos = append(repos, processRepo(repo.SubRepos[i], repo.Type)...)
	}

	for i := range repo.ReplacedRepos {
		repo.ReplacedRepos[i].IsReplacedRepo = true
		repo.ReplacedRepos[i].Type = repo.Type
		repo.ReplacedRepos[i].Tag = repo.Tag
		repo.ReplacedRepos[i].MainRepo = FullName(repo)
		repos = append(repos, processRepo(repo.ReplacedRepos[i], repo.Type)...)
	}

	for i := range repo.RelatedRepos {
		repo.RelatedRepos[i].IsRelatedRepo = true
		repo.RelatedRepos[i].Type = repo.Type
		repo.RelatedRepos[i].Tag = repo.Tag
		repo.RelatedRepos[i].MainRepo = FullName(repo)
		repos = append(repos, processRepo(repo.RelatedRepos[i], repo.Type)...)
	}

	return repos
}

func isValidSourceRepoURL(url string) bool {
	return urlutil.IsSourceRepo(url)
}
