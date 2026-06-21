package cmd

import (
	"os"
	"strings"
	"testing"
)

func TestRenderHuntHTML(t *testing.T) {
	report := &huntReport{
		GeneratedAt: "2026-06-06T12:00:00Z",
		DryRun:      true,
		Warnings: []huntWarning{
			{Provider: providerExa, Category: "tech", Message: "Rate limit exceeded, partial results returned"},
		},
		Failures: []huntFailure{
			{Provider: providerTavily, Category: "science", Message: "API key invalid"},
		},
		Stats: huntStats{
			CategoriesScanned:  3,
			ProviderCalls:      6,
			SuccessfulCalls:    4,
			RawCandidates:      28,
			AcceptedCandidates: 5,
			FilteredCandidates: 23,
		},
		Candidates: []huntCandidate{
			{
				Title:         "Engineering Blog",
				URL:           "https://blog.example.com",
				NormalizedURL: "https://blog.example.com",
				Category:      "tech",
				CandidateType: candSource,
				Provider:      providerExa,
				Reason:        "Similar to existing tech seeds and returned by Exa. High-quality engineering content.",
				Domain:        "blog.example.com",
				IsNew:         true,
				Score:         0.85,
				Confidence:    0.9,
				EvidenceURLs:  []string{"https://techcrunch.com", "https://arstechnica.com"},
			},
			{
				Title:         "John's Tech Newsletter",
				URL:           "https://johns.substack.com",
				NormalizedURL: "https://johns.substack.com",
				Category:      "tech",
				CandidateType: candNewsletter,
				Provider:      providerTavily,
				Reason:        "Popular tech newsletter with 50k subscribers.",
				Domain:        "johns.substack.com",
				IsNew:         false,
				SeenCount:     3,
				Score:         0.72,
				Confidence:    0.8,
			},
			{
				Title:         "awesome-go",
				URL:           "https://github.com/avelino/awesome-go",
				NormalizedURL: "https://github.com/avelino/awesome-go",
				Category:      "tech",
				CandidateType: candRepo,
				Provider:      providerExa,
				Reason:        "Curated list of Go frameworks and libraries.",
				Domain:        "github.com",
				IsNew:         true,
				Score:         0.65,
				Confidence:    0.7,
				EvidenceURLs:  []string{"https://go.dev"},
			},
			{
				Title:         "Science Daily",
				URL:           "https://sciencedaily.com",
				NormalizedURL: "https://sciencedaily.com",
				Category:      "science",
				CandidateType: candSource,
				Provider:      providerExa,
				Reason:        "Major science news aggregator with original research summaries.",
				Domain:        "sciencedaily.com",
				IsNew:         true,
				Score:         0.78,
				Confidence:    0.85,
			},
			{
				Title:         "AI Researcher Blog",
				URL:           "https://ai-researcher.dev",
				NormalizedURL: "https://ai-researcher.dev",
				Category:      "ai",
				CandidateType: candAuthor,
				Provider:      providerTavily,
				Reason:        "Personal blog of a leading AI researcher with deep technical posts.",
				Domain:        "ai-researcher.dev",
				IsNew:         false,
				SeenCount:     2,
				Score:         0.6,
				Confidence:    0.75,
			},
		},
	}

	html := renderHuntHTML(report)

	if err := os.WriteFile("/tmp/hunt-test.html", []byte(html), 0o644); err != nil {
		t.Fatalf("write HTML: %v", err)
	}

	t.Logf("Hunt report written to /tmp/hunt-test.html (%d bytes)", len(html))

	if len(html) == 0 {
		t.Fatal("rendered HTML is empty")
	}
	if !strings.Contains(html, "Source Discovery") {
		t.Error("missing title")
	}
	if !strings.Contains(html, "Engineering Blog") {
		t.Error("missing candidate title")
	}
	if !strings.Contains(html, "Rate limit exceeded") {
		t.Error("missing warning message")
	}
	if !strings.Contains(html, "API key invalid") {
		t.Error("missing failure message")
	}
}

func TestRenderTrnsPage(t *testing.T) {
	view := trnsPageView{
		Title:      "Episode 42: Building Resilient Systems",
		FeedTitle:  "Software Engineering Daily",
		EpisodeURL: "https://softwareengineeringdaily.com/ep42",
		Status:     "found",
		Summary:    "This episode discusses building resilient distributed systems with circuit breakers, bulkheads, and retry patterns.",
		Content: `Welcome to Software Engineering Daily. Today we're talking about building resilient systems.

Host: Today our guest is Jane Smith, author of "Resilient Architecture". Jane, welcome to the show.

Jane: Thanks for having me. I'm excited to talk about this topic.

Host: Let's start with the basics. What makes a system resilient?

Jane: A resilient system is one that can continue operating in the presence of failures. The key patterns are circuit breakers, bulkheads, and graceful degradation...`,
	}

	html := renderTrnsPage(&view)

	if err := os.WriteFile("/tmp/trns-test.html", []byte(html), 0o644); err != nil {
		t.Fatalf("write HTML: %v", err)
	}

	t.Logf("Trns page written to /tmp/trns-test.html (%d bytes)", len(html))

	if len(html) == 0 {
		t.Fatal("rendered HTML is empty")
	}
	if !strings.Contains(html, "Episode 42") {
		t.Error("missing title")
	}
	if !strings.Contains(html, "Software Engineering Daily") {
		t.Error("missing feed title")
	}
	if !strings.Contains(html, "AI Summary") {
		t.Error("missing AI Summary section")
	}
	if !strings.Contains(html, "circuit breakers") {
		t.Error("missing summary content")
	}
}

func TestRenderTrnsPageWithError(t *testing.T) {
	view := trnsPageView{
		Title:        "Failed Episode",
		FeedTitle:    "Test Feed",
		Status:       "failed",
		SummaryError: "AI service unavailable: connection timeout",
		Content:      "Raw transcript content here.",
	}

	html := renderTrnsPage(&view)

	if err := os.WriteFile("/tmp/trns-error-test.html", []byte(html), 0o644); err != nil {
		t.Fatalf("write HTML: %v", err)
	}

	if !strings.Contains(html, "AI Summary unavailable") {
		t.Error("missing summary error section")
	}
	if !strings.Contains(html, "connection timeout") {
		t.Error("missing error message")
	}
}
