package cmd

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/internal/linear"
)

func TestPriorityLabel(t *testing.T) {
	tests := []struct {
		want string
		p    float64
	}{
		{"🔥 P0", 1},
		{"🔴 P1", 2},
		{"⚡ P2", 3},
		{"📋 P3", 4},
		{"", 0},
		{"", 5},
		{"", 99},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, priorityLabel(tt.p))
		})
	}
}

func TestToIssueViews(t *testing.T) {
	issues := []linear.Issue{
		{Identifier: "LUC-1", Title: "Task 1", Priority: 1, TeamName: "Eng", DueDate: "2026-01-01", URL: "https://example.com/1"},
		{Identifier: "LUC-2", Title: "Task 2", Priority: 3, TeamName: "Ops"},
	}
	views := toIssueViews(issues)
	require.Len(t, views, 2)
	assert.Equal(t, "LUC-1", views[0].Identifier)
	assert.Equal(t, "🔥 P0", views[0].Priority)
	assert.Equal(t, "2026-01-01", views[0].DueDate)
	assert.Equal(t, "LUC-2", views[1].Identifier)
	assert.Equal(t, "⚡ P2", views[1].Priority)
	assert.Empty(t, views[1].DueDate)
}

func TestToStateChangeViews(t *testing.T) {
	changes := []linear.StateChange{
		{IssueIdentifier: "LUC-1", IssueTitle: "Task 1", FromState: "Todo", ToState: "Done", TeamName: "Eng", URL: "https://example.com/1"},
	}
	views := toStateChangeViews(changes)
	require.Len(t, views, 1)
	assert.Equal(t, "LUC-1", views[0].IssueIdentifier)
	assert.Equal(t, "Todo", views[0].FromState)
	assert.Equal(t, "Done", views[0].ToState)
}

func TestToIssueDetails(t *testing.T) {
	details := []linear.IssueDetail{
		{
			Identifier: "LUC-1", Title: "Task 1", Description: "desc",
			StateName: "Done", TeamName: "Eng", URL: "https://example.com/1",
			Priority: 2,
			Comments: []linear.Comment{{Body: "comment1", UserName: "Alice", CreatedAt: "2026-01-01"}},
		},
	}
	result := toIssueDetails(details)
	require.Len(t, result, 1)
	assert.Equal(t, "LUC-1", result[0].Identifier)
	assert.Equal(t, "🔴 P1", result[0].Priority)
	require.Len(t, result[0].Comments, 1)
	assert.Equal(t, "comment1", result[0].Comments[0].Body)
}

func TestNewMorningCmdHasFlags(t *testing.T) {
	cmd := newMorningCmd()
	assert.Equal(t, "morning", cmd.Use)
	f := cmd.Flags()
	require.NotNil(t, f.Lookup("config"))
	require.NotNil(t, f.Lookup("dry-run"))
}

func TestNewEveningCmdHasFlags(t *testing.T) {
	cmd := newEveningCmd()
	assert.Equal(t, "evening", cmd.Use)
	f := cmd.Flags()
	require.NotNil(t, f.Lookup("config"))
	require.NotNil(t, f.Lookup("dry-run"))
}

func TestNewExportCmdHasFlags(t *testing.T) {
	cmd := newExportCmd()
	assert.Equal(t, "export", cmd.Use)
	f := cmd.Flags()
	require.NotNil(t, f.Lookup("config"))
	require.NotNil(t, f.Lookup("days"))
	require.NotNil(t, f.Lookup("format"))
	require.NotNil(t, f.Lookup("output"))
}

func TestNewReviewCmdHasFlags(t *testing.T) {
	cmd := newReviewCmd()
	assert.Equal(t, "review", cmd.Use)
	f := cmd.Flags()
	require.NotNil(t, f.Lookup("config"))
	require.NotNil(t, f.Lookup("issue"))
	require.NotNil(t, f.Lookup("owner"))
	require.NotNil(t, f.Lookup("repo"))
	require.NotNil(t, f.Lookup("dry-run"))
}

func TestWriteOutputCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/test.txt"
	err := writeOutput([]byte("hello"), path)
	require.NoError(t, err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(data))
}

func TestWriteHTMLCreatesFile(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { os.Chdir(origDir) })

	err := writeHTML("<h1>test</h1>", "test")
	require.NoError(t, err)
	data, err := os.ReadFile("linear2nl_test.html")
	require.NoError(t, err)
	assert.Contains(t, string(data), "<h1>test</h1>")
}

func TestExportJSONWritesFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/export.json"
	err := exportJSON([]linear.IssueDetail{
		{Identifier: "LUC-1", Title: "Task 1", StateName: "Done", StateType: "completed", TeamName: "Eng", TeamKey: "ENG", URL: "https://example.com/1", UpdatedAt: "2026-01-01", Priority: 1},
	}, path, "20260101")
	require.NoError(t, err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "LUC-1")
}

func TestExportMarkdownWritesFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/export.md"
	err := exportMarkdown([]linear.IssueDetail{
		{Identifier: "LUC-1", Title: "Task 1", StateName: "Done", StateType: "completed", TeamName: "Eng", TeamKey: "ENG", URL: "https://example.com/1", UpdatedAt: "2026-01-01", Description: "A description", Comments: []linear.Comment{{Body: "nice", UserName: "Bob", CreatedAt: "2026-01-02"}}},
	}, path, "20260101")
	require.NoError(t, err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	md := string(data)
	assert.Contains(t, md, "# Linear Export")
	assert.Contains(t, md, "LUC-1 Task 1")
	assert.Contains(t, md, "A description")
	assert.Contains(t, md, "nice")
}

func captureStdout(t *testing.T) func() string {
	t.Helper()

	original := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	return func() string {
		require.NoError(t, w.Close())
		os.Stdout = original
		data, err := io.ReadAll(r)
		require.NoError(t, err)
		require.NoError(t, r.Close())

		return string(data)
	}
}

func TestToIssueViewsFromDetails(t *testing.T) {
	details := []linear.IssueDetail{
		{Identifier: "LUC-1", Title: "Task 1", Priority: 1, TeamName: "Eng", URL: "https://example.com/1"},
	}
	views := toIssueViewsFromDetails(details)
	require.Len(t, views, 1)
	assert.Equal(t, "LUC-1", views[0].Identifier)
	assert.Equal(t, "🔥 P0", views[0].Priority)
}

func TestFilterActiveDetails(t *testing.T) {
	completed := []linear.Issue{{Identifier: "LUC-1"}}
	changes := []linear.StateChange{{IssueIdentifier: "LUC-2"}}
	details := []linear.IssueDetail{
		{Identifier: "LUC-1"},
		{Identifier: "LUC-2"},
		{Identifier: "LUC-3"},
	}
	filtered := filterActiveDetails(completed, changes, details)
	assert.Len(t, filtered, 2)
}

func TestParsePerIssueReviewJSON(t *testing.T) {
	raw := `{"reviews":[{"identifier":"LUC-1","title":"Task","progress":["did stuff"],"knowledge":["learned"],"review":["looks good"]}]}`
	result := parsePerIssueReviewJSON(raw)
	require.NotNil(t, result)
	require.Contains(t, result.reviews, "LUC-1")
	assert.Contains(t, result.reviews["LUC-1"], "did stuff")
}

func TestParsePerIssueReviewJSONInvalid(t *testing.T) {
	result := parsePerIssueReviewJSON("not json")
	assert.Nil(t, result)
}

func TestParseAIReviewJSON(t *testing.T) {
	raw := `{"reviews":[{"identifier":"LUC-1","title":"Task"}],"summary":["done"]}`
	result, err := parseAIReviewJSON(raw)
	require.NoError(t, err)
	require.Len(t, result.Reviews, 1)
	assert.Equal(t, "LUC-1", result.Reviews[0].Identifier)
}

func TestParseAIReviewJSONInvalid(t *testing.T) {
	_, err := parseAIReviewJSON("not json")
	assert.Error(t, err)
}

func TestParsePlanJSON(t *testing.T) {
	raw := `{"reviews":[{"identifier":"LUC-1","title":"Task","context":["ctx"],"bottleneck":["bn"],"advice":["adv"]}]}`
	plans := parsePlanJSON(raw)
	require.NotNil(t, plans)
	require.Contains(t, plans, "LUC-1")
	assert.Contains(t, plans["LUC-1"], "ctx")
}

func TestParsePlanJSONInvalid(t *testing.T) {
	plans := parsePlanJSON("not json")
	assert.Nil(t, plans)
}
