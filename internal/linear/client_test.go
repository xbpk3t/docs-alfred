package linear

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	c := NewClient("test-key", []string{"TEAM"})
	require.NotNil(t, c)
	assert.Equal(t, "test-key", c.apiKey)
	assert.Equal(t, []string{"TEAM"}, c.teamKeys)
	assert.Equal(t, linearAPI, c.apiURL)
	assert.NotNil(t, c.http)
}

func TestNewClientWithHTTP(t *testing.T) {
	hc := &http.Client{Timeout: 10 * time.Second}
	c := NewClientWithHTTP("key", []string{"A"}, "https://custom.api", hc)
	assert.Equal(t, "key", c.apiKey)
	assert.Equal(t, []string{"A"}, c.teamKeys)
	assert.Equal(t, "https://custom.api", c.apiURL)
	assert.Equal(t, hc, c.http)
}

func TestBaseFilter_WithTeamKeys(t *testing.T) {
	c := &Client{teamKeys: []string{"ENG", "OPS"}}
	f := c.baseFilter()
	assert.NotNil(t, f["team"])
	teamFilter := f["team"].(map[string]any)
	keyFilter := teamFilter["key"].(map[string]any)
	assert.Equal(t, []string{"ENG", "OPS"}, keyFilter["in"])
}

func TestBaseFilter_WithoutTeamKeys(t *testing.T) {
	c := &Client{}
	f := c.baseFilter()
	_, hasTeam := f["team"]
	assert.False(t, hasTeam)
}

func TestApplyTeamFilter_WithKeys(t *testing.T) {
	c := &Client{teamKeys: []string{"ENG"}}
	f := map[string]any{}
	c.applyTeamFilter(f)
	assert.NotNil(t, f["team"])
}

func TestApplyTeamFilter_WithoutKeys(t *testing.T) {
	c := &Client{}
	f := map[string]any{}
	c.applyTeamFilter(f)
	_, hasTeam := f["team"]
	assert.False(t, hasTeam)
}

func TestMapAssignedIssues_Empty(t *testing.T) {
	result := mapAssignedIssues(nil)
	assert.Empty(t, result)
}

func TestMapAssignedIssues_FiltersSubIssues(t *testing.T) {
	nodes := []AssignedIssuesViewerUserAssignedIssuesIssueConnectionNodesIssue{
		{
			Id:         "1",
			Title:      "Top Level",
			Identifier: "ENG-1",
			Priority:   1,
			State:      AssignedIssuesViewerUserAssignedIssuesIssueConnectionNodesIssueStateWorkflowState{Name: "Todo", Type: "unstarted"},
			Team:       AssignedIssuesViewerUserAssignedIssuesIssueConnectionNodesIssueTeam{Name: "Eng", Key: "ENG"},
			DueDate:    "2024-06-01",
			Url:        "https://linear.app/eng/issue/ENG-1",
			UpdatedAt:  "2024-06-01T00:00:00Z",
		},
		{
			Id:         "2",
			Title:      "Sub Issue",
			Identifier: "ENG-2",
			Priority:   2,
			Parent:     AssignedIssuesViewerUserAssignedIssuesIssueConnectionNodesIssueParentIssue{Id: "1"},
			State:      AssignedIssuesViewerUserAssignedIssuesIssueConnectionNodesIssueStateWorkflowState{Name: "In Progress", Type: "started"},
			Team:       AssignedIssuesViewerUserAssignedIssuesIssueConnectionNodesIssueTeam{Name: "Eng", Key: "ENG"},
			Url:        "https://linear.app/eng/issue/ENG-2",
		},
		{
			Id:          "3",
			Title:       "Top Level 2",
			Identifier:  "ENG-3",
			Priority:    3,
			State:       AssignedIssuesViewerUserAssignedIssuesIssueConnectionNodesIssueStateWorkflowState{Name: "Done", Type: "completed"},
			Team:        AssignedIssuesViewerUserAssignedIssuesIssueConnectionNodesIssueTeam{Name: "Eng", Key: "ENG"},
			Url:         "https://linear.app/eng/issue/ENG-3",
			CompletedAt: "2024-06-01T12:00:00Z",
		},
	}

	result := mapAssignedIssues(nodes)
	require.Len(t, result, 2) // Sub issue (has parent) is filtered out
	assert.Equal(t, "ENG-1", result[0].Identifier)
	assert.Equal(t, "ENG-3", result[1].Identifier)
	assert.Equal(t, "Todo", result[0].StateName)
	assert.Equal(t, "unstarted", result[0].StateType)
	assert.Equal(t, "Eng", result[0].TeamName)
	assert.Equal(t, "ENG", result[0].TeamKey)
	assert.Equal(t, "2024-06-01", result[0].DueDate)
	assert.Equal(t, "https://linear.app/eng/issue/ENG-1", result[0].URL)
	assert.Equal(t, "2024-06-01T12:00:00Z", result[1].CompletedAt)
}

func TestMapIssueDetails_Empty(t *testing.T) {
	result := mapIssueDetails(nil)
	assert.Empty(t, result)
}

func TestMapIssueDetails_WithComments(t *testing.T) {
	nodes := []UpdatedIssuesWithDetailsViewerUserAssignedIssuesIssueConnectionNodesIssue{
		{
			Id:          "1",
			Identifier:  "ENG-1",
			Title:       "Test Issue",
			Description: "Description here",
			Priority:    2,
			Url:         "https://linear.app/eng/issue/ENG-1",
			UpdatedAt:   "2024-06-01T00:00:00Z",
			State:       UpdatedIssuesWithDetailsViewerUserAssignedIssuesIssueConnectionNodesIssueStateWorkflowState{Name: "Todo", Type: "unstarted"},
			Team:        UpdatedIssuesWithDetailsViewerUserAssignedIssuesIssueConnectionNodesIssueTeam{Name: "Eng", Key: "ENG"},
			Parent:      UpdatedIssuesWithDetailsViewerUserAssignedIssuesIssueConnectionNodesIssueParentIssue{},
			Comments: UpdatedIssuesWithDetailsViewerUserAssignedIssuesIssueConnectionNodesIssueCommentsCommentConnection{
				Nodes: []UpdatedIssuesWithDetailsViewerUserAssignedIssuesIssueConnectionNodesIssueCommentsCommentConnectionNodesComment{
					{
						Body:      "A comment",
						CreatedAt: "2024-06-01T12:00:00Z",
						User:      UpdatedIssuesWithDetailsViewerUserAssignedIssuesIssueConnectionNodesIssueCommentsCommentConnectionNodesCommentUser{Name: "Alice"},
					},
				},
			},
		},
	}

	result := mapIssueDetails(nodes)
	require.Len(t, result, 1)
	d := result[0]
	assert.Equal(t, "ENG-1", d.Identifier)
	assert.Equal(t, "Test Issue", d.Title)
	assert.Equal(t, "Description here", d.Description)
	assert.Equal(t, float64(2), d.Priority)
	assert.Equal(t, "Todo", d.StateName)
	assert.Equal(t, "unstarted", d.StateType)
	assert.Equal(t, "Eng", d.TeamName)
	assert.Equal(t, "ENG", d.TeamKey)
	assert.Equal(t, "https://linear.app/eng/issue/ENG-1", d.URL)
	assert.Equal(t, "2024-06-01T00:00:00Z", d.UpdatedAt)
	require.Len(t, d.Comments, 1)
	assert.Equal(t, "A comment", d.Comments[0].Body)
	assert.Equal(t, "Alice", d.Comments[0].UserName)
	assert.Equal(t, "2024-06-01T12:00:00Z", d.Comments[0].CreatedAt)
}

func TestAuthTransport_SetsAuthorizationHeader(t *testing.T) {
	var capturedToken string
	inner := &mockRoundTripper{handler: func(req *http.Request) {
		capturedToken = req.Header.Get("Authorization")
	}}
	at := authTransport{base: inner, token: "Bearer test-token"}
	req, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)
	_, err := at.RoundTrip(req)
	require.NoError(t, err)
	assert.Equal(t, "Bearer test-token", capturedToken)
}

func TestAuthTransport_EmptyToken(t *testing.T) {
	var capturedHasAuth bool
	inner := &mockRoundTripper{handler: func(req *http.Request) {
		capturedHasAuth = req.Header.Get("Authorization") != ""
	}}
	at := authTransport{base: inner, token: ""}
	req, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)
	_, err := at.RoundTrip(req)
	require.NoError(t, err)
	assert.False(t, capturedHasAuth)
}

func TestGraphQLClient_UsesProvidedHTTP(t *testing.T) {
	c := &Client{
		apiKey:   "test",
		apiURL:   "https://test.api",
		http:     &http.Client{Timeout: 5 * time.Second},
		teamKeys: nil,
	}
	client := c.graphQLClient()
	require.NotNil(t, client)
}

func TestGraphQLClient_FallsBackToDefault(t *testing.T) {
	c := &Client{
		apiKey: "test",
		apiURL: "https://test.api",
	}
	client := c.graphQLClient()
	require.NotNil(t, client)
}

func TestGraphQLClient_DefaultEndpoint(t *testing.T) {
	c := &Client{
		apiKey: "test",
		http:   &http.Client{},
	}
	client := c.graphQLClient()
	require.NotNil(t, client)
}

// --- Integration tests with mock HTTP server ---

func TestGetActiveIssues_WithMockServer(t *testing.T) {
	server := mockLinearServer(t, func(r *http.Request) any {
		return assignedIssuesResponse([]issueNode{
			{Id: "1", Title: "Task A", Identifier: "ENG-1", State: stateNode{Name: "Todo", Type: "unstarted"}, Team: teamNode{Name: "Eng", Key: "ENG"}},
		})
	})
	defer server.Close()

	c := NewClientWithHTTP("test-key", nil, server.URL, server.Client())
	issues, err := c.GetActiveIssues(context.Background())
	require.NoError(t, err)
	require.Len(t, issues, 1)
	assert.Equal(t, "ENG-1", issues[0].Identifier)
	assert.Equal(t, "Task A", issues[0].Title)
}

func TestGetFocusedIssues_WithMockServer(t *testing.T) {
	server := mockLinearServer(t, func(r *http.Request) any {
		return assignedIssuesResponse([]issueNode{
			{Id: "1", Title: "Focused", Identifier: "ENG-1", DueDate: "2024-06-01", State: stateNode{Name: "Todo", Type: "unstarted"}, Team: teamNode{Name: "Eng", Key: "ENG"}},
		})
	})
	defer server.Close()

	c := NewClientWithHTTP("test-key", nil, server.URL, server.Client())
	issues, err := c.GetFocusedIssues(context.Background(), "2024-06-01")
	require.NoError(t, err)
	require.Len(t, issues, 1)
	assert.Equal(t, "2024-06-01", issues[0].DueDate)
}

func TestGetCompletedTodayIssues_WithMockServer(t *testing.T) {
	server := mockLinearServer(t, func(r *http.Request) any {
		return assignedIssuesResponse([]issueNode{
			{Id: "1", Title: "Done", Identifier: "ENG-1", CompletedAt: "2024-06-01T12:00:00Z", State: stateNode{Name: "Done", Type: "completed"}, Team: teamNode{Name: "Eng", Key: "ENG"}},
		})
	})
	defer server.Close()

	c := NewClientWithHTTP("test-key", nil, server.URL, server.Client())
	since, _ := time.Parse(time.RFC3339, "2024-06-01T00:00:00Z")
	issues, err := c.GetCompletedTodayIssues(context.Background(), since)
	require.NoError(t, err)
	require.Len(t, issues, 1)
	assert.Equal(t, "2024-06-01T12:00:00Z", issues[0].CompletedAt)
}

func TestGetInProgressIssues_WithMockServer(t *testing.T) {
	server := mockLinearServer(t, func(r *http.Request) any {
		return assignedIssuesResponse([]issueNode{
			{Id: "1", Title: "In Progress", Identifier: "ENG-1", State: stateNode{Name: "In Progress", Type: "started"}, Team: teamNode{Name: "Eng", Key: "ENG"}},
		})
	})
	defer server.Close()

	c := NewClientWithHTTP("test-key", nil, server.URL, server.Client())
	issues, err := c.GetInProgressIssues(context.Background())
	require.NoError(t, err)
	require.Len(t, issues, 1)
	assert.Equal(t, "In Progress", issues[0].StateName)
}

func TestGetStateChanges_WithMockServer(t *testing.T) {
	server := mockLinearServer(t, func(r *http.Request) any {
		return map[string]any{
			"viewer": map[string]any{
				"assignedIssues": map[string]any{
					"nodes": []any{
						map[string]any{
							"id":         "1",
							"identifier": "ENG-1",
							"title":      "Task A",
							"url":        "https://linear.app/eng/issue/ENG-1",
							"updatedAt":  "2024-06-01T10:00:00Z",
							"team":       map[string]any{"name": "Eng", "key": "ENG"},
							"history": map[string]any{
								"nodes": []any{
									map[string]any{
										"fromState": map[string]any{"name": "Todo"},
										"toState":   map[string]any{"name": "In Progress"},
										"createdAt": "2024-06-01T10:00:00Z",
									},
									map[string]any{
										"fromState": map[string]any{"name": "In Progress"},
										"toState":   map[string]any{"name": "Todo"},
										"createdAt": "2024-05-01T00:00:00Z", // before since, should be skipped
									},
									map[string]any{
										"fromState": map[string]any{"name": ""},
										"toState":   map[string]any{"name": ""},
										"createdAt": "2024-06-01T11:00:00Z", // same from/to, should be skipped
									},
									map[string]any{
										"fromState": map[string]any{"name": "Todo"},
										"toState":   map[string]any{"name": "Todo"}, // same from/to, should be skipped
										"createdAt": "2024-06-01T12:00:00Z",
									},
								},
							},
						},
					},
				},
			},
		}
	})
	defer server.Close()

	c := NewClientWithHTTP("test-key", nil, server.URL, server.Client())
	since, _ := time.Parse(time.RFC3339, "2024-06-01T00:00:00Z")
	changes, err := c.GetStateChanges(context.Background(), since)
	require.NoError(t, err)
	require.Len(t, changes, 1) // Only the first transition should pass
	assert.Equal(t, "ENG-1", changes[0].IssueIdentifier)
	assert.Equal(t, "Todo", changes[0].FromState)
	assert.Equal(t, "In Progress", changes[0].ToState)
	assert.Equal(t, "Eng", changes[0].TeamName)
	assert.Equal(t, "ENG", changes[0].TeamKey)
}

func TestGetActiveIssuesWithDetails_WithMockServer(t *testing.T) {
	server := mockLinearServer(t, func(r *http.Request) any {
		return updatedIssuesWithDetailsResponse([]detailIssueNode{
			{
				Id: "1", Identifier: "ENG-1", Title: "Detailed", Description: "Full desc",
				Priority: 2, Url: "https://linear.app/eng/issue/ENG-1",
				State: stateNode{Name: "Todo", Type: "unstarted"}, Team: teamNode{Name: "Eng", Key: "ENG"},
				Parent: parentNode{},
				Comments: commentConnection{
					Nodes: []commentNode{{Body: "nice", CreatedAt: "2024-06-01", User: userNode{Name: "Bob"}}},
				},
			},
		})
	})
	defer server.Close()

	c := NewClientWithHTTP("test-key", nil, server.URL, server.Client())
	details, err := c.GetActiveIssuesWithDetails(context.Background())
	require.NoError(t, err)
	require.Len(t, details, 1)
	assert.Equal(t, "Detailed", details[0].Title)
	assert.Equal(t, "Full desc", details[0].Description)
	assert.Len(t, details[0].Comments, 1)
	assert.Equal(t, "Bob", details[0].Comments[0].UserName)
}

func TestGetUpdatedIssuesWithDetails_WithMockServer(t *testing.T) {
	server := mockLinearServer(t, func(r *http.Request) any {
		return updatedIssuesWithDetailsResponse([]detailIssueNode{
			{
				Id: "1", Identifier: "ENG-1", Title: "Updated", Description: "Desc",
				Priority: 1, Url: "https://linear.app/eng/issue/ENG-1",
				State: stateNode{Name: "Todo", Type: "unstarted"}, Team: teamNode{Name: "Eng", Key: "ENG"},
				Parent: parentNode{Id: "p1", Identifier: "ENG-0"},
			},
		})
	})
	defer server.Close()

	c := NewClientWithHTTP("test-key", []string{"ENG"}, server.URL, server.Client())
	since, _ := time.Parse(time.RFC3339, "2024-06-01T00:00:00Z")
	details, err := c.GetUpdatedIssuesWithDetails(context.Background(), since)
	require.NoError(t, err)
	require.Len(t, details, 1)
	assert.Equal(t, "ENG-0", details[0].ParentIdentifier)
}

func TestGetActiveIssues_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"Internal Server Error"}`))
	}))
	defer server.Close()

	c := NewClientWithHTTP("test-key", nil, server.URL, server.Client())
	_, err := c.GetActiveIssues(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "query active issues")
}

func TestGetFocusedIssues_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	c := NewClientWithHTTP("key", nil, server.URL, server.Client())
	_, err := c.GetFocusedIssues(context.Background(), "2024-06-01")
	require.Error(t, err)
}

func TestGetCompletedTodayIssues_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	c := NewClientWithHTTP("key", nil, server.URL, server.Client())
	_, err := c.GetCompletedTodayIssues(context.Background(), time.Now())
	require.Error(t, err)
}

func TestGetInProgressIssues_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	c := NewClientWithHTTP("key", nil, server.URL, server.Client())
	_, err := c.GetInProgressIssues(context.Background())
	require.Error(t, err)
}

func TestGetStateChanges_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	c := NewClientWithHTTP("key", nil, server.URL, server.Client())
	_, err := c.GetStateChanges(context.Background(), time.Now())
	require.Error(t, err)
}

func TestGetActiveIssuesWithDetails_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	c := NewClientWithHTTP("key", nil, server.URL, server.Client())
	_, err := c.GetActiveIssuesWithDetails(context.Background())
	require.Error(t, err)
}

func TestGetUpdatedIssuesWithDetails_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	c := NewClientWithHTTP("key", nil, server.URL, server.Client())
	_, err := c.GetUpdatedIssuesWithDetails(context.Background(), time.Now())
	require.Error(t, err)
}

// --- Helper types for mock server ---

type mockRoundTripper struct {
	handler func(req *http.Request)
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	m.handler(req)

	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
}

type issueNode struct {
	State       stateNode  `json:"state"`
	Team        teamNode   `json:"team"`
	Parent      parentNode `json:"parent"`
	Id          string     `json:"id"`
	Title       string     `json:"title"`
	Identifier  string     `json:"identifier"`
	DueDate     string     `json:"dueDate,omitempty"`
	Url         string     `json:"url"`
	UpdatedAt   string     `json:"updatedAt"`
	CompletedAt string     `json:"completedAt,omitempty"`
	Priority    float64    `json:"priority"`
}

type detailIssueNode struct {
	State       stateNode         `json:"state"`
	Team        teamNode          `json:"team"`
	Parent      parentNode        `json:"parent"`
	Id          string            `json:"id"`
	Identifier  string            `json:"identifier"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Url         string            `json:"url"`
	CompletedAt string            `json:"completedAt,omitempty"`
	UpdatedAt   string            `json:"updatedAt"`
	Comments    commentConnection `json:"comments"`
	Priority    float64           `json:"priority"`
}

type stateNode struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type teamNode struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

type parentNode struct {
	Id         string `json:"id,omitempty"`
	Identifier string `json:"identifier,omitempty"`
}

type commentConnection struct {
	Nodes []commentNode `json:"nodes"`
}

type commentNode struct {
	Body      string   `json:"body"`
	CreatedAt string   `json:"createdAt"`
	User      userNode `json:"user"`
}

type userNode struct {
	Name string `json:"name"`
}

func assignedIssuesResponse(nodes []issueNode) map[string]any {
	return map[string]any{
		"viewer": map[string]any{
			"assignedIssues": map[string]any{
				"nodes": nodes,
			},
		},
	}
}

func updatedIssuesWithDetailsResponse(nodes []detailIssueNode) map[string]any {
	return map[string]any{
		"viewer": map[string]any{
			"assignedIssues": map[string]any{
				"nodes": nodes,
			},
		},
	}
}

func mockLinearServer(t *testing.T, handler func(r *http.Request) any) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := handler(r)
		resp := map[string]any{"data": data}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
}
