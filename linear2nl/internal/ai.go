package internal

import (
	"bytes"
	"embed"
	"fmt"
	"log/slog"
	"strings"
	"text/template"

	"github.com/xbpk3t/docs-alfred/pkg/ai"
)

//go:embed prompts/morning-summary.txt prompts/evening-summary.txt
var promptFiles embed.FS

// --- Prompt data types ---

type morningPromptData struct {
	Lang   string
	Issues []IssueView
}

type eveningPromptData struct {
	Lang      string
	Completed []IssueView
	Changes   []StateChangeView
}

type eveningDeepPromptData struct {
	Lang   string
	Issues []IssueDetail
}

// AIProvider wraps pkg/ai and prompt templates for report generation.
type AIProvider struct {
	clientCfg *ai.ClientConfig
	prompts   embed.FS
	lang      string
}

// NewAIProvider creates an AIProvider from config.
func NewAIProvider(cfg AIConfig) *AIProvider {
	clientCfg := ai.ConfigWithOverrides(cfg.APIKey, cfg.BaseURL, cfg.Model)
	clientCfg.Timeout = cfg.Timeout
	lang := cfg.Language
	if lang == "" {
		lang = "zh"
	}

	return &AIProvider{
		clientCfg: clientCfg,
		lang:      lang,
		prompts:   promptFiles,
	}
}

// MorningSummary generates AI priority suggestions for the morning report.
// Returns markdown string; empty if AI is unavailable or call fails.
func (p *AIProvider) MorningSummary(issues []IssueView) string {
	prompt, err := p.renderPrompt("prompts/morning-summary.txt", morningPromptData{
		Lang:   p.lang,
		Issues: issues,
	})
	if err != nil {
		slog.Warn("failed to render morning prompt", "error", err)

		return ""
	}

	return p.chat(prompt)
}

// EveningSummary generates AI summary for the evening report (simple version).
// Returns markdown string; empty if AI is unavailable or call fails.
func (p *AIProvider) EveningSummary(completed []IssueView, changes []StateChangeView) string {
	prompt, err := p.renderPrompt("prompts/evening-summary.txt", eveningPromptData{
		Lang:      p.lang,
		Completed: completed,
		Changes:   changes,
	})
	if err != nil {
		slog.Warn("failed to render evening prompt", "error", err)

		return ""
	}

	return p.chat(prompt)
}

// EveningDeepReview generates per-issue deep review for the evening report.
// Returns markdown string; empty if AI is unavailable or call fails.
func (p *AIProvider) EveningDeepReview(issues []IssueDetail) string {
	prompt, err := p.renderPrompt("prompts/evening-summary.txt", eveningDeepPromptData{
		Lang:   p.lang,
		Issues: issues,
	})
	if err != nil {
		slog.Warn("failed to render evening deep review prompt", "error", err)

		return ""
	}

	return p.chat(prompt)
}

// IsConfigured returns whether the AI client has an API key.
func (p *AIProvider) IsConfigured() bool {
	return p.clientCfg.APIKey != ""
}

func (p *AIProvider) chat(prompt string) string {
	if !p.IsConfigured() {
		return ""
	}

	result, err := ai.Chat(p.clientCfg, []ai.Message{{Role: "user", Content: prompt}})
	if err != nil {
		slog.Warn("AI call failed", "error", err)

		return ""
	}

	return strings.TrimSpace(result)
}

func (p *AIProvider) renderPrompt(name string, data any) (string, error) {
	tmpl, err := template.ParseFS(p.prompts, name)
	if err != nil {
		return "", fmt.Errorf("parse prompt %s: %w", name, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render prompt %s: %w", name, err)
	}

	return buf.String(), nil
}
