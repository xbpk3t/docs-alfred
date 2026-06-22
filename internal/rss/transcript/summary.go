package transcript

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/xbpk3t/docs-alfred/pkg/ai"
	"github.com/xbpk3t/docs-alfred/pkg/textutil"
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
		language = "zh"
	}

	return &Summarizer{Config: cfg, Language: language}
}

// SummaryResult holds the generated summary.
type SummaryResult struct {
	Summary     string `json:"summary"`
	GeneratedAt string `json:"generatedAt"`
}

func (s *Summarizer) summaryPrompt() string {
	switch s.Language {
	case "zh":
		return "你是一个简洁的技术摘要助手。请用中文对以下内容进行摘要，3-5 句话，聚焦关键技术点、决策和要点。"
	case "en":
		return "You are a concise technical summary assistant. " +
			"Summarize the following content in English in 3-5 sentences, " +
			"focusing on key technical points, decisions, and highlights."
	default:
		return "你是一个简洁的技术摘要助手。请用中文对以下内容进行摘要，3-5 句话，聚焦关键技术点、决策和要点。"
	}
}

// GenerateSummary generates a summary of the transcript content.
// Uses system prompt + user message (TS summary/client.ts pattern).
func (s *Summarizer) GenerateSummary(ctx context.Context, episodeTitle, transcriptContent string) (*SummaryResult, error) {
	if s.Config == nil || s.Config.APIKey == "" {
		return nil, errors.New("AI not configured")
	}

	content := truncateTranscript(transcriptContent, 8000)

	// TS equivalent: system prompt + user message
	systemMsg := s.summaryPrompt()
	userMsg := fmt.Sprintf("Title: %s\n\nContent:\n%s", episodeTitle, content)

	messages := []ai.Message{
		{Role: "system", Content: systemMsg},
		{Role: "user", Content: userMsg},
	}

	result, err := ai.ChatContext(ctx, s.Config, messages)
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
	truncated := textutil.TruncateUTF8(content, maxChars)
	if !strings.HasSuffix(truncated, "...") {
		return truncated
	}
	truncated = strings.TrimSuffix(truncated, "...")
	if idx := strings.LastIndex(truncated, " "); idx > 0 {
		truncated = truncated[:idx]
	}

	return truncated + "..."
}
