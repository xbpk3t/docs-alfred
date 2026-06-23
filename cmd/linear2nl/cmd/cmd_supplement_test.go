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
	// non-dryRun path calls sendEmail which will fail with empty config,
	// but covers the sendEmail branch in sendOrWrite.
	cfg := &internal.Config{
		Resend: internal.ResendConfig{
			Token:  "re_test_key",
			MailTo: []string{"test@example.com"},
		},
	}
	err := sendOrWrite(cfg, "subject", "<h1>body</h1>", "test", false)
	// sendEmail will fail (invalid token) but the code path is covered
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

	completed := []linear.Issue{
		{Identifier: "LUC-1", Title: "Task 1", TeamName: "Eng", URL: "https://example.com/1"},
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

	html, err := buildEveningHTML(data, completed, changes, completedViews, changeViews)
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

	html, err := buildEveningHTML(data, nil, nil, nil, nil)
	require.NoError(t, err)
	assert.Contains(t, html, "今日收获")
	assert.NotContains(t, html, "完成")
}

func TestBuildMorningGroupsAINotConfigured(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("LLM_AxonHub", "")

	aiClient := internal.NewAIProvider(internal.AIConfig{APIKey: ""})
	views := []internal.IssueView{
		{Identifier: "LUC-1", Title: "Task 1", URL: "https://example.com/1"},
	}

	groups := buildMorningGroups(aiClient, views)
	require.Len(t, groups, 1)
	assert.Equal(t, fallbackGroupName, groups[0].Name)
	assert.Len(t, groups[0].Issues, 1)
}

func TestBuildMorningGroupsEmptyViews(t *testing.T) {
	aiClient := internal.NewAIProvider(internal.AIConfig{APIKey: ""})
	groups := buildMorningGroups(aiClient, nil)
	require.Len(t, groups, 1)
	assert.Empty(t, groups[0].Issues)
}

func TestEnrichActiveGroupsEmptyGroups(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("LLM_AxonHub", "")
	aiClient := internal.NewAIProvider(internal.AIConfig{APIKey: ""})

	groups := []internal.GroupView{
		{Name: "Uncategorized", Issues: []internal.GroupItemView{{Identifier: "LUC-1"}}},
	}
	enrichActiveGroups(aiClient, groups, nil)
	// Should not modify non-active groups
	assert.Empty(t, groups[0].Issues[0].Context)
}

func TestEnrichActiveGroupsWithFIXMEGroup(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("LLM_AxonHub", "")
	aiClient := internal.NewAIProvider(internal.AIConfig{APIKey: ""})

	groups := []internal.GroupView{
		{Name: "FIXME", Issues: []internal.GroupItemView{{Identifier: "LUC-1", Title: "Fix this"}}},
	}
	details := []internal.IssueDetail{
		{Identifier: "LUC-1", Title: "Fix this", Description: "desc"},
	}

	// Without API key, should not crash
	enrichActiveGroups(aiClient, groups, details)
	assert.Equal(t, "LUC-1", groups[0].Issues[0].Identifier)
}

func TestEnrichActiveGroupsNoMatchingDetails(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("LLM_AxonHub", "")
	aiClient := internal.NewAIProvider(internal.AIConfig{APIKey: ""})

	groups := []internal.GroupView{
		{Name: "FIXME", Issues: []internal.GroupItemView{{Identifier: "LUC-999"}}},
	}
	enrichActiveGroups(aiClient, groups, nil)
	// Should not crash when no matching details
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

	// Should not crash
	buildEveningSummary(aiClient, details, completedViews, changeViews)
	assert.Empty(t, completedViews[0].Review)
}

func TestRawSectionMethods(t *testing.T) {
	rs := &rawSection{content: "test content"}
	assert.Equal(t, "test content", rs.Markdown())
	rs.Add() // should not panic
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

func TestBuildGroupsFromResultNoGroups(t *testing.T) {
	result := &internal.MorningReviewJSON{Groups: nil}
	views := []internal.IssueView{
		{Identifier: "LUC-1", Title: "Task 1", URL: "https://example.com/1"},
	}
	groups := buildGroupsFromResult(result, views)
	require.Len(t, groups, 1)
	assert.Equal(t, fallbackGroupName, groups[0].Name)
	assert.Len(t, groups[0].Issues, 1)
}

func TestToGroupItemsNilAIData(t *testing.T) {
	views := []internal.IssueView{
		{Identifier: "LUC-1", Title: "Task 1"},
	}
	items := toGroupItems(views, nil)
	require.Len(t, items, 1)
	assert.Equal(t, "LUC-1", items[0].Identifier)
	assert.Nil(t, items[0].Context)
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

// --- writeOutput error path ---

func TestWriteOutputErrorPath(t *testing.T) {
	err := writeOutput([]byte("hello"), "/nonexistent/dir/file.txt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "write file")
}

// --- newReportCmd error path ---

func TestNewReportCmdConfigError(t *testing.T) {
	cmd := newReportCmd("test", "test cmd", func(cfg *internal.Config, dryRun bool) error {
		return nil
	})
	cmd.SetArgs([]string{"--config", "/nonexistent/path/to/config.yml"})
	err := cmd.Execute()
	require.Error(t, err)
}

// --- newExportCmd error path ---

func TestNewExportCmdConfigError(t *testing.T) {
	cmd := newExportCmd()
	cmd.SetArgs([]string{"--config", "/nonexistent/path/to/config.yml"})
	err := cmd.Execute()
	require.Error(t, err)
}

// --- buildMorningGroups with AI configured but empty response ---

func TestBuildMorningGroupsAIConfiguredEmptyResponse(t *testing.T) {
	// Use a real HTTP server that returns error to simulate AI failure
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	t.Setenv("OPENAI_API_KEY", "sk-test")
	t.Setenv("OPENAI_BASE_URL", srv.URL+"/v1")
	t.Setenv("LLM_MODEL", "test")

	aiClient := internal.NewAIProvider(internal.AIConfig{
		APIKey:  "sk-test",
		BaseURL: srv.URL + "/v1",
		Model:   "test",
		Timeout: 1_000_000_000, // 1s
	})

	views := []internal.IssueView{
		{Identifier: "LUC-1", Title: "Task 1", URL: "https://example.com/1"},
	}

	groups := buildMorningGroups(aiClient, views)
	// AI call fails -> falls back
	require.Len(t, groups, 1)
	assert.Equal(t, fallbackGroupName, groups[0].Name)
}

// --- buildMorningGroups with AI returning invalid JSON ---

func TestBuildMorningGroupsAIInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"not valid json"}}]}`))
	}))
	t.Cleanup(srv.Close)

	aiClient := internal.NewAIProvider(internal.AIConfig{
		APIKey:  "sk-test",
		BaseURL: srv.URL + "/v1",
		Model:   "test",
		Timeout: 2_000_000_000, // 2s
	})

	views := []internal.IssueView{
		{Identifier: "LUC-1", Title: "Task 1", URL: "https://example.com/1"},
	}

	groups := buildMorningGroups(aiClient, views)
	// Invalid JSON -> falls back
	require.Len(t, groups, 1)
	assert.Equal(t, fallbackGroupName, groups[0].Name)
}

// --- buildMorningGroups with AI returning valid JSON ---

func TestBuildMorningGroupsAIValidJSON(t *testing.T) {
	responseJSON := `{"groups":[{"name":"FIXME","issues":[{"identifier":"LUC-1","title":"Fix this","context":["ctx"],"bottleneck":["bn"],"advice":["adv"]}]}]}`
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

	views := []internal.IssueView{
		{Identifier: "LUC-1", Title: "Fix this", URL: "https://example.com/1"},
	}

	groups := buildMorningGroups(aiClient, views)
	require.NotEmpty(t, groups)
	// Should have FIXME group
	found := false
	for _, g := range groups {
		if g.Name == "FIXME" {
			found = true
			require.Len(t, g.Issues, 1)
			assert.Equal(t, "LUC-1", g.Issues[0].Identifier)
			assert.NotEmpty(t, g.Issues[0].Content) // rendered content
		}
	}
	assert.True(t, found, "FIXME group should be present")
}

// --- enrichActiveGroups with AI returning valid data ---

func TestEnrichActiveGroupsAIValidResponse(t *testing.T) {
	responseJSON := `{"reviews":[{"identifier":"LUC-1","title":"Fix this","context":["from AI"],"bottleneck":["bottleneck"],"advice":["do this"]}]}`
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

	groups := []internal.GroupView{
		{Name: "FIXME", Issues: []internal.GroupItemView{{Identifier: "LUC-1", Title: "Fix this"}}},
	}
	details := []internal.IssueDetail{
		{Identifier: "LUC-1", Title: "Fix this", Description: "desc"},
	}

	enrichActiveGroups(aiClient, groups, details)
	assert.Equal(t, []string{"from AI"}, groups[0].Issues[0].Context)
	assert.Equal(t, []string{"bottleneck"}, groups[0].Issues[0].Bottleneck)
	assert.NotEmpty(t, groups[0].Issues[0].Content)
}

func TestEnrichActiveGroupsAIInvalidJSON(t *testing.T) {
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

	groups := []internal.GroupView{
		{Name: "FIXME", Issues: []internal.GroupItemView{{Identifier: "LUC-1", Title: "Fix this"}}},
	}
	details := []internal.IssueDetail{
		{Identifier: "LUC-1", Title: "Fix this", Description: "desc"},
	}

	enrichActiveGroups(aiClient, groups, details)
	// Invalid JSON -> no enrichment
	assert.Empty(t, groups[0].Issues[0].Context)
}

func TestEnrichActiveGroupsAIEmptyResponse(t *testing.T) {
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

	groups := []internal.GroupView{
		{Name: "FIXME", Issues: []internal.GroupItemView{{Identifier: "LUC-1", Title: "Fix this"}}},
	}
	details := []internal.IssueDetail{
		{Identifier: "LUC-1", Title: "Fix this", Description: "desc"},
	}

	enrichActiveGroups(aiClient, groups, details)
	assert.Empty(t, groups[0].Issues[0].Context)
}

func TestEnrichActiveGroupsAIIdentifierMismatch(t *testing.T) {
	// AI returns a review for an identifier not in the groups
	responseJSON := `{"reviews":[{"identifier":"LUC-999","title":"Other","context":["ctx"],"bottleneck":[],"advice":[]}]}`
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

	groups := []internal.GroupView{
		{Name: "FIXME", Issues: []internal.GroupItemView{{Identifier: "LUC-1", Title: "Fix this"}}},
	}
	details := []internal.IssueDetail{
		{Identifier: "LUC-1", Title: "Fix this", Description: "desc"},
	}

	enrichActiveGroups(aiClient, groups, details)
	// LUC-999 not in groups -> no enrichment
	assert.Empty(t, groups[0].Issues[0].Context)
}

// --- buildPerIssueReviews with AI ---

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

// --- buildEveningSummary with AI ---

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

// --- buildGroupsFromResult edge cases ---

func TestBuildGroupsFromResultDuplicateIdentifiers(t *testing.T) {
	result := &internal.MorningReviewJSON{
		Groups: []internal.MorningGroupJSON{
			{Name: "FIXME", Issues: []internal.MorningIssueItem{
				{Identifier: "LUC-1", Title: "Fix this"},
			}},
			{Name: "MAYBE", Issues: []internal.MorningIssueItem{
				{Identifier: "LUC-1", Title: "Fix this duplicate"},
			}},
		},
	}
	views := []internal.IssueView{
		{Identifier: "LUC-1", Title: "Fix this", URL: "https://example.com/1"},
	}
	groups := buildGroupsFromResult(result, views)
	// LUC-1 appears in FIXME, mentioned set prevents it in MAYBE (which has 0 items and is filtered)
	// Then uncategorized check: LUC-1 is mentioned, so no uncategorized group
	require.NotEmpty(t, groups)
}

func TestBuildGroupsFromResultCustomGroupName(t *testing.T) {
	result := &internal.MorningReviewJSON{
		Groups: []internal.MorningGroupJSON{
			{Name: "CUSTOM", Issues: []internal.MorningIssueItem{{Identifier: "LUC-1", Title: "Task"}}},
		},
	}
	views := []internal.IssueView{
		{Identifier: "LUC-1", Title: "Task", URL: "https://example.com/1"},
	}
	groups := buildGroupsFromResult(result, views)
	require.Len(t, groups, 1)
	assert.Equal(t, "CUSTOM", groups[0].Name)
}

// --- renderMorningIssueContent with partial sections ---

func TestRenderMorningIssueContentOnlyContext(t *testing.T) {
	item := &internal.GroupItemView{Context: []string{"context only"}}
	got := renderMorningIssueContent(item)
	assert.NotEmpty(t, got)
	assert.Contains(t, got, "context only")
}

func TestRenderMorningIssueContentOnlyBottleneck(t *testing.T) {
	item := &internal.GroupItemView{Bottleneck: []string{"bottleneck only"}}
	got := renderMorningIssueContent(item)
	assert.NotEmpty(t, got)
	assert.Contains(t, got, "bottleneck only")
}

func TestRenderMorningIssueContentOnlyAdvice(t *testing.T) {
	item := &internal.GroupItemView{Advice: []string{"advice only"}}
	got := renderMorningIssueContent(item)
	assert.NotEmpty(t, got)
	assert.Contains(t, got, "advice only")
}

// --- buildGroupItems without viewMap match ---

func TestBuildGroupItemsNoViewMapMatch(t *testing.T) {
	g := internal.MorningGroupJSON{
		Name:   "FIXME",
		Issues: []internal.MorningIssueItem{{Identifier: "UNKNOWN", Title: "Unknown task"}},
	}
	viewMap := map[string]internal.IssueView{} // empty
	mentioned := make(map[string]bool)
	items := buildGroupItems(g, viewMap, mentioned)
	require.Len(t, items, 1)
	assert.Equal(t, "UNKNOWN", items[0].Identifier)
	assert.Empty(t, items[0].URL) // no viewMap match
	assert.True(t, mentioned["UNKNOWN"])
}

// --- sendBriefEmptyEmail dry run with different subject ---

func TestSendBriefEmptyEmailDryRunAlt(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { os.Chdir(origDir) })

	cfg := &internal.Config{}
	err := sendBriefEmptyEmail(cfg, "Subject", "Body text", true)
	require.NoError(t, err)
}

// --- toIssueViews edge cases ---

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

// --- helper to JSON-encode a string value for embedding in test JSON ---

func jsonMarshalString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
