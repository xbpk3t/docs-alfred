package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

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
