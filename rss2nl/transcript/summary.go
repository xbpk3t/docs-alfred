package transcript

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/xbpk3t/docs-alfred/pkg/ai"
)

// Summarizer generates AI summaries of transcripts.
// TS equivalent: summary/client.ts — uses system prompt + user message.
type Summarizer struct {
	Config   *ai.ClientConfig
	Language string
}

// NewSummarizer creates a new summarizer with the given AI config.
func NewSummarizer(cfg *ai.ClientConfig, language string) *Summarizer {
	if language == "" {
		language = "en"
	}

	return &Summarizer{Config: cfg, Language: language}
}

// SummaryResult holds the generated summary.
type SummaryResult struct {
	Summary     string `json:"summary"`
	GeneratedAt string `json:"generatedAt"`
}

// GenerateSummary generates a summary of the transcript content.
// Uses system prompt + user message (TS summary/client.ts pattern).
func (s *Summarizer) GenerateSummary(ctx context.Context, episodeTitle, transcriptContent string) (*SummaryResult, error) {
	if s.Config == nil || s.Config.APIKey == "" {
		return nil, errors.New("AI not configured")
	}

	content := truncateTranscript(transcriptContent, 8000)

	// TS equivalent: system prompt + user message
	systemMsg := "你是一个简洁的技术摘要助手。请用中文对以下内容进行摘要，3-5 句话，聚焦关键技术点、决策和要点。"
	userMsg := fmt.Sprintf("Title: %s\n\nContent:\n%s", episodeTitle, content)

	messages := []ai.Message{
		{Role: "system", Content: systemMsg},
		{Role: "user", Content: userMsg},
	}

	result, err := ai.Chat(s.Config, messages)
	if err != nil {
		return nil, fmt.Errorf("ai summary: %w", err)
	}

	result = strings.TrimSpace(result)
	if result == "" {
		return nil, errors.New("empty summary from AI")
	}

	return &SummaryResult{
		Summary: result,
	}, nil
}

func truncateTranscript(content string, maxChars int) string {
	if len(content) <= maxChars {
		return content
	}

	truncated := content[:maxChars]
	if idx := strings.LastIndex(truncated, " "); idx > 0 {
		truncated = truncated[:idx]
	}

	return truncated + "..."
}
