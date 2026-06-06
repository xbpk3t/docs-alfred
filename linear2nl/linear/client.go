package linear

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// GraphQL field/argument keys used in queries.
const (
	keyFilter = "filter"
	keyFirst  = "first"
	keyIn     = "in"
	keyKey    = "key"
)

// issuesQuery is the common GraphQL query for assigned issues with standard fields.
const issuesQuery = `query($filter: IssueFilter!, $first: Int) {
		viewer {
			assignedIssues(filter: $filter, first: $first) {
				nodes {
					id title identifier priority
					parent { id }
					state { name type }
					team { name key }
					dueDate url updatedAt completedAt
				}
			}
		}
	}`

// Client communicates with the Linear GraphQL API via plain HTTP.
type Client struct {
	http     *http.Client
	apiKey   string
	teamKeys []string
}

// NewClient creates a new Linear API client.
func NewClient(apiKey string, teamKeys []string) *Client {
	return &Client{
		apiKey:   apiKey,
		teamKeys: teamKeys,
		http:     &http.Client{Timeout: 30 * time.Second},
	}
}

// GetActiveIssues returns non-completed issues assigned to the viewer.
func (c *Client) GetActiveIssues(ctx context.Context) ([]Issue, error) {
	vars := map[string]any{
		keyFilter: c.baseFilter(),
		keyFirst:  50,
	}
	resp, err := c.queryNodes(ctx, issuesQuery, vars)
	if err != nil {
		return nil, fmt.Errorf("query active issues: %w", err)
	}

	return mapIssues(resp), nil
}

// GetFocusedIssues returns issues due today with started/unstarted state.
func (c *Client) GetFocusedIssues(ctx context.Context, date string) ([]Issue, error) {
	filter := map[string]any{
		"dueDate": map[string]any{"eq": date},
		"state":   map[string]any{"type": map[string]any{keyIn: []string{"started", "unstarted"}}},
	}
	if len(c.teamKeys) > 0 {
		filter["team"] = map[string]any{keyKey: map[string]any{keyIn: c.teamKeys}}
	}
	resp, err := c.queryNodes(ctx, issuesQuery, map[string]any{keyFilter: filter, keyFirst: 50})
	if err != nil {
		return nil, fmt.Errorf("query focused issues: %w", err)
	}

	return mapIssues(resp), nil
}

// GetCompletedTodayIssues returns issues completed since the given time.
func (c *Client) GetCompletedTodayIssues(ctx context.Context, since time.Time) ([]Issue, error) {
	filter := map[string]any{
		"completedAt": map[string]any{"gte": since.Format(time.RFC3339)},
	}
	if len(c.teamKeys) > 0 {
		filter["team"] = map[string]any{keyKey: map[string]any{keyIn: c.teamKeys}}
	}
	resp, err := c.queryNodes(ctx, issuesQuery, map[string]any{keyFilter: filter, keyFirst: 50})
	if err != nil {
		return nil, fmt.Errorf("query completed today: %w", err)
	}

	return mapIssues(resp), nil
}

// GetInProgressIssues returns currently in-progress issues.
func (c *Client) GetInProgressIssues(ctx context.Context) ([]Issue, error) {
	filter := map[string]any{
		"state": map[string]any{"type": map[string]any{"eq": "started"}},
	}
	if len(c.teamKeys) > 0 {
		filter["team"] = map[string]any{keyKey: map[string]any{keyIn: c.teamKeys}}
	}
	resp, err := c.queryNodes(ctx, issuesQuery, map[string]any{keyFilter: filter, keyFirst: 50})
	if err != nil {
		return nil, fmt.Errorf("query in-progress issues: %w", err)
	}

	return mapIssues(resp), nil
}

// GetStateChanges returns state transitions since the given time.
func (c *Client) GetStateChanges(ctx context.Context, since time.Time) ([]StateChange, error) {
	sinceStr := since.Format(time.RFC3339)
	filter := c.baseFilter()
	filter["updatedAt"] = map[string]any{"gte": sinceStr}

	query := `query($filter: IssueFilter!, $first: Int) {
		viewer {
			assignedIssues(filter: $filter, first: $first) {
				nodes {
					id identifier title
					team { name key }
					url updatedAt
					history(first: 5) {
						nodes {
							fromState { name }
							toState { name }
							createdAt
						}
					}
				}
			}
		}
	}`

	type rawHistoryNode struct {
		FromState *struct {
			Name string `json:"name"`
		} `json:"fromState"`
		ToState *struct {
			Name string `json:"name"`
		} `json:"toState"`
		CreatedAt string `json:"createdAt"`
	}
	type rawIssueNode struct {
		ID         string `json:"id"`
		Identifier string `json:"identifier"`
		Title      string `json:"title"`
		Team       struct {
			Name string `json:"name"`
			Key  string `json:"key"`
		} `json:"team"`
		URL       string `json:"url"`
		UpdatedAt string `json:"updatedAt"`
		History   struct {
			Nodes []rawHistoryNode `json:"nodes"`
		} `json:"history"`
	}

	vars := map[string]any{keyFilter: filter, keyFirst: 20}
	var stateChangeResp struct {
		Viewer struct {
			AssignedIssues struct {
				Nodes []rawIssueNode `json:"nodes"`
			} `json:"assignedIssues"`
		} `json:"viewer"`
	}
	if err := c.doRawQuery(ctx, query, vars, &stateChangeResp); err != nil {
		return nil, fmt.Errorf("query state changes: %w", err)
	}

	var changes []StateChange
	for i := range stateChangeResp.Viewer.AssignedIssues.Nodes {
		n := &stateChangeResp.Viewer.AssignedIssues.Nodes[i]
		for _, h := range n.History.Nodes {
			if h.CreatedAt < sinceStr {
				continue
			}
			fromName, toName := "", ""
			if h.FromState != nil {
				fromName = h.FromState.Name
			}
			if h.ToState != nil {
				toName = h.ToState.Name
			}
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
				URL:             n.URL,
			})
		}
	}

	return changes, nil
}

// GetUpdatedIssuesWithDetails returns issues updated since the given time,
// including full description and comments for AI review.
func (c *Client) GetUpdatedIssuesWithDetails(ctx context.Context, since time.Time) ([]IssueDetail, error) {
	sinceStr := since.Format(time.RFC3339)

	query := `query($filter: IssueFilter!, $first: Int) {
		viewer {
			assignedIssues(filter: $filter, first: $first) {
				nodes {
					id identifier title description priority url
					completedAt updatedAt
					state { name type }
					team { name key }
					comments(first: 10) {
						nodes {
							body
							createdAt
							user { name }
						}
					}
				}
			}
		}
	}`

	type rawComment struct {
		User *struct {
			Name string `json:"name"`
		} `json:"user"`
		Body      string `json:"body"`
		CreatedAt string `json:"createdAt"`
	}

	type rawDetailNode struct {
		Description *string `json:"description"`
		CompletedAt *string `json:"completedAt"`
		State       struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"state"`
		Team struct {
			Name string `json:"name"`
			Key  string `json:"key"`
		} `json:"team"`
		ID         string `json:"id"`
		Identifier string `json:"identifier"`
		Title      string `json:"title"`
		URL        string `json:"url"`
		UpdatedAt  string `json:"updatedAt"`
		Comments   struct {
			Nodes []rawComment `json:"nodes"`
		} `json:"comments"`
		Priority float64 `json:"priority"`
	}

	filter := map[string]any{
		"updatedAt": map[string]any{"gte": sinceStr},
	}
	if len(c.teamKeys) > 0 {
		filter["team"] = map[string]any{keyKey: map[string]any{keyIn: c.teamKeys}}
	}

	var resp struct {
		Viewer struct {
			AssignedIssues struct {
				Nodes []rawDetailNode `json:"nodes"`
			} `json:"assignedIssues"`
		} `json:"viewer"`
	}

	vars := map[string]any{keyFilter: filter, keyFirst: 50}
	if err := c.doRawQuery(ctx, query, vars, &resp); err != nil {
		return nil, fmt.Errorf("query updated issues with details: %w", err)
	}

	details := make([]IssueDetail, 0, len(resp.Viewer.AssignedIssues.Nodes))
	for i := range resp.Viewer.AssignedIssues.Nodes {
		n := &resp.Viewer.AssignedIssues.Nodes[i]
		d := IssueDetail{
			Identifier: n.Identifier,
			Title:      n.Title,
			Priority:   n.Priority,
			StateName:  n.State.Name,
			StateType:  n.State.Type,
			TeamName:   n.Team.Name,
			TeamKey:    n.Team.Key,
			URL:        n.URL,
			UpdatedAt:  n.UpdatedAt,
			Comments:   make([]Comment, 0, len(n.Comments.Nodes)),
		}
		if n.Description != nil {
			d.Description = *n.Description
		}
		if n.CompletedAt != nil {
			d.CompletedAt = *n.CompletedAt
		}
		for _, c := range n.Comments.Nodes {
			userName := ""
			if c.User != nil {
				userName = c.User.Name
			}
			d.Comments = append(d.Comments, Comment{
				Body:      c.Body,
				UserName:  userName,
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
	if len(c.teamKeys) > 0 {
		filter["team"] = map[string]any{
			keyKey: map[string]any{keyIn: c.teamKeys},
		}
	}

	return filter
}

// --- GraphQL execution ---

const linearAPI = "https://api.linear.app/graphql"

type gqlPayload struct {
	Variables map[string]any `json:"variables,omitempty"`
	Query     string         `json:"query"`
}

type gqlEnvelope struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors,omitempty"`
}

// queryNodes performs a GraphQL query that returns viewer.assignedIssues.nodes.
func (c *Client) queryNodes(ctx context.Context, query string, vars map[string]any) ([]issueNode, error) {
	var envelope struct {
		Viewer struct {
			AssignedIssues struct {
				Nodes []issueNode `json:"nodes"`
			} `json:"assignedIssues"`
		} `json:"viewer"`
	}
	if err := c.doRawQuery(ctx, query, vars, &envelope); err != nil {
		return nil, err
	}

	return envelope.Viewer.AssignedIssues.Nodes, nil
}

// doRawQuery runs a raw GraphQL query against the Linear API and decodes the
// "data" field into the provided response pointer.
func (c *Client) doRawQuery(ctx context.Context, query string, vars map[string]any, response any) error {
	payload := gqlPayload{
		Query:     query,
		Variables: vars,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, linearAPI, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var envelope gqlEnvelope
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	if len(envelope.Errors) > 0 {
		return fmt.Errorf("GraphQL errors: %s", envelope.Errors[0].Message)
	}
	if err := json.Unmarshal(envelope.Data, response); err != nil {
		return fmt.Errorf("unmarshal data: %w", err)
	}

	return nil
}

// --- Response types ---

type issueNode struct {
	DueDate     *string `json:"dueDate"`
	CompletedAt *string `json:"completedAt"`
	Parent      *struct {
		ID string `json:"id"`
	} `json:"parent"`
	State struct {
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"state"`
	Team struct {
		Name string `json:"name"`
		Key  string `json:"key"`
	} `json:"team"`
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	Identifier string  `json:"identifier"`
	URL        string  `json:"url"`
	UpdatedAt  string  `json:"updatedAt"`
	Priority   float64 `json:"priority"`
}

func mapIssues(nodes []issueNode) []Issue {
	issues := make([]Issue, 0, len(nodes))
	for i := range nodes {
		n := &nodes[i]
		// Skip sub-issues — only show top-level issues.
		if n.Parent != nil {
			continue
		}
		iss := Issue{
			ID:         n.ID,
			Title:      n.Title,
			Identifier: n.Identifier,
			Priority:   n.Priority,
			StateName:  n.State.Name,
			StateType:  n.State.Type,
			TeamName:   n.Team.Name,
			TeamKey:    n.Team.Key,
			URL:        n.URL,
			UpdatedAt:  n.UpdatedAt,
		}
		if n.DueDate != nil {
			iss.DueDate = *n.DueDate
		}
		if n.CompletedAt != nil {
			iss.CompletedAt = *n.CompletedAt
		}
		issues = append(issues, iss)
	}

	return issues
}
