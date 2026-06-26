package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIssueDetailView(t *testing.T) {
	d := IssueDetail{
		Identifier:  "LUC-1",
		Title:       "Test",
		Description: "desc",
		StateName:   "Done",
		TeamName:    "Eng",
		URL:         "https://example.com/1",
		Priority:    "P0",
		Comments: []Comment{
			{Body: "comment", UserName: "alice", CreatedAt: "2024-01-01"},
		},
	}
	assert.Equal(t, "LUC-1", d.Identifier)
	assert.Len(t, d.Comments, 1)
}
