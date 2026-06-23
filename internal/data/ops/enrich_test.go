package dataops

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/internal/gh/enrich"
)

// mockEnricher implements enrich.Enricher for testing.
type mockEnricher struct {
	fields *enrich.EnrichFields
	err    error
}

func (m *mockEnricher) Enrich(_ context.Context, _, _ string) (*enrich.EnrichFields, error) {
	return m.fields, m.err
}

// parseFirstItem writes YAML content to a temp file, parses it, and returns
// the first ItemNode. This is needed because ItemNode.pending is unexported
// and must be initialised via ParseYAMLFile.
func parseFirstItem(t *testing.T, content string) *enrich.ItemNode {
	t.Helper()
	f := filepath.Join(t.TempDir(), "test.yml")
	require.NoError(t, os.WriteFile(f, []byte(content), 0o644))
	items, _, err := enrich.ParseYAMLFile(f)
	require.NoError(t, err)
	require.NotEmpty(t, items)

	return items[0]
}

func TestValidateInput_Nil(t *testing.T) {
	err := validateInput(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

func TestValidateInput_EmptyPath(t *testing.T) {
	// Books require explicit path
	err := validateInput(&EnrichInput{
		Resource: enrich.ResourceBook,
		APIKey:   "test",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple files")
}

func TestValidateInput_MovieDefaultPath(t *testing.T) {
	// Movie has a default path
	input := &EnrichInput{
		Resource: enrich.ResourceMovie,
		APIKey:   "test",
	}
	err := validateInput(input)
	require.NoError(t, err)
	assert.NotEmpty(t, input.Path)
}

func TestValidateInput_TVDefaultPath(t *testing.T) {
	input := &EnrichInput{
		Resource: enrich.ResourceTV,
		APIKey:   "test",
	}
	err := validateInput(input)
	require.NoError(t, err)
	assert.NotEmpty(t, input.Path)
}

func TestValidateInput_UnknownResource(t *testing.T) {
	err := validateInput(&EnrichInput{
		Resource: enrich.ResourceType("unknown"),
		APIKey:   "test",
	})
	require.Error(t, err)
}

func TestValidateInput_NoAPIKey(t *testing.T) {
	err := validateInput(&EnrichInput{
		Resource: enrich.ResourceMovie,
		Path:     "/tmp/test.yml",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API key")
}

func TestBuildReport(t *testing.T) {
	results := []*enrich.EnrichResult{
		{Index: 0, Name: "item1", Actions: []enrich.EnrichAction{{Field: "publishAt", Value: "2023"}}},
		{Index: 1, Name: "item2", Err: assert.AnError},
		{Index: 2, Name: "item3", NeedsReview: true},
		nil, // nil result
	}
	input := &EnrichInput{
		Resource: enrich.ResourceMovie,
		Path:     "/tmp/test.yml",
		DryRun:   true,
	}

	report := buildReport(results, input)
	require.NotNil(t, report)
	require.NotNil(t, report.Report)
	assert.Equal(t, enrich.ResourceMovie, report.Report.Resource)
	assert.True(t, report.Report.DryRun)
	assert.Len(t, report.Report.Results, 4)
	assert.Equal(t, "(unknown)", report.Report.Results[3].Name)
}

func TestFormatEnrichReport(t *testing.T) {
	report := &enrich.EnrichReport{
		Resource: enrich.ResourceMovie,
		File:     "/tmp/test.yml",
		DryRun:   true,
		Results: []enrich.EnrichResult{
			{Index: 0, Name: "Movie A", Actions: []enrich.EnrichAction{{Field: "publishAt", Value: "2023"}}},
			{Index: 1, Name: "Movie B", Err: assert.AnError},
			{Index: 2, Name: "Movie C", NeedsReview: true},
			{Index: 3, Name: "Movie D"}, // already complete
			{Index: 4, Name: "Movie E", Actions: []enrich.EnrichAction{{Field: "alias", Skipped: true}}},
		},
	}

	output := FormatEnrichReport(report)
	assert.Contains(t, output, "dry-run")
	assert.Contains(t, output, "Movie A")
	assert.Contains(t, output, "Movie B")
	assert.Contains(t, output, "Movie C")
	assert.Contains(t, output, "already complete")
	assert.Contains(t, output, "Summary:")
}

func TestEnrichPathForResource(t *testing.T) {
	tests := []struct {
		resource enrich.ResourceType
		wantErr  bool
	}{
		{enrich.ResourceMovie, false},
		{enrich.ResourceTV, false},
		{enrich.ResourceBook, true},
		{enrich.ResourceType("unknown"), true},
	}
	for _, tt := range tests {
		t.Run(string(tt.resource), func(t *testing.T) {
			path, err := enrichPathForResource(tt.resource)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, path)
			}
		})
	}
}

func TestProcessItem_NoName(t *testing.T) {
	// processItem with no name field returns an error result
	// This is tested indirectly through RunEnrich with a real YAML file
}

func TestRunEnrich_InvalidInput(t *testing.T) {
	_, err := RunEnrich(context.Background(), nil)
	require.Error(t, err)
}

func TestProcessItem_EmptyNameReturnsErrorResult(t *testing.T) {
	item := parseFirstItem(t, "- publishAt: 2023\n  alias: foo\n")
	m := &mockEnricher{}

	res, err := processItem(context.Background(), item, m, false, 0)

	require.NoError(t, err, "processItem should not return a Go error")
	require.NotNil(t, res)
	assert.Equal(t, "(unnamed)", res.Name)
	assert.Equal(t, 0, res.Index)
	require.Error(t, res.Err, "result should carry an error for empty name")
	assert.Contains(t, res.Err.Error(), "no name field")
}

func TestProcessItem_AllFieldsExistReturnsNoActions(t *testing.T) {
	item := parseFirstItem(t, `---
- name: Complete Movie
  publishAt: "2023"
  alias: Original Title
  dict: Director A
  author: Author A
  cast: Actor A
`)
	m := &mockEnricher{
		fields: &enrich.EnrichFields{PublishAt: "2024"},
	}

	res, err := processItem(context.Background(), item, m, false, 2)

	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, "Complete Movie", res.Name)
	assert.Equal(t, 2, res.Index)
	assert.Empty(t, res.Actions, "should have no actions when all fields already exist")
	assert.Nil(t, res.Err)
	assert.False(t, res.NeedsReview)
}

func TestProcessItem_EnricherNotFoundReturnsNeedsReview(t *testing.T) {
	item := parseFirstItem(t, "- name: Unknown Movie\n")
	m := &mockEnricher{err: enrich.ErrNotFound}

	res, err := processItem(context.Background(), item, m, false, 1)

	require.NoError(t, err, "ErrNotFound should not propagate as Go error")
	require.NotNil(t, res)
	assert.True(t, res.NeedsReview)
	assert.Nil(t, res.Err, "ErrNotFound should not set result.Err")
	assert.Equal(t, "Unknown Movie", res.Name)
	assert.Equal(t, 1, res.Index)
}

func TestProcessItem_NonNotFoundErrorReturnsError(t *testing.T) {
	item := parseFirstItem(t, "- name: Broken Movie\n")
	m := &mockEnricher{err: errors.New("api timeout")}

	res, err := processItem(context.Background(), item, m, false, 3)

	require.NoError(t, err, "processItem should not return a Go error")
	require.NotNil(t, res)
	require.Error(t, res.Err)
	assert.Contains(t, res.Err.Error(), "api timeout")
	assert.Contains(t, res.Err.Error(), "Broken Movie", "error should include the item name for context")
	assert.False(t, res.NeedsReview, "non-ErrNotFound should not mark NeedsReview")
}

func TestProcessItem_DryRunDoesNotApplyChanges(t *testing.T) {
	item := parseFirstItem(t, "- name: Dry Movie\n")
	m := &mockEnricher{
		fields: &enrich.EnrichFields{
			PublishAt: "2024",
			Alias:     "Original Title",
		},
	}

	res, err := processItem(context.Background(), item, m, true, 0)

	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Nil(t, res.Err)
	assert.Len(t, res.Actions, 2, "dry-run should still report actions")
	// Under dry-run the item's YAML fields must remain absent because
	// SetField is never called.
	assert.False(t, item.FieldExists(enrich.FieldPublishAt),
		"dry-run must not add publishAt to the item")
	assert.False(t, item.FieldExists(enrich.FieldAlias),
		"dry-run must not add alias to the item")
}

func TestFormatEnrichReport_NoDryRunLabel(t *testing.T) {
	report := &enrich.EnrichReport{
		Resource: enrich.ResourceMovie,
		File:     "/tmp/movies.yml",
		DryRun:   false,
		Results: []enrich.EnrichResult{
			{
				Index: 0,
				Name:  "Movie A",
				Actions: []enrich.EnrichAction{
					{Field: "publishAt", Value: "2023"},
				},
			},
		},
	}

	output := FormatEnrichReport(report)

	assert.NotContains(t, output, "dry-run",
		"non-dry-run report should not mention dry-run")
	assert.Contains(t, output, "/tmp/movies.yml")
	assert.Contains(t, output, "Movie A")
	assert.Contains(t, output, "Summary:")
}

func TestFormatEnrichReport_SkippedActions(t *testing.T) {
	report := &enrich.EnrichReport{
		Resource: enrich.ResourceMovie,
		File:     "/tmp/movies.yml",
		DryRun:   false,
		Results: []enrich.EnrichResult{
			{
				Index: 0,
				Name:  "Movie A",
				Actions: []enrich.EnrichAction{
					{Field: "publishAt", Value: "2023"},
					{Field: "alias", Skipped: true},
				},
			},
		},
	}

	output := FormatEnrichReport(report)

	assert.Contains(t, output, "alias ⏭️",
		"skipped action should show the skip indicator")
	assert.Contains(t, output, `publishAt="2023"`,
		"non-skipped action should show the value")
	assert.Contains(t, output, "Summary:")
}

func TestProcessItem_DictAction(t *testing.T) {
	item := parseFirstItem(t, "- name: Movie Needing Dict\n")
	m := &mockEnricher{
		fields: &enrich.EnrichFields{
			Dict: "Director A",
		},
	}

	res, err := processItem(context.Background(), item, m, true, 0)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Len(t, res.Actions, 1)
	assert.Equal(t, enrich.FieldDict, res.Actions[0].Field)
	assert.Equal(t, "Director A", res.Actions[0].Value)
}

func TestProcessItem_AuthorAction(t *testing.T) {
	item := parseFirstItem(t, "- name: Book Needing Author\n")
	m := &mockEnricher{
		fields: &enrich.EnrichFields{
			Author: "Author Name",
		},
	}

	res, err := processItem(context.Background(), item, m, true, 0)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Len(t, res.Actions, 1)
	assert.Equal(t, enrich.FieldAuthor, res.Actions[0].Field)
	assert.Equal(t, "Author Name", res.Actions[0].Value)
}

func TestProcessItem_AllActionsIncludingDictAndAuthor(t *testing.T) {
	item := parseFirstItem(t, "- name: Complete Coverage\n")
	m := &mockEnricher{
		fields: &enrich.EnrichFields{
			PublishAt: "2024",
			Alias:     "Original Title",
			Dict:      "Director A",
			Author:    "Author B",
			Cast:      "Actor C、Actor D",
		},
	}

	res, err := processItem(context.Background(), item, m, true, 0)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Len(t, res.Actions, 5, "should have all 5 field actions")
}
