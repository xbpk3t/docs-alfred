package linear

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/xbpk3t/docs-alfred/pkg/httputil"
)

// GraphQL field/argument keys and state values used in Linear filters.
const (
	keyIn  = "in"
	keyKey = "key"

	stateStarted   = "started"
	stateUnstarted = "unstarted"
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
		http:     httputil.StdHTTPClient(30 * time.Second),
	}
}

// NewClientWithHTTP creates a Linear API client with a custom HTTP client for testing.
func NewClientWithHTTP(apiKey string, teamKeys []string, apiURL string, httpClient *http.Client) *Client {
	return &Client{
		apiKey:   apiKey,
		teamKeys: teamKeys,
		apiURL:   apiURL,
		http:     httpClient,
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
		"state":   map[string]any{"type": map[string]any{keyIn: []string{stateStarted, stateUnstarted}}},
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
		"state": map[string]any{"type": map[string]any{"eq": stateStarted}},
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

	changes := make([]StateChange, 0, len(resp.Viewer.AssignedIssues.Nodes))
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

// GetActiveIssuesWithDetails returns non-completed issues assigned to the viewer,
// including full description and comments for AI review.
func (c *Client) GetActiveIssuesWithDetails(ctx context.Context) ([]IssueDetail, error) {
	resp, err := UpdatedIssuesWithDetails(ctx, c.graphQLClient(), c.baseFilter(), 50, 100)
	if err != nil {
		return nil, fmt.Errorf("query active issues with details: %w", err)
	}

	details := mapIssueDetails(resp.Viewer.AssignedIssues.Nodes)

	return details, nil
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

	details := mapIssueDetails(resp.Viewer.AssignedIssues.Nodes)

	return details, nil
}

// mapIssueDetails converts UpdatedIssuesWithDetails response nodes to IssueDetail slice.
func mapIssueDetails(nodes []UpdatedIssuesWithDetailsViewerUserAssignedIssuesIssueConnectionNodesIssue) []IssueDetail {
	details := make([]IssueDetail, 0, len(nodes))
	for i := range nodes {
		n := &nodes[i]
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

	return details
}

// baseFilter returns the common filter for active (non-completed) issues.
func (c *Client) baseFilter() map[string]any {
	filter := map[string]any{
		"state": map[string]any{
			"type": map[string]any{"nin": []string{"completed", "canceled", "backlog"}},
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

	httpClient := httputil.StdHTTPClient(30 * time.Second)
	if c.http != nil {
		httpClient = c.http
	}
	base := httpClient.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	httpClient.Transport = authTransport{token: c.apiKey, base: base}

	return graphql.NewClient(endpoint, httpClient)
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

// CreateIssueInput is the payload for issueCreate.
type CreateIssueInput struct {
	TeamID      string
	Title       string
	Description string
	// StateID sets workflow state (e.g. In Review). Empty leaves team default.
	StateID string
	// AssigneeID assigns the issue. Empty leaves unassigned.
	AssigneeID string
	// Priority is Linear priority: 0 none, 1 urgent, 2 high, 3 medium, 4 low.
	// Negative means omit (API default).
	Priority int
}

// CreateIssue creates a new Linear issue via issueCreate.
// Always creates; callers that need dedup must implement it themselves.
func (c *Client) CreateIssue(ctx context.Context, in *CreateIssueInput) (*Issue, error) {
	if in == nil {
		return nil, fmt.Errorf("create issue: input is required")
	}
	if strings.TrimSpace(in.TeamID) == "" {
		return nil, fmt.Errorf("create issue: teamId is required")
	}
	if strings.TrimSpace(in.Title) == "" {
		return nil, fmt.Errorf("create issue: title is required")
	}

	resp, err := issueCreateMutation(ctx, c.graphQLClient(), in)
	if err != nil {
		return nil, fmt.Errorf("create issue: %w", err)
	}
	if !resp.IssueCreate.Success || resp.IssueCreate.Issue.Id == "" {
		return nil, fmt.Errorf("create issue: mutation reported success=%v id=%q",
			resp.IssueCreate.Success, resp.IssueCreate.Issue.Id)
	}

	n := resp.IssueCreate.Issue
	return &Issue{
		ID:         n.Id,
		Title:      n.Title,
		Identifier: n.Identifier,
		URL:        n.Url,
		TeamName:   n.Team.Name,
		TeamKey:    n.Team.Key,
		Priority:   n.Priority,
		StateName:  n.State.Name,
		StateType:  n.State.Type,
	}, nil
}

// ViewerID returns the user id for the authenticated API key.
func (c *Client) ViewerID(ctx context.Context) (string, error) {
	resp, err := viewerIDQuery(ctx, c.graphQLClient())
	if err != nil {
		return "", fmt.Errorf("query viewer id: %w", err)
	}
	id := strings.TrimSpace(resp.Viewer.Id)
	if id == "" {
		return "", fmt.Errorf("query viewer id: empty id")
	}
	return id, nil
}

// ResolveStateID finds a workflow state id on a team by exact name (case-sensitive).
func (c *Client) ResolveStateID(ctx context.Context, teamID, stateName string) (string, error) {
	teamID = strings.TrimSpace(teamID)
	stateName = strings.TrimSpace(stateName)
	if teamID == "" {
		return "", fmt.Errorf("resolve state id: teamId is required")
	}
	if stateName == "" {
		return "", fmt.Errorf("resolve state id: stateName is required")
	}

	resp, err := teamStatesQuery(ctx, c.graphQLClient(), teamID)
	if err != nil {
		return "", fmt.Errorf("resolve state id %q: %w", stateName, err)
	}
	for _, n := range resp.Team.States.Nodes {
		if n.Name == stateName {
			if n.Id == "" {
				return "", fmt.Errorf("resolve state id %q: empty id", stateName)
			}
			return n.Id, nil
		}
	}
	return "", fmt.Errorf("resolve state id %q: not found on team", stateName)
}

// ResolveTeamID looks up a team UUID by its key (e.g. "LUC").
func (c *Client) ResolveTeamID(ctx context.Context, teamKey string) (string, error) {
	teamKey = strings.TrimSpace(teamKey)
	if teamKey == "" {
		return "", fmt.Errorf("resolve team id: teamKey is required")
	}

	resp, err := teamsByKeyQuery(ctx, c.graphQLClient(), teamKey)
	if err != nil {
		return "", fmt.Errorf("resolve team id %s: %w", teamKey, err)
	}
	nodes := resp.Teams.Nodes
	if len(nodes) == 0 {
		return "", fmt.Errorf("resolve team id %s: not found", teamKey)
	}
	id := nodes[0].Id
	if id == "" {
		return "", fmt.Errorf("resolve team id %s: empty id", teamKey)
	}
	return id, nil
}

// GetIssueByIdentifier fetches a single issue by its identifier (e.g. "LUC-153")
// including description and comments. Uses the issue(id:) query which accepts
// both UUIDs and identifiers like "LUC-153".
func (c *Client) GetIssueByIdentifier(ctx context.Context, identifier string) (*IssueDetail, error) {
	resp, err := IssueByIDQuery(ctx, c.graphQLClient(), identifier, 100)
	if err != nil {
		return nil, fmt.Errorf("query issue %s: %w", identifier, err)
	}

	if resp.Issue.Id == "" {
		return nil, fmt.Errorf("issue %s not found", identifier)
	}

	n := resp.Issue
	d := &IssueDetail{
		Identifier:  n.Identifier,
		Title:       n.Title,
		Description: n.Description,
		Priority:    n.Priority,
		StateName:   n.State.Name,
		StateType:   n.State.Type,
		TeamName:    n.Team.Name,
		TeamKey:     n.Team.Key,
		URL:         n.Url,
		CompletedAt: n.CompletedAt,
		UpdatedAt:   n.UpdatedAt,
		Comments:    make([]Comment, 0, len(n.Comments.Nodes)),
	}

	for _, cm := range n.Comments.Nodes {
		d.Comments = append(d.Comments, Comment{
			Body:      cm.Body,
			UserName:  cm.User.Name,
			CreatedAt: cm.CreatedAt,
		})
	}

	return d, nil
}

// IssueByIDResponse is returned by IssueByIDQuery on success.
type IssueByIDResponse struct {
	Issue IssueByIDIssue `json:"issue"`
}

// IssueByIDIssue includes the requested fields of the GraphQL type Issue.
type IssueByIDIssue struct {
	State       IssueByIDIssueStateWorkflowState        `json:"state"`
	Team        IssueByIDIssueTeam                      `json:"team"`
	Id          string                                  `json:"id"`
	Identifier  string                                  `json:"identifier"`
	Title       string                                  `json:"title"`
	Description string                                  `json:"description"`
	Url         string                                  `json:"url"`
	CompletedAt string                                  `json:"completedAt"`
	UpdatedAt   string                                  `json:"updatedAt"`
	Comments    IssueByIDIssueCommentsCommentConnection `json:"comments"`
	Priority    float64                                 `json:"priority"`
}

// IssueByIDIssueStateWorkflowState includes the requested fields of the GraphQL type WorkflowState.
type IssueByIDIssueStateWorkflowState struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// IssueByIDIssueTeam includes the requested fields of the GraphQL type Team.
type IssueByIDIssueTeam struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

// IssueByIDIssueCommentsCommentConnection includes the requested fields of the GraphQL type CommentConnection.
type IssueByIDIssueCommentsCommentConnection struct {
	Nodes []IssueByIDIssueCommentsCommentConnectionNodesComment `json:"nodes"`
}

// IssueByIDIssueCommentsCommentConnectionNodesComment includes the requested fields of the GraphQL type Comment.
type IssueByIDIssueCommentsCommentConnectionNodesComment struct {
	Body      string                                                  `json:"body"`
	CreatedAt string                                                  `json:"createdAt"`
	User      IssueByIDIssueCommentsCommentConnectionNodesCommentUser `json:"user"`
}

// IssueByIDIssueCommentsCommentConnectionNodesCommentUser includes the requested fields of the GraphQL type User.
type IssueByIDIssueCommentsCommentConnectionNodesCommentUser struct {
	Name string `json:"name"`
}

// IssueByIDQuery fetches a single issue by ID or identifier with comments.
func IssueByIDQuery(
	ctx context.Context,
	c graphql.Client,
	id string,
	commentsFirst int,
) (data *IssueByIDResponse, err error) {
	req := &graphql.Request{
		OpName: "IssueByID",
		Query: `
query IssueByID ($id: String!, $commentsFirst: Int) {
	issue(id: $id) {
		id
		identifier
		title
		description
		priority
		url
		completedAt
		updatedAt
		state {
			name
			type
		}
		team {
			name
			key
		}
		comments(first: $commentsFirst) {
			nodes {
				body
				createdAt
				user {
					name
				}
			}
		}
	}
}
`,
		Variables: &issueByIDInput{
			ID:            id,
			CommentsFirst: commentsFirst,
		},
	}

	data = &IssueByIDResponse{}
	resp := &graphql.Response{Data: data}

	err = c.MakeRequest(ctx, req, resp)

	return data, err
}

// issueByIDInput holds variables for the IssueByID query.
type issueByIDInput struct {
	ID            string `json:"id"`
	CommentsFirst int    `json:"commentsFirst"`
}

// --- issueCreate (hand-written, same style as IssueByIDQuery) ---

type issueCreateResponse struct {
	IssueCreate issueCreatePayload `json:"issueCreate"`
}

type issueCreatePayload struct {
	Issue   issueCreateIssue `json:"issue"`
	Success bool             `json:"success"`
}

type issueCreateIssue struct {
	Team       issueCreateTeam  `json:"team"`
	State      issueCreateState `json:"state"`
	Id         string           `json:"id"`
	Identifier string           `json:"identifier"`
	Title      string           `json:"title"`
	Url        string           `json:"url"`
	Priority   float64          `json:"priority"`
}

type issueCreateTeam struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

type issueCreateState struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type issueCreateVars struct {
	Input map[string]any `json:"input"`
}

func issueCreateMutation(
	ctx context.Context,
	c graphql.Client,
	in *CreateIssueInput,
) (*issueCreateResponse, error) {
	input := map[string]any{
		"teamId": in.TeamID,
		"title":  in.Title,
	}
	if strings.TrimSpace(in.Description) != "" {
		input["description"] = in.Description
	}
	if strings.TrimSpace(in.StateID) != "" {
		input["stateId"] = in.StateID
	}
	if strings.TrimSpace(in.AssigneeID) != "" {
		input["assigneeId"] = in.AssigneeID
	}
	if in.Priority >= 0 {
		input["priority"] = in.Priority
	}

	req := &graphql.Request{
		OpName: "IssueCreate",
		Query: `
mutation IssueCreate ($input: IssueCreateInput!) {
	issueCreate(input: $input) {
		success
		issue {
			id
			identifier
			title
			url
			priority
			state {
				name
				type
			}
			team {
				name
				key
			}
		}
	}
}
`,
		Variables: &issueCreateVars{Input: input},
	}

	data := &issueCreateResponse{}
	resp := &graphql.Response{Data: data}
	err := c.MakeRequest(ctx, req, resp)
	return data, err
}

// --- viewer id ---

type viewerIDResponse struct {
	Viewer viewerIDNode `json:"viewer"`
}

type viewerIDNode struct {
	Id string `json:"id"`
}

func viewerIDQuery(ctx context.Context, c graphql.Client) (*viewerIDResponse, error) {
	req := &graphql.Request{
		OpName: "ViewerID",
		Query: `
query ViewerID {
	viewer {
		id
	}
}
`,
	}
	data := &viewerIDResponse{}
	resp := &graphql.Response{Data: data}
	err := c.MakeRequest(ctx, req, resp)
	return data, err
}

// --- team workflow states ---

type teamStatesResponse struct {
	Team teamStatesTeam `json:"team"`
}

type teamStatesTeam struct {
	Id     string               `json:"id"`
	States teamStatesConnection `json:"states"`
}

type teamStatesConnection struct {
	Nodes []teamStateNode `json:"nodes"`
}

type teamStateNode struct {
	Id   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type teamStatesVars struct {
	ID string `json:"id"`
}

func teamStatesQuery(ctx context.Context, c graphql.Client, teamID string) (*teamStatesResponse, error) {
	req := &graphql.Request{
		OpName: "TeamStates",
		Query: `
query TeamStates ($id: String!) {
	team(id: $id) {
		id
		states {
			nodes {
				id
				name
				type
			}
		}
	}
}
`,
		Variables: &teamStatesVars{ID: teamID},
	}
	data := &teamStatesResponse{}
	resp := &graphql.Response{Data: data}
	err := c.MakeRequest(ctx, req, resp)
	return data, err
}

// --- teams by key ---

type teamsByKeyResponse struct {
	Teams teamsByKeyConnection `json:"teams"`
}

type teamsByKeyConnection struct {
	Nodes []teamsByKeyNode `json:"nodes"`
}

type teamsByKeyNode struct {
	Id   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

type teamsByKeyVars struct {
	Filter map[string]any `json:"filter"`
}

func teamsByKeyQuery(
	ctx context.Context,
	c graphql.Client,
	teamKey string,
) (*teamsByKeyResponse, error) {
	req := &graphql.Request{
		OpName: "TeamsByKey",
		Query: `
query TeamsByKey ($filter: TeamFilter) {
	teams(filter: $filter) {
		nodes {
			id
			key
			name
		}
	}
}
`,
		Variables: &teamsByKeyVars{
			Filter: map[string]any{
				"key": map[string]any{"eq": teamKey},
			},
		},
	}

	data := &teamsByKeyResponse{}
	resp := &graphql.Response{Data: data}
	err := c.MakeRequest(ctx, req, resp)
	return data, err
}

//go:generate go run github.com/Khan/genqlient genqlient.yaml
