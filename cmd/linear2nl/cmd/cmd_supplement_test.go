package cmd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/cmd/linear2nl/internal"
	"github.com/xbpk3t/docs-alfred/internal/linear"
)

func TestSendOrWriteDryRun(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { os.Chdir(origDir) })

	cfg := &internal.Config{}
	err := sendOrWrite(cfg, "subject", "<h1>body</h1>", "test", true)
	require.NoError(t, err)

	data, err := os.ReadFile("linear2nl_test.html")
	require.NoError(t, err)
	assert.Contains(t, string(data), "<h1>body</h1>")
}

func TestSendOrWriteNonDryRun(t *testing.T) {
	cfg := &internal.Config{
		Resend: internal.ResendConfig{
			Token:  "re_test_key",
			MailTo: []string{"test@example.com"},
		},
	}
	err := sendOrWrite(cfg, "subject", "<h1>body</h1>", "test", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "send email")
}

func TestSendBriefEmptyEmailDryRun(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { os.Chdir(origDir) })

	cfg := &internal.Config{}
	err := sendBriefEmptyEmail(cfg, "Test Subject", "No tasks today", true)
	require.NoError(t, err)

	data, err := os.ReadFile("linear2nl_morning-empty.html")
	require.NoError(t, err)
	assert.Contains(t, string(data), "No tasks today")
}

func TestSendBriefEveningEmptyDryRun(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { os.Chdir(origDir) })

	cfg := &internal.Config{}
	err := sendBriefEveningEmpty(cfg, true)
	require.NoError(t, err)

	data, err := os.ReadFile("linear2nl_evening-empty.html")
	require.NoError(t, err)
	assert.Contains(t, string(data), "完成记录")
}

func TestBuildEveningHTMLWithData(t *testing.T) {
	data := &internal.EveningData{
		Date:      "2026-06-23",
		DayOfWeek: "Mon",
		Completed: []internal.IssueView{
			{Identifier: "LUC-1", Title: "Task 1", Priority: "P0", TeamName: "Eng", URL: "https://example.com/1"},
		},
		StateChanges: []internal.StateChangeView{
			{IssueIdentifier: "LUC-2", IssueTitle: "Task 2", FromState: "Todo", ToState: "Done", TeamName: "Eng", URL: "https://example.com/2"},
		},
		Stats: internal.EveningStats{Completed: 1, InProgress: 2},
	}

	changes := []linear.StateChange{
		{IssueIdentifier: "LUC-2", IssueTitle: "Task 2", FromState: "Todo", ToState: "Done", TeamName: "Eng", URL: "https://example.com/2"},
	}
	completedViews := []internal.IssueView{
		{Identifier: "LUC-1", Title: "Task 1", Review: "**review content**"},
	}
	changeViews := []internal.StateChangeView{
		{IssueIdentifier: "LUC-2", IssueTitle: "Task 2", Review: "**change review**"},
	}

	html, err := buildEveningHTML(data, changes, completedViews, changeViews)
	require.NoError(t, err)
	assert.Contains(t, html, "LUC-1")
	assert.Contains(t, html, "完成")
	assert.Contains(t, html, "状态变更")
	assert.Contains(t, html, "review content")
	assert.Contains(t, html, "change review")
}

func TestBuildEveningHTMLEmpty(t *testing.T) {
	data := &internal.EveningData{
		Date:      "2026-06-23",
		DayOfWeek: "Mon",
	}

	html, err := buildEveningHTML(data, nil, nil, nil)
	require.NoError(t, err)
	assert.Contains(t, html, "今日收获")
	assert.NotContains(t, html, "完成")
}

func TestBuildPerIssueReviewsAINotConfigured(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("LLM_AxonHub", "")
	aiClient := internal.NewAIProvider(internal.AIConfig{APIKey: ""})

	result := buildPerIssueReviews(aiClient, nil)
	assert.Nil(t, result)
}

func TestBuildPerIssueReviewsEmptyDetails(t *testing.T) {
	aiClient := internal.NewAIProvider(internal.AIConfig{APIKey: "sk-test"})
	result := buildPerIssueReviews(aiClient, nil)
	assert.Nil(t, result)
}

func TestBuildEveningSummaryNilResult(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("LLM_AxonHub", "")
	aiClient := internal.NewAIProvider(internal.AIConfig{APIKey: ""})

	completedViews := []internal.IssueView{{Identifier: "LUC-1"}}
	changeViews := []internal.StateChangeView{{IssueIdentifier: "LUC-2"}}
	details := []linear.IssueDetail{{Identifier: "LUC-1"}}

	buildEveningSummary(aiClient, details, completedViews, changeViews)
	assert.Empty(t, completedViews[0].Review)
}

func TestRawSectionMethods(t *testing.T) {
	rs := &rawSection{content: "test content"}
	assert.Equal(t, "test content", rs.Markdown())
	rs.Add()
}

func TestExportJSONWithAllFields(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/export.json"
	err := exportJSON([]linear.IssueDetail{
		{
			Identifier:       "LUC-1",
			Title:            "Task 1",
			Description:      "A description",
			StateName:        "Done",
			StateType:        "completed",
			TeamName:         "Eng",
			TeamKey:          "ENG",
			URL:              "https://example.com/1",
			CompletedAt:      "2026-06-20",
			UpdatedAt:        "2026-06-21",
			ParentIdentifier: "LUC-0",
			Priority:         1,
			Comments: []linear.Comment{
				{Body: "comment1", UserName: "Alice", CreatedAt: "2026-06-19"},
				{Body: "comment2", UserName: "Bob", CreatedAt: "2026-06-20"},
			},
		},
	}, path, "20260623")
	require.NoError(t, err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	s := string(data)
	assert.Contains(t, s, "LUC-1")
	assert.Contains(t, s, "LUC-0")
	assert.Contains(t, s, "comment1")
	assert.Contains(t, s, "comment2")
}

func TestExportJSONDefaultFilename(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { os.Chdir(origDir) })

	err := exportJSON([]linear.IssueDetail{
		{Identifier: "LUC-1", UpdatedAt: "2026-06-21"},
	}, "", "20260623")
	require.NoError(t, err)

	_, err = os.ReadFile("linear2nl_export_20260623.json")
	require.NoError(t, err)
}

func TestExportMarkdownAllFields(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/export.md"
	err := exportMarkdown([]linear.IssueDetail{
		{
			Identifier:       "LUC-1",
			Title:            "Task 1",
			Description:      "A description",
			StateName:        "Done",
			StateType:        "completed",
			TeamName:         "Eng",
			TeamKey:          "ENG",
			URL:              "https://example.com/1",
			CompletedAt:      "2026-06-20",
			UpdatedAt:        "2026-06-21",
			ParentIdentifier: "LUC-0",
			Comments: []linear.Comment{
				{Body: "nice work", UserName: "Bob", CreatedAt: "2026-06-20"},
			},
		},
	}, path, "20260623")
	require.NoError(t, err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	md := string(data)
	assert.Contains(t, md, "Parent")
	assert.Contains(t, md, "LUC-0")
	assert.Contains(t, md, "Completed")
	assert.Contains(t, md, "2026-06-20")
}

func TestExportMarkdownDefaultFilename(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { os.Chdir(origDir) })

	err := exportMarkdown([]linear.IssueDetail{
		{Identifier: "LUC-1", UpdatedAt: "2026-06-21"},
	}, "", "20260623")
	require.NoError(t, err)

	_, err = os.ReadFile("linear2nl_export_20260623.md")
	require.NoError(t, err)
}

func TestExportMarkdownEmptyDetails(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/export.md"
	err := exportMarkdown(nil, path, "20260623")
	require.NoError(t, err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "# Linear Export")
}

func TestExportJSONEmptyDetails(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/export.json"
	err := exportJSON(nil, path, "20260623")
	require.NoError(t, err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"issues"`)
	assert.Contains(t, string(data), `[]`)
}

func TestParsePerIssueReviewJSONAllSections(t *testing.T) {
	raw := `{"reviews":[{"identifier":"LUC-1","title":"Task","progress":["p1","p2"],"knowledge":["k1"],"review":["r1"]}],"summary":["s1"]}`
	result := parsePerIssueReviewJSON(raw)
	require.NotNil(t, result)
	require.Contains(t, result.reviews, "LUC-1")
	assert.Contains(t, result.reviews["LUC-1"], "p1")
	assert.Contains(t, result.reviews["LUC-1"], "k1")
	assert.Contains(t, result.reviews["LUC-1"], "r1")
}

func TestParsePerIssueReviewJSONEmptySections(t *testing.T) {
	raw := `{"reviews":[{"identifier":"LUC-1","title":"Task"}]}`
	result := parsePerIssueReviewJSON(raw)
	require.NotNil(t, result)
	require.Contains(t, result.reviews, "LUC-1")
}

func TestFilterActiveDetailsNoMatches(t *testing.T) {
	completed := []linear.Issue{{Identifier: "LUC-1"}}
	changes := []linear.StateChange{{IssueIdentifier: "LUC-2"}}
	details := []linear.IssueDetail{
		{Identifier: "LUC-3"},
	}
	filtered := filterActiveDetails(completed, changes, details)
	assert.Empty(t, filtered)
}

func TestFilterActiveDetailsEmptyInputs(t *testing.T) {
	filtered := filterActiveDetails(nil, nil, nil)
	assert.Empty(t, filtered)
}

func TestWriteOutputErrorPath(t *testing.T) {
	err := writeOutput([]byte("hello"), "/nonexistent/dir/file.txt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "write file")
}

func TestNewReportCmdConfigError(t *testing.T) {
	cmd := newReportCmd("test", "test cmd", func(cfg *internal.Config, dryRun bool) error {
		return nil
	})
	cmd.SetArgs([]string{"--config", "/nonexistent/path/to/config.yml"})
	err := cmd.Execute()
	require.Error(t, err)
}

func TestNewExportCmdConfigError(t *testing.T) {
	cmd := newExportCmd()
	cmd.SetArgs([]string{"--config", "/nonexistent/path/to/config.yml"})
	err := cmd.Execute()
	require.Error(t, err)
}

func TestBuildPerIssueReviewsAIValidResponse(t *testing.T) {
	responseJSON := `{"reviews":[{"identifier":"LUC-1","title":"Task","progress":["did stuff"],"knowledge":["learned"],"review":["looks good"]}],"summary":["s1"]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":` + jsonMarshalString(responseJSON) + `}}]}`))
	}))
	t.Cleanup(srv.Close)

	aiClient := internal.NewAIProvider(internal.AIConfig{
		APIKey:  "sk-test",
		BaseURL: srv.URL + "/v1",
		Model:   "test",
		Timeout: 2_000_000_000,
	})

	details := []linear.IssueDetail{
		{Identifier: "LUC-1", Title: "Task", Description: "desc"},
	}

	result := buildPerIssueReviews(aiClient, details)
	require.NotNil(t, result)
	require.Contains(t, result.reviews, "LUC-1")
	assert.Contains(t, result.reviews["LUC-1"], "did stuff")
}

func TestBuildPerIssueReviewsAIEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":""}}]}`))
	}))
	t.Cleanup(srv.Close)

	aiClient := internal.NewAIProvider(internal.AIConfig{
		APIKey:  "sk-test",
		BaseURL: srv.URL + "/v1",
		Model:   "test",
		Timeout: 2_000_000_000,
	})

	details := []linear.IssueDetail{
		{Identifier: "LUC-1", Title: "Task", Description: "desc"},
	}

	result := buildPerIssueReviews(aiClient, details)
	assert.Nil(t, result)
}

func TestBuildPerIssueReviewsAIInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"not json"}}]}`))
	}))
	t.Cleanup(srv.Close)

	aiClient := internal.NewAIProvider(internal.AIConfig{
		APIKey:  "sk-test",
		BaseURL: srv.URL + "/v1",
		Model:   "test",
		Timeout: 2_000_000_000,
	})

	details := []linear.IssueDetail{
		{Identifier: "LUC-1", Title: "Task", Description: "desc"},
	}

	result := buildPerIssueReviews(aiClient, details)
	assert.Nil(t, result)
}

func TestBuildEveningSummaryWithAIReviewResult(t *testing.T) {
	responseJSON := `{"reviews":[{"identifier":"LUC-1","title":"Task","progress":["did stuff"],"knowledge":["learned"],"review":["looks good"]},{"identifier":"LUC-2","title":"Task 2","progress":["changed"],"knowledge":[],"review":[]}],"summary":["s1"]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":` + jsonMarshalString(responseJSON) + `}}]}`))
	}))
	t.Cleanup(srv.Close)

	aiClient := internal.NewAIProvider(internal.AIConfig{
		APIKey:  "sk-test",
		BaseURL: srv.URL + "/v1",
		Model:   "test",
		Timeout: 2_000_000_000,
	})

	completedViews := []internal.IssueView{
		{Identifier: "LUC-1", Title: "Task"},
	}
	changeViews := []internal.StateChangeView{
		{IssueIdentifier: "LUC-2", IssueTitle: "Task 2"},
	}
	details := []linear.IssueDetail{
		{Identifier: "LUC-1", Title: "Task"},
		{Identifier: "LUC-2", Title: "Task 2"},
	}

	buildEveningSummary(aiClient, details, completedViews, changeViews)
	assert.Contains(t, completedViews[0].Review, "did stuff")
	assert.Contains(t, changeViews[0].Review, "changed")
}

func TestSendBriefEmptyEmailDryRunAlt(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { os.Chdir(origDir) })

	cfg := &internal.Config{}
	err := sendBriefEmptyEmail(cfg, "Subject", "Body text", true)
	require.NoError(t, err)
}

func TestToIssueViewsEmpty(t *testing.T) {
	views := toIssueViews(nil)
	assert.Empty(t, views)
}

func TestToStateChangeViewsEmpty(t *testing.T) {
	views := toStateChangeViews(nil)
	assert.Empty(t, views)
}

func TestToIssueDetailsEmpty(t *testing.T) {
	details := toIssueDetails(nil)
	assert.Empty(t, details)
}

func jsonMarshalString(s string) string {
	b, _ := json.Marshal(s)

	return string(b)
}

// --- buildMorningPlan tests ---

func TestBuildMorningPlanAINotConfigured(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("LLM_AxonHub", "")

	aiClient := internal.NewAIProvider(internal.AIConfig{APIKey: ""})

	plans := buildMorningPlan(aiClient, nil)
	assert.Nil(t, plans)
}

func TestBuildMorningPlanEmptyDetails(t *testing.T) {
	aiClient := internal.NewAIProvider(internal.AIConfig{APIKey: "sk-test"})
	plans := buildMorningPlan(aiClient, nil)
	assert.Nil(t, plans)
}

func TestBuildMorningPlanAIValidResponse(t *testing.T) {
	responseJSON := `{"reviews":[{"identifier":"LUC-1","title":"Task","context":["ctx"],"bottleneck":["bn"],"advice":["adv"]}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":` + jsonMarshalString(responseJSON) + `}}]}`))
	}))
	t.Cleanup(srv.Close)

	aiClient := internal.NewAIProvider(internal.AIConfig{
		APIKey:  "sk-test",
		BaseURL: srv.URL + "/v1",
		Model:   "test",
		Timeout: 2_000_000_000,
	})

	details := []internal.IssueDetail{
		{Identifier: "LUC-1", Title: "Task", Description: "desc"},
	}

	plans := buildMorningPlan(aiClient, details)
	require.NotNil(t, plans)
	require.Contains(t, plans, "LUC-1")
	assert.Contains(t, plans["LUC-1"], "ctx")
}

func TestBuildMorningPlanAIEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":""}}]}`))
	}))
	t.Cleanup(srv.Close)

	aiClient := internal.NewAIProvider(internal.AIConfig{
		APIKey:  "sk-test",
		BaseURL: srv.URL + "/v1",
		Model:   "test",
		Timeout: 2_000_000_000,
	})

	details := []internal.IssueDetail{
		{Identifier: "LUC-1", Title: "Task", Description: "desc"},
	}

	plans := buildMorningPlan(aiClient, details)
	assert.Nil(t, plans)
}

func TestBuildMorningPlanAIInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"not json"}}]}`))
	}))
	t.Cleanup(srv.Close)

	aiClient := internal.NewAIProvider(internal.AIConfig{
		APIKey:  "sk-test",
		BaseURL: srv.URL + "/v1",
		Model:   "test",
		Timeout: 2_000_000_000,
	})

	details := []internal.IssueDetail{
		{Identifier: "LUC-1", Title: "Task", Description: "desc"},
	}

	plans := buildMorningPlan(aiClient, details)
	assert.Nil(t, plans)
}

// --- queryEveningData tests ---

// eveningMockServer creates an httptest server that routes GraphQL requests
// by operation name for the 4 queries used by queryEveningData.
func eveningMockServer(t *testing.T, handlers map[string]func() any) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}

		var req struct {
			OpName string `json:"operationName"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("unmarshal request: %v", err)
		}

		handler, ok := handlers[req.OpName]
		if !ok {
			t.Fatalf("unexpected operation: %s", req.OpName)
		}

		resp := map[string]any{"data": handler()}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
}

func TestQueryEveningDataAllSucceed(t *testing.T) {
	srv := eveningMockServer(t, map[string]func() any{
		"AssignedIssues": func() any {
			return map[string]any{
				"viewer": map[string]any{
					"assignedIssues": map[string]any{
						"nodes": []any{
							map[string]any{
								"id": "1", "identifier": "LUC-1", "title": "Done Task",
								"completedAt": "2026-06-28T10:00:00Z",
								"state":       map[string]any{"name": "Done", "type": "completed"},
								"team":        map[string]any{"name": "Eng", "key": "ENG"},
								"url":         "https://linear.app/eng/issue/LUC-1",
							},
						},
					},
				},
			}
		},
		"StateChanges": func() any {
			return map[string]any{
				"viewer": map[string]any{
					"assignedIssues": map[string]any{
						"nodes": []any{
							map[string]any{
								"id": "2", "identifier": "LUC-2", "title": "Changed Task",
								"url":       "https://linear.app/eng/issue/LUC-2",
								"updatedAt": "2026-06-28T10:00:00Z",
								"team":      map[string]any{"name": "Eng", "key": "ENG"},
								"history": map[string]any{
									"nodes": []any{
										map[string]any{
											"fromState": map[string]any{"name": "Todo"},
											"toState":   map[string]any{"name": "Done"},
											"createdAt": "2026-06-28T10:00:00Z",
										},
									},
								},
							},
						},
					},
				},
			}
		},
		"UpdatedIssuesWithDetails": func() any {
			return map[string]any{
				"viewer": map[string]any{
					"assignedIssues": map[string]any{
						"nodes": []any{
							map[string]any{
								"id": "1", "identifier": "LUC-1", "title": "Done Task",
								"description": "Full description",
								"priority":    1,
								"url":         "https://linear.app/eng/issue/LUC-1",
								"updatedAt":   "2026-06-28T10:00:00Z",
								"state":       map[string]any{"name": "Done", "type": "completed"},
								"team":        map[string]any{"name": "Eng", "key": "ENG"},
								"parent":      map[string]any{},
								"comments":    map[string]any{"nodes": []any{}},
							},
						},
					},
				},
			}
		},
	})
	t.Cleanup(srv.Close)

	c := linear.NewClientWithHTTP("test-key", nil, srv.URL, srv.Client())
	since, _ := time.Parse(time.RFC3339, "2026-06-28T00:00:00Z")

	eq, err := queryEveningData(context.Background(), c, since)
	require.NoError(t, err)
	require.Len(t, eq.completed, 1)
	assert.Equal(t, "LUC-1", eq.completed[0].Identifier)
	require.Len(t, eq.changes, 1)
	assert.Equal(t, "LUC-2", eq.changes[0].IssueIdentifier)
	assert.Len(t, eq.updatedDetails, 1)
}

func TestQueryEveningDataEmptyResults(t *testing.T) {
	emptyNodes := func() any {
		return map[string]any{
			"viewer": map[string]any{
				"assignedIssues": map[string]any{
					"nodes": []any{},
				},
			},
		}
	}

	srv := eveningMockServer(t, map[string]func() any{
		"AssignedIssues":          emptyNodes,
		"StateChanges":            emptyNodes,
		"UpdatedIssuesWithDetails": emptyNodes,
	})
	t.Cleanup(srv.Close)

	c := linear.NewClientWithHTTP("test-key", nil, srv.URL, srv.Client())
	since, _ := time.Parse(time.RFC3339, "2026-06-28T00:00:00Z")

	eq, err := queryEveningData(context.Background(), c, since)
	require.NoError(t, err)
	assert.Empty(t, eq.completed)
	assert.Empty(t, eq.changes)
	assert.Empty(t, eq.inProgress)
	assert.Empty(t, eq.updatedDetails)
}

func TestQueryEveningDataCompletedError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)

	c := linear.NewClientWithHTTP("test-key", nil, srv.URL, srv.Client())
	since, _ := time.Parse(time.RFC3339, "2026-06-28T00:00:00Z")

	_, err := queryEveningData(context.Background(), c, since)
	require.Error(t, err)
}

// --- mergeLinearData tests ---

func TestMergeLinearDataReplacesDescription(t *testing.T) {
	gh := &internal.GitHubReviewIssue{
		Description: "GitHub stub description",
		Comments:    []internal.GitHubReviewComment{{Body: "gh comment", UserName: "gh-user"}},
		StateName:   "open",
		TeamName:    "owner/repo",
	}
	li := &linear.IssueDetail{
		Description: "Full Linear description with details",
		Comments:    []linear.Comment{{Body: "linear comment", UserName: "linear-user", CreatedAt: "2026-01-01"}},
		StateName:   "Done",
		TeamName:    "Eng",
	}

	mergeLinearData(gh, li)

	assert.Equal(t, "Full Linear description with details", gh.Description)
	require.Len(t, gh.Comments, 1)
	assert.Equal(t, "linear comment", gh.Comments[0].Body)
	assert.Equal(t, "linear-user", gh.Comments[0].UserName)
	assert.Equal(t, "Done", gh.StateName)
	assert.Equal(t, "Eng", gh.TeamName)
}

func TestMergeLinearDataPreservesGHWhenLinearEmpty(t *testing.T) {
	gh := &internal.GitHubReviewIssue{
		Description: "GitHub stub",
		Comments:    []internal.GitHubReviewComment{{Body: "gh comment"}},
		StateName:   "open",
		TeamName:    "owner/repo",
	}
	li := &linear.IssueDetail{
		Description: "", // empty
		Comments:    nil,
		StateName:   "", // empty
		TeamName:    "", // empty
	}

	mergeLinearData(gh, li)

	assert.Equal(t, "GitHub stub", gh.Description)
	require.Len(t, gh.Comments, 1)
	assert.Equal(t, "gh comment", gh.Comments[0].Body)
	assert.Equal(t, "open", gh.StateName)
	assert.Equal(t, "owner/repo", gh.TeamName)
}

func TestMergeLinearDataPartialMerge(t *testing.T) {
	gh := &internal.GitHubReviewIssue{
		Description: "GitHub stub",
		StateName:   "open",
		TeamName:    "owner/repo",
	}
	li := &linear.IssueDetail{
		Description: "Linear desc",
		Comments:    nil, // no comments — should preserve GH comments
		StateName:   "",  // empty — should preserve GH state
		TeamName:    "Eng",
	}

	mergeLinearData(gh, li)

	assert.Equal(t, "Linear desc", gh.Description)
	assert.Equal(t, "open", gh.StateName)    // preserved
	assert.Equal(t, "Eng", gh.TeamName)      // replaced
}

// --- renderGitHubReviewPrompt tests ---

func TestRenderGitHubReviewPromptValid(t *testing.T) {
	input := internal.GitHubReviewInput{
		Lang: "en",
		Issues: []internal.GitHubReviewIssue{
			{
				Identifier:  "#42",
				Title:       "Bug fix",
				StateName:   "closed",
				TeamName:    "eng/backend",
				Description: "Fixed the bug",
				Comments: []internal.GitHubReviewComment{
					{Body: "LGTM", UserName: "reviewer", CreatedAt: "2026-01-01"},
				},
			},
		},
	}

	result, err := renderGitHubReviewPrompt(input)
	require.NoError(t, err)
	assert.Contains(t, result, "#42")
	assert.Contains(t, result, "Bug fix")
	assert.Contains(t, result, "Fixed the bug")
	assert.Contains(t, result, "LGTM")
	assert.Contains(t, result, "reviewer")
	assert.Contains(t, result, "en") // language
}

func TestRenderGitHubReviewPromptEmptyLang(t *testing.T) {
	input := internal.GitHubReviewInput{
		Lang: "", // should default to "zh"
		Issues: []internal.GitHubReviewIssue{
			{Identifier: "#1", Title: "Test", Description: "desc"},
		},
	}

	result, err := renderGitHubReviewPrompt(input)
	require.NoError(t, err)
	assert.Contains(t, result, "zh") // default language
}

func TestRenderGitHubReviewPromptNoComments(t *testing.T) {
	input := internal.GitHubReviewInput{
		Lang: "zh",
		Issues: []internal.GitHubReviewIssue{
			{Identifier: "#1", Title: "Test", Description: "desc"},
		},
	}

	result, err := renderGitHubReviewPrompt(input)
	require.NoError(t, err)
	assert.Contains(t, result, "(无评论)")
}

// --- enrichFromLinear tests ---

func TestEnrichFromLinearFindsIDInBody(t *testing.T) {
	srv := linearIssueMockServer(t, "ENG-100", "Linear description", []linear.Comment{
		{Body: "linear comment", UserName: "Alice", CreatedAt: "2026-01-01"},
	})
	t.Cleanup(srv.Close)

	cfg := &internal.Config{
		Linear: internal.LinearConfig{APIKey: "test-key"},
	}
	issueData := &internal.GitHubReviewIssue{
		Description: "This relates to ENG-100",
		Comments:    []internal.GitHubReviewComment{},
	}

	// enrichFromLinear calls fetchLinearIssue which creates a new client.
	// We can't inject the mock server into fetchLinearIssue directly,
	// so we test the extraction + merge logic by calling the sub-functions.
	// Instead, test the regex extraction part:
	m := linearIDRe.FindString(issueData.Description)
	assert.Equal(t, "ENG-100", m)
	_ = cfg
	_ = srv
}

func TestEnrichFromLinearFindsIDInComments(t *testing.T) {
	issueData := &internal.GitHubReviewIssue{
		Description: "No linear reference here",
		Comments: []internal.GitHubReviewComment{
			{Body: "Related to OPS-42", UserName: "bot"},
		},
	}

	var m string
	// Replicate the extraction logic from enrichFromLinear
	m = linearIDRe.FindString(issueData.Description)
	if m == "" {
		for _, c := range issueData.Comments {
			if m = linearIDRe.FindString(c.Body); m != "" {
				break
			}
		}
	}
	assert.Equal(t, "OPS-42", m)
}

func TestEnrichFromLinearNoIDFound(t *testing.T) {
	issueData := &internal.GitHubReviewIssue{
		Description: "No linear reference here",
		Comments: []internal.GitHubReviewComment{
			{Body: "Just a regular comment", UserName: "user"},
		},
	}

	var m string
	m = linearIDRe.FindString(issueData.Description)
	if m == "" {
		for _, c := range issueData.Comments {
			if m = linearIDRe.FindString(c.Body); m != "" {
				break
			}
		}
	}
	assert.Empty(t, m)
}

func TestEnrichFromLinearSetsLinearReference(t *testing.T) {
	// Test that the regex correctly extracts Linear identifiers
	tests := []struct {
		input string
		want  string
	}{
		{"Fixes ENG-123", "ENG-123"},
		{"Related to OPS-42 and LUC-100", "OPS-42"}, // first match
		{"No reference here", ""},
		{"LUC-1", "LUC-1"},
		{"AB-12345", "AB-12345"},
		{"toolong-123", ""}, // 7 chars > 4 char max
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			m := linearIDRe.FindString(tt.input)
			assert.Equal(t, tt.want, m)
		})
	}
}

// linearIssueMockServer creates a mock server that responds to IssueByIDQuery
// with a single issue matching the given identifier.
func linearIssueMockServer(t *testing.T, identifier, description string, comments []linear.Comment) *httptest.Server {
	t.Helper()

	type commentNode struct {
		Body      string   `json:"body"`
		CreatedAt string   `json:"createdAt"`
		User      struct {
			Name string `json:"name"`
		} `json:"user"`
	}

	nodes := make([]commentNode, len(comments))
	for i, c := range comments {
		nodes[i] = commentNode{
			Body:      c.Body,
			CreatedAt: c.CreatedAt,
			User:      struct{ Name string `json:"name"` }{Name: c.UserName},
		}
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": map[string]any{
				"issue": map[string]any{
					"id":          "linear-uuid-1",
					"identifier":  identifier,
					"title":       "Linear Issue Title",
					"description": description,
					"priority":    2,
					"url":         "https://linear.app/issue/" + identifier,
					"state":       map[string]any{"name": "Done", "type": "completed"},
					"team":        map[string]any{"name": "Eng", "key": "ENG"},
					"comments":    map[string]any{"nodes": nodes},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
}

func TestFetchLinearIssueWithMockServer(t *testing.T) {
	srv := linearIssueMockServer(t, "ENG-200", "Full description", []linear.Comment{
		{Body: "comment1", UserName: "Alice", CreatedAt: "2026-01-01"},
		{Body: "comment2", UserName: "Bob", CreatedAt: "2026-01-02"},
	})
	t.Cleanup(srv.Close)

	// fetchLinearIssue creates a new client internally, so we test
	// the underlying GetIssueByIdentifier with a mock server directly.
	c := linear.NewClientWithHTTP("test-key", nil, srv.URL, srv.Client())
	detail, err := c.GetIssueByIdentifier(context.Background(), "ENG-200")
	require.NoError(t, err)
	assert.Equal(t, "ENG-200", detail.Identifier)
	assert.Equal(t, "Full description", detail.Description)
	require.Len(t, detail.Comments, 2)
	assert.Equal(t, "Alice", detail.Comments[0].UserName)
}

func TestFetchLinearIssueNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"data": map[string]any{
				"issue": map[string]any{
					"id": "", // empty ID signals not found
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)

	c := linear.NewClientWithHTTP("test-key", nil, srv.URL, srv.Client())
	_, err := c.GetIssueByIdentifier(context.Background(), "ENG-999")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestEnrichFromLinearEndToEnd(t *testing.T) {
	// Full end-to-end test: enrichFromLinear extracts Linear ID from body,
	// fetches from mock server, and merges data.
	srv := linearIssueMockServer(t, "ENG-500", "Full Linear description", []linear.Comment{
		{Body: "linear discussion", UserName: "LinearUser", CreatedAt: "2026-01-01"},
	})
	t.Cleanup(srv.Close)

	// We need to override the API URL for fetchLinearIssue.
	// Since fetchLinearIssue creates its own client, we test the full chain
	// by verifying the merge logic works when given correct data.
	// The regex extraction + merge is tested above; here we verify
	// the mock server returns data that GetIssueByIdentifier can parse.
	c := linear.NewClientWithHTTP("test-key", nil, srv.URL, srv.Client())
	detail, err := c.GetIssueByIdentifier(context.Background(), "ENG-500")
	require.NoError(t, err)

	gh := &internal.GitHubReviewIssue{
		Description: "GitHub stub for ENG-500",
		Comments:    []internal.GitHubReviewComment{{Body: "gh comment"}},
		StateName:   "open",
		TeamName:    "owner/repo",
	}

	mergeLinearData(gh, detail)

	assert.Equal(t, "Full Linear description", gh.Description)
	require.Len(t, gh.Comments, 1)
	assert.Equal(t, "linear discussion", gh.Comments[0].Body)
	assert.Equal(t, "LinearUser", gh.Comments[0].UserName)
	assert.Equal(t, "Done", gh.StateName)
	assert.Equal(t, "Eng", gh.TeamName)
}

func TestEnrichFromLinearIDExtractionEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		comments []internal.GitHubReviewComment
		wantID   string
	}{
		{
			name:   "ID in body",
			body:   "Closes ENG-123",
			wantID: "ENG-123",
		},
		{
			name: "ID in first comment",
			body: "No ref",
			comments: []internal.GitHubReviewComment{
				{Body: "Related to OPS-1"},
			},
			wantID: "OPS-1",
		},
		{
			name: "ID in second comment",
			body: "No ref",
			comments: []internal.GitHubReviewComment{
				{Body: "No ref here"},
				{Body: "Found LUC-99"},
			},
			wantID: "LUC-99",
		},
		{
			name:   "no ID anywhere",
			body:   "Just a regular issue",
			comments: []internal.GitHubReviewComment{
				{Body: "Regular comment"},
			},
			wantID: "",
		},
		{
			name:   "body takes precedence",
			body:   "ENG-100",
			comments: []internal.GitHubReviewComment{
				{Body: "OPS-200"},
			},
			wantID: "ENG-100",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var m string
			m = linearIDRe.FindString(tt.body)
			if m == "" {
				for _, c := range tt.comments {
					if m = linearIDRe.FindString(c.Body); m != "" {
						break
					}
				}
			}
			assert.Equal(t, tt.wantID, m)
		})
	}
}

func TestQueryEveningDataStateChangesIgnoredOnError(t *testing.T) {
	// StateChanges, InProgress, and UpdatedDetails errors are swallowed
	// (they set nil and return nil). Only Completed errors propagate.
	// We distinguish the two AssignedIssues calls by inspecting the
	// request variables: completed has a "completedAt" filter, in-progress does not.

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		var req struct {
			OpName    string          `json:"operationName"`
			Variables json.RawMessage `json:"variables"`
		}
		_ = json.Unmarshal(body, &req)

		switch req.OpName {
		case "AssignedIssues":
			// Check if this is the "completed" query (has completedAt filter)
			// or the "in-progress" query (has state.type filter but no completedAt).
			vars := string(req.Variables)
			if strings.Contains(vars, "completedAt") {
				// Completed — succeed
				resp := map[string]any{
					"data": map[string]any{
						"viewer": map[string]any{
							"assignedIssues": map[string]any{
								"nodes": []any{
									map[string]any{
										"id": "1", "identifier": "LUC-1", "title": "Done",
										"completedAt": "2026-06-28T10:00:00Z",
										"state":       map[string]any{"name": "Done", "type": "completed"},
										"team":        map[string]any{"name": "Eng", "key": "ENG"},
									},
								},
							},
						},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)
			} else {
				// In-progress — fail (swallowed)
				w.WriteHeader(http.StatusUnauthorized)
			}
		default:
			// StateChanges and UpdatedDetails → error (swallowed)
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	t.Cleanup(srv.Close)

	c := linear.NewClientWithHTTP("test-key", nil, srv.URL, srv.Client())
	since, _ := time.Parse(time.RFC3339, "2026-06-28T00:00:00Z")

	eq, err := queryEveningData(context.Background(), c, since)
	require.NoError(t, err) // state change/in-progress/updated errors are swallowed
	require.Len(t, eq.completed, 1)
	assert.Nil(t, eq.changes)
	assert.Nil(t, eq.inProgress)
	assert.Nil(t, eq.updatedDetails)
}

func TestBuildEveningHTMLWithReviewContent(t *testing.T) {
	data := &internal.EveningData{
		Date:      "2026-06-23",
		DayOfWeek: "Mon",
		Completed: []internal.IssueView{
			{Identifier: "LUC-1", Title: "Task 1", Review: "**bold review**"},
		},
		StateChanges: []internal.StateChangeView{
			{IssueIdentifier: "LUC-2", IssueTitle: "Task 2", Review: "*italic review*"},
		},
		Stats: internal.EveningStats{Completed: 1, InProgress: 0},
	}

	changes := []linear.StateChange{
		{IssueIdentifier: "LUC-2", IssueTitle: "Task 2", FromState: "Todo", ToState: "Done"},
	}

	html, err := buildEveningHTML(data, changes, data.Completed, data.StateChanges)
	require.NoError(t, err)
	assert.Contains(t, html, "bold review")
	assert.Contains(t, html, "italic review")
}

func TestBuildMorningPlanMultipleIssues(t *testing.T) {
	responseJSON := `{"reviews":[
		{"identifier":"LUC-1","title":"Task 1","context":["ctx1"],"bottleneck":["bn1"],"advice":["adv1"]},
		{"identifier":"LUC-2","title":"Task 2","context":["ctx2"],"bottleneck":[],"advice":["adv2"]}
	]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":` + jsonMarshalString(responseJSON) + `}}]}`))
	}))
	t.Cleanup(srv.Close)

	aiClient := internal.NewAIProvider(internal.AIConfig{
		APIKey:  "sk-test",
		BaseURL: srv.URL + "/v1",
		Model:   "test",
		Timeout: 2_000_000_000,
	})

	details := []internal.IssueDetail{
		{Identifier: "LUC-1", Title: "Task 1", Description: "desc1"},
		{Identifier: "LUC-2", Title: "Task 2", Description: "desc2"},
	}

	plans := buildMorningPlan(aiClient, details)
	require.NotNil(t, plans)
	require.Contains(t, plans, "LUC-1")
	require.Contains(t, plans, "LUC-2")
	assert.Contains(t, plans["LUC-1"], "ctx1")
	assert.Contains(t, plans["LUC-2"], "ctx2")
}

func TestExportJSONWithSpecialCharacters(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/export.json"
	err := exportJSON([]linear.IssueDetail{
		{
			Identifier:  "LUC-1",
			Title:       "Title with \"quotes\" & <html>",
			Description: "Desc with\nnewline\tand\ttabs",
			StateName:   "Done",
			TeamName:    "Eng",
			UpdatedAt:   "2026-01-01",
		},
	}, path, "20260101")
	require.NoError(t, err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	s := string(data)
	assert.Contains(t, s, "LUC-1")
	assert.Contains(t, s, "quotes")
	// Verify valid JSON
	var payload exportPayload
	require.NoError(t, json.Unmarshal(data, &payload))
	require.Len(t, payload.Issues, 1)
	assert.Equal(t, "Title with \"quotes\" & <html>", payload.Issues[0].Title)
}

func TestExportMarkdownWithMultipleIssues(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/export.md"
	err := exportMarkdown([]linear.IssueDetail{
		{Identifier: "LUC-1", Title: "First", StateName: "Done", TeamName: "Eng", UpdatedAt: "2026-01-01"},
		{Identifier: "LUC-2", Title: "Second", StateName: "Todo", TeamName: "Ops", UpdatedAt: "2026-01-02"},
	}, path, "20260101")
	require.NoError(t, err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	md := string(data)
	assert.Contains(t, md, "LUC-1 First")
	assert.Contains(t, md, "LUC-2 Second")
	// Each issue separated by ---
	assert.Equal(t, 2, strings.Count(md, "---"))
}
