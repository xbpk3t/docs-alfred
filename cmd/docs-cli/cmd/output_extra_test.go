package cmd

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	wikiuc "github.com/xbpk3t/docs-alfred/internal/docs/ingest"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

func captureStderr(t *testing.T) func() string {
	t.Helper()

	original := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = w

	return func() string {
		require.NoError(t, w.Close())
		os.Stderr = original
		data, err := io.ReadAll(r)
		require.NoError(t, err)
		require.NoError(t, r.Close())

		return string(data)
	}
}

func TestNormalizeOutputFormat(t *testing.T) {
	tests := []struct {
		input string
		want  string
		err   bool
	}{
		{"", outputFormatText, false},
		{"text", outputFormatText, false},
		{"json", outputFormatJSON, false},
		{"yaml", "", true},
		{"xml", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := normalizeOutputFormat(tt.input)
			if tt.err {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "unsupported")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestWriteCommandOutputTextFormat(t *testing.T) {
	stdout := captureStdout(t)

	err := writeCommandOutput("text", &CommandOutput{
		Name:    "test",
		OK:      true,
		Actions: []string{"action1"},
	}, "details here\n")
	require.NoError(t, err)

	got := stdout()
	assert.Contains(t, got, "details here")
	assert.Contains(t, got, "[actions]")
	assert.Contains(t, got, "action1")
}

func TestWriteCommandOutputTextNoTrailingNewline(t *testing.T) {
	stdout := captureStdout(t)

	err := writeCommandOutput("text", &CommandOutput{
		Name: "test",
	}, "no newline")
	require.NoError(t, err)

	got := stdout()
	assert.Contains(t, got, "no newline\n")
}

func TestWriteCheckCommandOutputTextFormat(t *testing.T) {
	stdout := captureStdout(t)

	err := writeCheckCommandOutput("text", &checkCommandOutput{
		Name:   "test check",
		Issues: nil,
	}, "text details\n")
	require.NoError(t, err)

	got := stdout()
	assert.Contains(t, got, "text details")
}

func TestWriteCheckCommandOutputWithErrors(t *testing.T) {
	stderr := captureStderr(t)

	err := writeCheckCommandOutput("text", &checkCommandOutput{
		Name: "test check",
		Issues: []checkutil.Issue{
			{Message: "bad thing", Severity: checkutil.SeverityError},
		},
	}, "details\n")
	require.NoError(t, err)
	_ = stderr
}

func TestFormatActionsNonEmpty(t *testing.T) {
	got := formatActions([]string{"do thing 1", "do thing 2"})
	assert.Contains(t, got, "[actions]")
	assert.Contains(t, got, "do thing 1")
	assert.Contains(t, got, "do thing 2")
}

func TestFormatActionsEmpty(t *testing.T) {
	got := formatActions(nil)
	assert.Empty(t, got)
}

func TestResolveWikiAPIKeyAlreadySet(t *testing.T) {
	cfg := &wikiuc.Config{AI: wikiuc.AIConfig{APIKey: "existing-key"}}
	resolveWikiAPIKey(cfg)
	assert.Equal(t, "existing-key", cfg.AI.APIKey)
}

func TestResolveWikiAPIKeyFromOpenAI(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "openai-key")
	t.Setenv("LLM_AxonHub", "")
	cfg := &wikiuc.Config{}
	resolveWikiAPIKey(cfg)
	assert.Equal(t, "openai-key", cfg.AI.APIKey)
}

func TestResolveWikiAPIKeyFromAxonHub(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("LLM_AxonHub", "axon-key")
	cfg := &wikiuc.Config{}
	resolveWikiAPIKey(cfg)
	assert.Equal(t, "axon-key", cfg.AI.APIKey)
}

func TestResolveWikiAPIKeyNoneSet(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("LLM_AxonHub", "")
	cfg := &wikiuc.Config{}
	resolveWikiAPIKey(cfg)
	assert.Empty(t, cfg.AI.APIKey)
}

func TestResolveWikiAPIKeyOpenAIWins(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "openai-key")
	t.Setenv("LLM_AxonHub", "axon-key")
	cfg := &wikiuc.Config{}
	resolveWikiAPIKey(cfg)
	assert.Equal(t, "openai-key", cfg.AI.APIKey)
}

func TestApplyWikiFlagOverridesModel(t *testing.T) {
	cfg := &wikiuc.Config{AI: wikiuc.AIConfig{Model: "old-model"}}
	flags := &wikiFlags{model: "new-model"}
	applyWikiFlagOverrides(cfg, flags)
	assert.Equal(t, "new-model", cfg.AI.Model)
}

func TestApplyWikiFlagOverridesMaxContentSize(t *testing.T) {
	cfg := &wikiuc.Config{Wiki: wikiuc.WikiConfig{MaxContentSize: 10000}}
	flags := &wikiFlags{maxContentSize: 50000}
	applyWikiFlagOverrides(cfg, flags)
	assert.Equal(t, 50000, cfg.Wiki.MaxContentSize)
}

func TestApplyWikiFlagOverridesNoOp(t *testing.T) {
	cfg := &wikiuc.Config{
		AI:   wikiuc.AIConfig{Model: "keep-model"},
		Wiki: wikiuc.WikiConfig{MaxContentSize: 20000},
	}
	flags := &wikiFlags{}
	applyWikiFlagOverrides(cfg, flags)
	assert.Equal(t, "keep-model", cfg.AI.Model)
	assert.Equal(t, 20000, cfg.Wiki.MaxContentSize)
}

func TestFormatWikiTextResultOK(t *testing.T) {
	result := &wikiuc.Result{
		Name: "wiki add",
		URLResults: []wikiuc.URLResult{
			{URL: "https://example.com/a", Status: wikiuc.StatusSummaryWritten, OutputPath: "wiki/topic/a.md", TopicPath: "topic/a", Handled: true},
		},
	}
	got := formatWikiTextResult(result)
	assert.Contains(t, got, "wiki add passed")
	assert.Contains(t, got, "https://example.com/a")
	assert.Contains(t, got, "wiki/topic/a.md")
	assert.Contains(t, got, "topic=topic/a")
}

func TestFormatWikiTextResultFailed(t *testing.T) {
	result := &wikiuc.Result{
		Name: "wiki add",
		URLResults: []wikiuc.URLResult{
			{URL: "https://example.com/b", Status: wikiuc.StatusUnhandledError, Error: "timeout"},
		},
	}
	got := formatWikiTextResult(result)
	assert.Contains(t, got, "wiki add failed")
	assert.Contains(t, got, "error=timeout")
}

func TestFormatWikiAuditTextResultOK(t *testing.T) {
	result := &wikiuc.AuditResult{
		Name: "wiki audit",
	}
	got := formatWikiAuditTextResult(result)
	assert.Contains(t, got, "wiki audit passed")
}

func TestFormatWikiAuditTextResultFailed(t *testing.T) {
	result := &wikiuc.AuditResult{
		Name: "wiki audit",
		Issues: []checkutil.Issue{
			{Message: "bad thing", Severity: checkutil.SeverityError},
		},
	}
	got := formatWikiAuditTextResult(result)
	assert.Contains(t, got, "wiki audit failed")
}

func TestWriteWikiResultTextFormat(t *testing.T) {
	stdout := captureStdout(t)

	err := writeWikiResult(&wikiuc.Result{
		Name: "wiki add",
		URLResults: []wikiuc.URLResult{
			{URL: "https://example.com/a", Status: wikiuc.StatusSummaryWritten, Handled: true},
		},
	}, "text")
	require.NoError(t, err)

	got := stdout()
	assert.Contains(t, got, "wiki add passed")
}

func TestWriteWikiResultTextFormatFailed(t *testing.T) {
	stdout := captureStdout(t)
	_ = stdout

	err := writeWikiResult(&wikiuc.Result{
		Name: "wiki add",
		URLResults: []wikiuc.URLResult{
			{URL: "https://example.com/b", Status: wikiuc.StatusUnhandledError},
		},
	}, "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "wiki add failed")
}

func TestWriteWikiAuditResultTextFormat(t *testing.T) {
	stdout := captureStdout(t)

	err := writeWikiAuditResult(&wikiuc.AuditResult{
		Name: "wiki audit",
	}, "text")
	require.NoError(t, err)

	got := stdout()
	assert.Contains(t, got, "wiki audit passed")
}

func TestWriteWikiAuditResultFailed(t *testing.T) {
	err := writeWikiAuditResult(&wikiuc.AuditResult{
		Name: "wiki audit",
		Issues: []checkutil.Issue{
			{Message: "bad", Severity: checkutil.SeverityError},
		},
	}, "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "wiki audit failed")
}

func TestDotfilesCheckCommandFlags(t *testing.T) {
	dotfilesCmd, _, err := newRootCmd().Find([]string{"dotfiles", "check"})
	require.NoError(t, err)

	f := dotfilesCmd.Flags()
	require.NotNil(t, f.Lookup("path"))
	require.NotNil(t, f.Lookup("data-dir"))
	require.NotNil(t, dotfilesCmd.InheritedFlags().Lookup("format"))
}

func TestImagesCheckCommandFlags(t *testing.T) {
	imagesCmd, _, err := newRootCmd().Find([]string{"images", "check"})
	require.NoError(t, err)

	f := imagesCmd.Flags()
	require.NotNil(t, f.Lookup("data-dir"))
	require.NotNil(t, f.Lookup("images-dir"))
	require.NotNil(t, f.Lookup("list"))
	require.NotNil(t, f.Lookup("skip-extra-files"))
	require.NotNil(t, imagesCmd.InheritedFlags().Lookup("format"))
}


func TestWikiDigestLocalCommandFlags(t *testing.T) {
	wikiCmd, _, err := newRootCmd().Find([]string{"wiki", "digest-local"})
	require.NoError(t, err)

	f := wikiCmd.Flags()
	require.NotNil(t, f.Lookup("config"))
	require.NotNil(t, f.Lookup("wiki-root"))
	require.NotNil(t, f.Lookup("from-dir"))
}

func TestWikiCheckCommandFlags(t *testing.T) {
	wikiCmd, _, err := newRootCmd().Find([]string{"wiki", "check"})
	require.NoError(t, err)

	f := wikiCmd.Flags()
	require.NotNil(t, f.Lookup("gh-root"))
	require.NotNil(t, f.Lookup("wiki-root"))
	require.NotNil(t, wikiCmd.InheritedFlags().Lookup("format"))
}
