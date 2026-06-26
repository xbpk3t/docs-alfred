package internal

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/go-github/v70/github"
)

// GitHubReviewInput is the template data for GitHub issue review prompt.
type GitHubReviewInput struct {
	Lang   string
	Issues []GitHubReviewIssue
}

// GitHubReviewIssue is a single GitHub issue for AI review.
type GitHubReviewIssue struct {
	Identifier      string
	Title           string
	StateName       string
	TeamName        string
	Description     string
	LinearReference string // Linear issue identifier (e.g. "ENG-123"), extracted from issue body
	Comments        []GitHubReviewComment
}

// GitHubReviewComment is a single comment on a GitHub issue.
type GitHubReviewComment struct {
	UserName  string
	CreatedAt string
	Body      string
}

// GitHubClient wraps the go-github client for issue operations.
type GitHubClient struct {
	client *github.Client
	owner  string
	repo   string
}

// NewGitHubClient creates a GitHub client authenticated with the given token.
func NewGitHubClient(token, owner, repo string) *GitHubClient {
	return &GitHubClient{
		client: github.NewClient(nil).WithAuthToken(token),
		owner:  owner,
		repo:   repo,
	}
}

// GetIssueDetail fetches a GitHub issue with its comments.
func (g *GitHubClient) GetIssueDetail(ctx context.Context, number int) (*GitHubReviewIssue, error) {
	issue, _, err := g.client.Issues.Get(ctx, g.owner, g.repo, number)
	if err != nil {
		return nil, fmt.Errorf("get issue #%d: %w", number, err)
	}

	if issue.IsPullRequest() {
		return nil, fmt.Errorf("#%d is a pull request, not an issue", number)
	}

	comments, err := g.listAllComments(ctx, number)
	if err != nil {
		return nil, fmt.Errorf("list comments for #%d: %w", number, err)
	}

	teamName := g.owner + "/" + g.repo
	if len(issue.Labels) > 0 {
		teamName = issue.Labels[0].GetName()
	}

	detail := &GitHubReviewIssue{
		Identifier:  fmt.Sprintf("#%d", number),
		Title:       issue.GetTitle(),
		Description: issue.GetBody(),
		StateName:   issue.GetState(),
		TeamName:    teamName,
		Comments:    comments,
	}

	slog.Info("fetched GitHub issue", "identifier", detail.Identifier, "comments", len(comments))

	return detail, nil
}

// PostReviewComment posts a review comment on the given issue.
func (g *GitHubClient) PostReviewComment(ctx context.Context, number int, body string) error {
	_, _, err := g.client.Issues.CreateComment(ctx, g.owner, g.repo, number, &github.IssueComment{
		Body: github.Ptr(body),
	})
	if err != nil {
		return fmt.Errorf("post comment on #%d: %w", number, err)
	}

	slog.Info("posted review comment", "issue", fmt.Sprintf("#%d", number))

	return nil
}

// listAllComments fetches all comments for an issue, handling pagination.
func (g *GitHubClient) listAllComments(ctx context.Context, number int) ([]GitHubReviewComment, error) {
	var all []GitHubReviewComment
	opts := &github.IssueListCommentsOptions{ListOptions: github.ListOptions{PerPage: 100}}

	for {
		comments, resp, err := g.client.Issues.ListComments(ctx, g.owner, g.repo, number, opts)
		if err != nil {
			return nil, err
		}

		for _, c := range comments {
			all = append(all, GitHubReviewComment{
				Body:      c.GetBody(),
				UserName:  c.GetUser().GetLogin(),
				CreatedAt: c.GetCreatedAt().Format(time.RFC3339),
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return all, nil
}
