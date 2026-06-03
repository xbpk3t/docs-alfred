package gh

// ToRepos converts ConfigRepos to flat Repos list.
func (cr ConfigRepos) ToRepos() Repos {
	var repos Repos

	for _, config := range cr {
		config.Using.Tag = config.Tag
		repos = append(repos, processRepo(&config.Using, config.Type)...)

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
	if mainRepo := processMainRepo(repo, configType); mainRepo != nil {
		repos = append(repos, mainRepo)
	}
	repos = append(repos, processAllSubRepos(repo)...)

	return repos
}

func processMainRepo(repo *Repository, configType string) *Repository {
	if !isValidGithubURL(repo.URL) {
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
		repo.SubRepos[i].MainRepo = repo.FullName()
		repos = append(repos, processRepo(repo.SubRepos[i], repo.Type)...)
	}

	for i := range repo.ReplacedRepos {
		repo.ReplacedRepos[i].IsReplacedRepo = true
		repo.ReplacedRepos[i].Type = repo.Type
		repo.ReplacedRepos[i].Tag = repo.Tag
		repo.ReplacedRepos[i].MainRepo = repo.FullName()
		repos = append(repos, processRepo(repo.ReplacedRepos[i], repo.Type)...)
	}

	for i := range repo.RelatedRepos {
		repo.RelatedRepos[i].IsRelatedRepo = true
		repo.RelatedRepos[i].Type = repo.Type
		repo.RelatedRepos[i].Tag = repo.Tag
		repo.RelatedRepos[i].MainRepo = repo.FullName()
		repos = append(repos, processRepo(repo.RelatedRepos[i], repo.Type)...)
	}

	return repos
}

func isValidGithubURL(url string) bool {
	return len(url) > len(GhURL) && url[:len(GhURL)] == GhURL
}
