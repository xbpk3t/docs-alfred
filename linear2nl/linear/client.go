package linear

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Khan/genqlient/graphql"
)

// GraphQL field/argument keys used in Linear filters.
const (
	keyIn  = "in"
	keyKey = "key"
)

// Client communicates with the Linear GraphQL API.
type Client struct {
	http     *http.Client
	apiKey   string
	apiURL   string
	teamKeys []string
}

// NewClient creates a new Linear API client.
func NewClient(apiKey string, teamKeys []string) *Client {
	return &Client{
		apiKey:   apiKey,
		teamKeys: teamKeys,
		apiURL:   linearAPI,
		http:     &http.Client{Timeout: 30 * time.Second},
	}
}

// GetActiveIssues returns non-completed issues assigned to the viewer.
func (c *Client) GetActiveIssues(ctx context.Context) ([]Issue, error) {
	resp, err := AssignedIssues(ctx, c.graphQLClient(), c.baseFilter(), 50)
	if err != nil {
		return nil, fmt.Errorf("query active issues: %w", err)
	}

	return mapAssignedIssues(resp.Viewer.AssignedIssues.Nodes), nil
}

// GetFocusedIssues returns issues due today with started/unstarted state.
func (c *Client) GetFocusedIssues(ctx context.Context, date string) ([]Issue, error) {
	filter := map[string]any{
		"dueDate": map[string]any{"eq": date},
		"state":   map[string]any{"type": map[string]any{keyIn: []string{"started", "unstarted"}}},
	}
	c.applyTeamFilter(filter)

	resp, err := AssignedIssues(ctx, c.graphQLClient(), filter, 50)
	if err != nil {
		return nil, fmt.Errorf("query focused issues: %w", err)
	}

	return mapAssignedIssues(resp.Viewer.AssignedIssues.Nodes), nil
}

// GetCompletedTodayIssues returns issues completed since the given time.
func (c *Client) GetCompletedTodayIssues(ctx context.Context, since time.Time) ([]Issue, error) {
	filter := map[string]any{
		"completedAt": map[string]any{"gte": since.Format(time.RFC3339)},
	}
	c.applyTeamFilter(filter)

	resp, err := AssignedIssues(ctx, c.graphQLClient(), filter, 50)
	if err != nil {
		return nil, fmt.Errorf("query completed today: %w", err)
	}

	return mapAssignedIssues(resp.Viewer.AssignedIssues.Nodes), nil
}

// GetInProgressIssues returns currently in-progress issues.
func (c *Client) GetInProgressIssues(ctx context.Context) ([]Issue, error) {
	filter := map[string]any{
		"state": map[string]any{"type": map[string]any{"eq": "started"}},
	}
	c.applyTeamFilter(filter)

	resp, err := AssignedIssues(ctx, c.graphQLClient(), filter, 50)
	if err != nil {
		return nil, fmt.Errorf("query in-progress issues: %w", err)
	}

	return mapAssignedIssues(resp.Viewer.AssignedIssues.Nodes), nil
}

// GetStateChanges returns state transitions since the given time.
func (c *Client) GetStateChanges(ctx context.Context, since time.Time) ([]StateChange, error) {
	sinceStr := since.Format(time.RFC3339)
	filter := c.baseFilter()
	filter["updatedAt"] = map[string]any{"gte": sinceStr}

	resp, err := StateChanges(ctx, c.graphQLClient(), filter, 20, 5)
	if err != nil {
		return nil, fmt.Errorf("query state changes: %w", err)
	}

	changes := make([]StateChange, 0)
	for i := range resp.Viewer.AssignedIssues.Nodes {
		n := &resp.Viewer.AssignedIssues.Nodes[i]
		for _, h := range n.History.Nodes {
			if h.CreatedAt < sinceStr {
				continue
			}
			fromName := h.FromState.Name
			toName := h.ToState.Name
			if fromName == "" && toName == "" {
				continue
			}
			if fromName == toName {
				continue
			}
			changes = append(changes, StateChange{
				IssueIdentifier: n.Identifier,
				IssueTitle:      n.Title,
				FromState:       fromName,
				ToState:         toName,
				CreatedAt:       h.CreatedAt,
				TeamName:        n.Team.Name,
				TeamKey:         n.Team.Key,
				URL:             n.Url,
			})
		}
	}

	return changes, nil
}

// GetUpdatedIssuesWithDetails returns issues updated since the given time,
// including full description and comments for AI review.
func (c *Client) GetUpdatedIssuesWithDetails(ctx context.Context, since time.Time) ([]IssueDetail, error) {
	filter := map[string]any{
		"updatedAt": map[string]any{"gte": since.Format(time.RFC3339)},
	}
	c.applyTeamFilter(filter)

	resp, err := UpdatedIssuesWithDetails(ctx, c.graphQLClient(), filter, 50, 100)
	if err != nil {
		return nil, fmt.Errorf("query updated issues with details: %w", err)
	}

	details := make([]IssueDetail, 0, len(resp.Viewer.AssignedIssues.Nodes))
	for i := range resp.Viewer.AssignedIssues.Nodes {
		n := &resp.Viewer.AssignedIssues.Nodes[i]
		d := IssueDetail{
			Identifier:       n.Identifier,
			Title:            n.Title,
			Description:      n.Description,
			Priority:         n.Priority,
			StateName:        n.State.Name,
			StateType:        n.State.Type,
			TeamName:         n.Team.Name,
			TeamKey:          n.Team.Key,
			URL:              n.Url,
			CompletedAt:      n.CompletedAt,
			UpdatedAt:        n.UpdatedAt,
			ParentIdentifier: n.Parent.Identifier,
			Comments:         make([]Comment, 0, len(n.Comments.Nodes)),
		}
		for _, c := range n.Comments.Nodes {
			d.Comments = append(d.Comments, Comment{
				Body:      c.Body,
				UserName:  c.User.Name,
				CreatedAt: c.CreatedAt,
			})
		}
		details = append(details, d)
	}

	return details, nil
}

// baseFilter returns the common filter for active (non-completed) issues.
func (c *Client) baseFilter() map[string]any {
	filter := map[string]any{
		"state": map[string]any{
			"type": map[string]any{"nin": []string{"completed", "canceled"}},
		},
	}
	c.applyTeamFilter(filter)

	return filter
}

func (c *Client) applyTeamFilter(filter map[string]any) {
	if len(c.teamKeys) == 0 {
		return
	}
	filter["team"] = map[string]any{
		keyKey: map[string]any{keyIn: c.teamKeys},
	}
}

func (c *Client) graphQLClient() graphql.Client {
	endpoint := c.apiURL
	if endpoint == "" {
		endpoint = linearAPI
	}

	httpClient := http.Client{Timeout: 30 * time.Second}
	if c.http != nil {
		httpClient = *c.http
	}
	base := httpClient.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	httpClient.Transport = authTransport{token: c.apiKey, base: base}

	return graphql.NewClient(endpoint, &httpClient)
}

const linearAPI = "https://api.linear.app/graphql"

type authTransport struct {
	base  http.RoundTripper
	token string
}

func (t authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.token != "" {
		req.Header.Set("Authorization", t.token)
	}

	return t.base.RoundTrip(req)
}

func mapAssignedIssues(nodes []AssignedIssuesViewerUserAssignedIssuesIssueConnectionNodesIssue) []Issue {
	issues := make([]Issue, 0, len(nodes))
	for i := range nodes {
		n := &nodes[i]
		// Skip sub-issues; Linear returns an empty parent object for top-level issues.
		if n.Parent.Id != "" {
			continue
		}
		issues = append(issues, Issue{
			ID:          n.Id,
			Title:       n.Title,
			Identifier:  n.Identifier,
			Priority:    n.Priority,
			StateName:   n.State.Name,
			StateType:   n.State.Type,
			TeamName:    n.Team.Name,
			TeamKey:     n.Team.Key,
			DueDate:     n.DueDate,
			URL:         n.Url,
			UpdatedAt:   n.UpdatedAt,
			CompletedAt: n.CompletedAt,
		})
	}

	return issues
}

//go:generate go run github.com/Khan/genqlient genqlient.yaml
