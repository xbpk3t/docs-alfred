package linear

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIssue_Fields(t *testing.T) {
	issue := Issue{
		ID:          "id1",
		Title:       "title",
		Identifier:  "ENG-1",
		StateName:   "Todo",
		StateType:   "unstarted",
		TeamName:    "Eng",
		TeamKey:     "ENG",
		DueDate:     "2024-06-01",
		URL:         "https://linear.app/eng/issue/ENG-1",
		UpdatedAt:   "2024-06-01T00:00:00Z",
		CompletedAt: "",
		Priority:    2,
	}
	assert.Equal(t, "id1", issue.ID)
	assert.Equal(t, "ENG-1", issue.Identifier)
	assert.Equal(t, float64(2), issue.Priority)
}

func TestStateChange_Fields(t *testing.T) {
	sc := StateChange{
		IssueIdentifier: "ENG-1",
		IssueTitle:      "Title",
		FromState:       "Todo",
		ToState:         "In Progress",
		CreatedAt:       "2024-06-01T10:00:00Z",
		TeamName:        "Eng",
		TeamKey:         "ENG",
		URL:             "https://linear.app/eng/issue/ENG-1",
	}
	assert.Equal(t, "ENG-1", sc.IssueIdentifier)
	assert.Equal(t, "Todo", sc.FromState)
	assert.Equal(t, "In Progress", sc.ToState)
}

func TestIssueDetail_Fields(t *testing.T) {
	d := IssueDetail{
		Identifier:       "ENG-1",
		Title:            "Title",
		Description:      "Desc",
		StateName:        "Todo",
		StateType:        "unstarted",
		TeamName:         "Eng",
		TeamKey:          "ENG",
		URL:              "https://linear.app",
		CompletedAt:      "",
		UpdatedAt:        "2024-06-01",
		ParentIdentifier: "ENG-0",
		Priority:         2,
		Comments:         []Comment{{Body: "comment", UserName: "Alice", CreatedAt: "2024-06-01"}},
	}
	assert.Equal(t, "ENG-1", d.Identifier)
	assert.Len(t, d.Comments, 1)
	assert.Equal(t, "ENG-0", d.ParentIdentifier)
}

func TestComment_Fields(t *testing.T) {
	c := Comment{Body: "body", UserName: "Bob", CreatedAt: "2024-06-01"}
	assert.Equal(t, "body", c.Body)
	assert.Equal(t, "Bob", c.UserName)
}

func TestGenerated_Getters_AssignedIssuesResponse(t *testing.T) {
	r := &AssignedIssuesResponse{
		Viewer: AssignedIssuesViewerUser{
			AssignedIssues: AssignedIssuesViewerUserAssignedIssuesIssueConnection{
				Nodes: []AssignedIssuesViewerUserAssignedIssuesIssueConnectionNodesIssue{
					{Id: "1", Title: "T", Identifier: "ENG-1", Priority: 2, DueDate: "2024-01-01", Url: "u", UpdatedAt: "2024-01-01", CompletedAt: "2024-01-01"},
				},
			},
		},
	}
	v := &r.Viewer
	a := &v.AssignedIssues
	nodes := a.GetNodes()
	assert.Equal(t, "1", nodes[0].GetId())
	assert.Equal(t, "T", nodes[0].GetTitle())
	assert.Equal(t, "ENG-1", nodes[0].GetIdentifier())
	assert.Equal(t, float64(2), nodes[0].GetPriority())
	assert.Equal(t, "2024-01-01", nodes[0].GetDueDate())
	assert.Equal(t, "u", nodes[0].GetUrl())
	assert.Equal(t, "2024-01-01", nodes[0].GetUpdatedAt())
	assert.Equal(t, "2024-01-01", nodes[0].GetCompletedAt())
}

func TestGenerated_Getters_InputTypes(t *testing.T) {
	a := &__AssignedIssuesInput{Filter: map[string]interface{}{"f": "v"}, First: 10}
	assert.Equal(t, map[string]interface{}{"f": "v"}, a.GetFilter())
	assert.Equal(t, 10, a.GetFirst())

	sc := &__StateChangesInput{Filter: map[string]interface{}{"f": "v"}, First: 5, HistoryFirst: 3}
	assert.Equal(t, map[string]interface{}{"f": "v"}, sc.GetFilter())
	assert.Equal(t, 5, sc.GetFirst())
	assert.Equal(t, 3, sc.GetHistoryFirst())

	ui := &__UpdatedIssuesWithDetailsInput{Filter: map[string]interface{}{"f": "v"}, First: 5, CommentsFirst: 10}
	assert.Equal(t, map[string]interface{}{"f": "v"}, ui.GetFilter())
	assert.Equal(t, 5, ui.GetFirst())
	assert.Equal(t, 10, ui.GetCommentsFirst())
}

// TestGenerated_Getters_Comprehensive tests all generated getters for full coverage.
func TestGenerated_Getters_Comprehensive(t *testing.T) {
	// AssignedIssuesResponse
	ar := &AssignedIssuesResponse{
		Viewer: AssignedIssuesViewerUser{
			AssignedIssues: AssignedIssuesViewerUserAssignedIssuesIssueConnection{
				Nodes: []AssignedIssuesViewerUserAssignedIssuesIssueConnectionNodesIssue{
					{
						Id: "1", Title: "T", Identifier: "ENG-1", Priority: 2,
						DueDate: "2024-01-01", Url: "u", UpdatedAt: "2024-01-01", CompletedAt: "c",
						Parent: AssignedIssuesViewerUserAssignedIssuesIssueConnectionNodesIssueParentIssue{Id: "p1"},
						State:  AssignedIssuesViewerUserAssignedIssuesIssueConnectionNodesIssueStateWorkflowState{Name: "S", Type: "st"},
						Team:   AssignedIssuesViewerUserAssignedIssuesIssueConnectionNodesIssueTeam{Name: "E", Key: "EK"},
					},
				},
			},
		},
	}
	// Access through pointer chain
	node0 := &ar.Viewer.AssignedIssues.Nodes[0]
	_ = node0.GetId()
	_ = node0.GetTitle()
	_ = node0.GetIdentifier()
	_ = node0.GetPriority()
	_ = node0.GetParent()
	_ = node0.GetState()
	_ = node0.GetTeam()
	_ = node0.GetDueDate()
	_ = node0.GetUrl()
	_ = node0.GetUpdatedAt()
	_ = node0.GetCompletedAt()
	parent0 := &node0.Parent
	_ = parent0.GetId()
	state0 := &node0.State
	_ = state0.GetName()
	_ = state0.GetType()
	team0 := &node0.Team
	_ = team0.GetName()
	_ = team0.GetKey()

	// StateChangesResponse
	scr := &StateChangesResponse{
		Viewer: StateChangesViewerUser{
			AssignedIssues: StateChangesViewerUserAssignedIssuesIssueConnection{
				Nodes: []StateChangesViewerUserAssignedIssuesIssueConnectionNodesIssue{
					{
						Id: "1", Identifier: "ENG-1", Title: "T",
						Url: "u", UpdatedAt: "2024-01-01",
						Team: StateChangesViewerUserAssignedIssuesIssueConnectionNodesIssueTeam{Name: "E", Key: "EK"},
						History: StateChangesViewerUserAssignedIssuesIssueConnectionNodesIssueHistoryIssueHistoryConnection{
							Nodes: []StateChangesViewerUserAssignedIssuesIssueConnectionNodesIssueHistoryIssueHistoryConnectionNodesIssueHistory{
								{
									FromState: StateChangesViewerUserAssignedIssuesIssueConnectionNodesIssueHistoryIssueHistoryConnectionNodesIssueHistoryFromStateWorkflowState{Name: "A"},
									ToState:   StateChangesViewerUserAssignedIssuesIssueConnectionNodesIssueHistoryIssueHistoryConnectionNodesIssueHistoryToStateWorkflowState{Name: "B"},
									CreatedAt: "2024-01-01",
								},
							},
						},
					},
				},
			},
		},
	}
	scNode := &scr.Viewer.AssignedIssues.Nodes[0]
	_ = scNode.GetId()
	_ = scNode.GetIdentifier()
	_ = scNode.GetTitle()
	_ = scNode.GetTeam()
	_ = scNode.GetUrl()
	_ = scNode.GetUpdatedAt()
	_ = scNode.GetHistory()
	scTeam := &scNode.Team
	_ = scTeam.GetName()
	_ = scTeam.GetKey()
	histNode := &scNode.History.Nodes[0]
	_ = histNode.GetFromState()
	_ = histNode.GetToState()
	_ = histNode.GetCreatedAt()
	fromState := &histNode.FromState
	_ = fromState.GetName()
	toState := &histNode.ToState
	_ = toState.GetName()

	// UpdatedIssuesWithDetailsResponse
	uir := &UpdatedIssuesWithDetailsResponse{
		Viewer: UpdatedIssuesWithDetailsViewerUser{
			AssignedIssues: UpdatedIssuesWithDetailsViewerUserAssignedIssuesIssueConnection{
				Nodes: []UpdatedIssuesWithDetailsViewerUserAssignedIssuesIssueConnectionNodesIssue{
					{
						Id: "1", Identifier: "ENG-1", Title: "T", Description: "D",
						Priority: 2, Url: "u", CompletedAt: "c", UpdatedAt: "2024-01-01",
						State:  UpdatedIssuesWithDetailsViewerUserAssignedIssuesIssueConnectionNodesIssueStateWorkflowState{Name: "S", Type: "st"},
						Team:   UpdatedIssuesWithDetailsViewerUserAssignedIssuesIssueConnectionNodesIssueTeam{Name: "E", Key: "EK"},
						Parent: UpdatedIssuesWithDetailsViewerUserAssignedIssuesIssueConnectionNodesIssueParentIssue{Id: "p1", Identifier: "ENG-0"},
						Comments: UpdatedIssuesWithDetailsViewerUserAssignedIssuesIssueConnectionNodesIssueCommentsCommentConnection{
							Nodes: []UpdatedIssuesWithDetailsViewerUserAssignedIssuesIssueConnectionNodesIssueCommentsCommentConnectionNodesComment{
								{Body: "b", CreatedAt: "2024-01-01", User: UpdatedIssuesWithDetailsViewerUserAssignedIssuesIssueConnectionNodesIssueCommentsCommentConnectionNodesCommentUser{Name: "U"}},
							},
						},
					},
				},
			},
		},
	}
	uiNode := &uir.Viewer.AssignedIssues.Nodes[0]
	_ = uiNode.GetId()
	_ = uiNode.GetIdentifier()
	_ = uiNode.GetTitle()
	_ = uiNode.GetDescription()
	_ = uiNode.GetPriority()
	_ = uiNode.GetUrl()
	_ = uiNode.GetCompletedAt()
	_ = uiNode.GetUpdatedAt()
	_ = uiNode.GetState()
	_ = uiNode.GetTeam()
	_ = uiNode.GetParent()
	_ = uiNode.GetComments()
	uiState := &uiNode.State
	_ = uiState.GetName()
	_ = uiState.GetType()
	uiTeam := &uiNode.Team
	_ = uiTeam.GetName()
	_ = uiTeam.GetKey()
	uiParent := &uiNode.Parent
	_ = uiParent.GetId()
	_ = uiParent.GetIdentifier()
	commentNode := &uiNode.Comments.Nodes[0]
	_ = commentNode.GetBody()
	_ = commentNode.GetCreatedAt()
	_ = commentNode.GetUser()
	commentUser := &commentNode.User
	_ = commentUser.GetName()

	// Call top-level response getters (pointer receivers on response types)
	_ = ar.GetViewer()
	aiConn := &ar.Viewer.AssignedIssues
	_ = aiConn.GetNodes()

	_ = scr.GetViewer()
	scConn := &scr.Viewer.AssignedIssues
	_ = scConn.GetNodes()
	histConn := &scNode.History
	_ = histConn.GetNodes()

	_ = uir.GetViewer()
	uiConn := &uir.Viewer.AssignedIssues
	_ = uiConn.GetNodes()
	cmntConn := &uiNode.Comments
	_ = cmntConn.GetNodes()

	// Verify a few values to ensure the getters work
	assert.Equal(t, "1", node0.GetId())
	assert.Equal(t, "p1", parent0.GetId())
	assert.Equal(t, "S", state0.GetName())
	assert.Equal(t, "E", team0.GetName())
	assert.Equal(t, "1", scNode.GetId())
	assert.Equal(t, "A", fromState.GetName())
	assert.Equal(t, "1", uiNode.GetId())
	assert.Equal(t, "ENG-0", uiParent.GetIdentifier())
	assert.Equal(t, "b", commentNode.GetBody())
	assert.Equal(t, "U", commentUser.GetName())
}
