package gh

import (
	"context"

	"github.com/google/go-github/v56/github"
	"github.com/gregjones/httpcache"
)

type GithubClient struct {
	c *github.Client
}

func NewGithubClient(token string) *GithubClient {
	client := github.NewClient(httpcache.NewMemoryCacheTransport().Client()).WithAuthToken(token)

	return &GithubClient{c: client}
}

// github API
// func (client *GithubClient) ListStarredRepositories() ([]*github.Repository, error) {
// 	opt := &github.ActivityListStarredOptions{
// 		ListOptions: github.ListOptions{PerPage: 30},
// 		Sort:        "pushed",
// 	}
//
// 	var repos []*github.Repository
//
// 	for {
// 		result, resp, err := client.c.Activity.ListStarred(context.Background(), "", opt)
// 		if err != nil {
// 			return repos, err
// 		}
// 		for _, starred := range result {
// 			repos = append(repos, starred.Repository)
// 		}
// 		if resp.NextPage == 0 {
// 			break
// 		}
// 		opt.ListOptions.Page = resp.NextPage
// 	}
//
// 	return repos, nil
// }

func (client *GithubClient) ListUserRepositories() ([]*github.Repository, error) {
	opt := &github.RepositoryListOptions{
		ListOptions: github.ListOptions{PerPage: 30},
		Sort:        "pushed",
	}

	var repos []*github.Repository

	for {
		result, resp, err := client.c.Repositories.List(context.Background(), "", opt)
		if err != nil {
			return repos, err
		}
		repos = append(repos, result...)
		if resp.NextPage == 0 {
			break
		}
		opt.ListOptions.Page = resp.NextPage
	}

	return repos, nil
}

// [Search - GitHub Docs](https://docs.github.com/en/rest/search/search?apiVersion=2022-11-28#search-repositories)
// func (client *GithubClient) SearchRepositories(title string) ([]*github.Repository, error) {
// 	opt := &github.SearchOptions{
// 		ListOptions: github.ListOptions{PerPage: 10},
// 		Sort:        "stars",
// 	}
// 	var repos []*github.Repository
//
// 	result, _, err := client.c.Search.Repositories(context.Background(), title, opt)
// 	if err != nil {
// 		return repos, err
// 	}
// 	repos = append(repos, result.Repositories...)
// 	// if resp.NextPage == 0 {
// 	// 	break
// 	// }
// 	// opt.ListOptions.Page = resp.NextPage
// 	return repos, nil
// }

// 获取用户名
func (client *GithubClient) GetUsername() string {
	user := &github.User{}
	response, _, err := client.c.Users.Get(context.Background(), user.GetName())
	if err != nil {
		return ""
	}
	return response.GetLogin()
}
