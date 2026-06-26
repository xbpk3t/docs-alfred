package internal

import (
	"bytes"
	"embed"
	"fmt"
	"log/slog"
	"strings"
	"text/template"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/xbpk3t/docs-alfred/pkg/ai"
)

//go:embed prompts/plan.txt prompts/summary.txt
var promptFiles embed.FS

// PromptFiles is the embedded prompt filesystem, exported for use by review command.
var PromptFiles = promptFiles

// --- Prompt data types ---

type planPromptData struct {
	Lang   string
	Issues []IssueDetail
}

type summaryPromptData struct {
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

// PlanJSON is the expected JSON structure from the AI morning plan.
type PlanJSON struct {
	Reviews []PlanItemJSON `json:"reviews"`
}

// PlanItemJSON is a single issue item in the morning plan JSON response.
type PlanItemJSON struct {
	Identifier string   `json:"identifier"`
	Title      string   `json:"title"`
	Context    []string `json:"context"`
	Bottleneck []string `json:"bottleneck"`
	Advice     []string `json:"advice"`
}

// MorningPlan generates per-issue plan for the morning report.
// Takes IssueDetail (with description + comments).
// Returns raw JSON string; empty if AI is unavailable or call fails.
func (p *AIProvider) MorningPlan(issues []IssueDetail) string {
	prompt, err := p.renderPrompt("prompts/plan.txt", planPromptData{
		Lang:   p.lang,
		Issues: issues,
	})
	if err != nil {
		slog.Warn("failed to render morning plan prompt", "error", err)

		return ""
	}

	return p.chat(prompt)
}

// EveningDeepReview generates per-issue deep review for the evening report.
// Returns raw JSON string; empty if AI is unavailable or call fails.
func (p *AIProvider) EveningDeepReview(issues []IssueDetail) string {
	prompt, err := p.renderPrompt("prompts/summary.txt", summaryPromptData{
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

	var result string
	err := retry.Do(
		func() error {
			r, err := ai.Chat(p.clientCfg, []ai.Message{{Role: "user", Content: prompt}})
			if err != nil {
				return err
			}
			result = r

			return nil
		},
		retry.Attempts(3),
		retry.Delay(1*time.Second),
		retry.DelayType(retry.BackOffDelay),
	)
	if err != nil {
		slog.Warn("AI call failed after retries", "error", err)

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
