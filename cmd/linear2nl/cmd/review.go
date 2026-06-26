package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"text/template"

	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/cmd/linear2nl/internal"
	"github.com/xbpk3t/docs-alfred/pkg/ai"
	"github.com/xbpk3t/docs-alfred/pkg/md"
)

// linearIDRe matches Linear issue identifiers like "ENG-123".
var linearIDRe = regexp.MustCompile(`[A-Z]{2,4}-\d+`)

func newReviewCmd() *cobra.Command {
	var (
		cfgFile string
		issue   int
		owner   string
		repo    string
		dryRun  bool
	)

	cmd := &cobra.Command{
		Use:   "review",
		Short: "AI review for a closed GitHub issue",
		Long: `Fetch a GitHub issue, generate an AI review, and post it as a comment.

Requires GITHUB_TOKEN environment variable for authentication.
Designed to run from GitHub Actions on issue close events.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := internal.LoadConfig(cfgFile)
			if err != nil {
				return err
			}

			if owner == "" {
				owner = cfg.GitHub.Owner
			}
			if repo == "" {
				repo = cfg.GitHub.Repo
			}
			if owner == "" || repo == "" {
				return fmt.Errorf("--owner and --repo are required (or set github.owner/repo in config)")
			}

			token := os.Getenv("GITHUB_TOKEN")
			if token == "" {
				return fmt.Errorf("GITHUB_TOKEN environment variable is required")
			}

			return runReview(cfg, token, owner, repo, issue, dryRun)
		},
	}

	cmd.Flags().StringVarP(&cfgFile, "config", "c", "cmd/linear2nl/linear2nl.yml", "config file path")
	cmd.Flags().IntVarP(&issue, "issue", "i", 0, "GitHub issue number")
	cmd.Flags().StringVar(&owner, "owner", "", "GitHub repo owner")
	cmd.Flags().StringVar(&repo, "repo", "", "GitHub repo name")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print review to stdout instead of posting comment")

	_ = cmd.MarkFlagRequired("issue")

	return cmd
}

// reviewJSON is the expected JSON structure from the AI review.
type reviewJSON struct {
	Reviews []reviewItemJSON `json:"reviews"`
	Summary []string         `json:"summary"`
}

// reviewItemJSON is a single review item in the JSON response.
type reviewItemJSON struct {
	Identifier string   `json:"identifier"`
	Title      string   `json:"title"`
	Progress   []string `json:"progress"`
	Knowledge  []string `json:"knowledge"`
	Review     []string `json:"review"`
}

func runReview(cfg *internal.Config, token, owner, repo string, issueNumber int, dryRun bool) error {
	ctx := context.Background()

	// 1. Fetch issue from GitHub.
	gh := internal.NewGitHubClient(token, owner, repo)
	issueData, err := gh.GetIssueDetail(ctx, issueNumber)
	if err != nil {
		return err
	}

	// 2. Extract Linear issue reference from GitHub issue body.
	if m := linearIDRe.FindString(issueData.Description); m != "" {
		issueData.LinearReference = m
		slog.Info("found Linear reference in issue body", "identifier", m)
	}

	// 3. Build prompt input (field names match summary.txt template).
	input := internal.GitHubReviewInput{
		Lang:   cfg.AI.Language,
		Issues: []internal.GitHubReviewIssue{*issueData},
	}

	// 4. Render prompt.
	prompt, err := renderGitHubReviewPrompt(input)
	if err != nil {
		return fmt.Errorf("render prompt: %w", err)
	}

	// 5. Call AI.
	clientCfg := ai.ConfigWithOverrides(cfg.AI.APIKey, cfg.AI.BaseURL, cfg.AI.Model)
	clientCfg.Timeout = cfg.AI.Timeout

	raw, err := ai.Chat(clientCfg, []ai.Message{{Role: "user", Content: prompt}})
	if err != nil {
		return fmt.Errorf("AI call failed: %w", err)
	}
	if raw == "" {
		return fmt.Errorf("AI returned empty response")
	}
	slog.Info("AI raw response preview", "len", len(raw), "raw", raw[:min(len(raw), 2000)])

	// 6. Parse response.
	review, err := parseReviewJSON(raw)
	if err != nil {
		return fmt.Errorf("parse AI response: %w", err)
	}
	if review == "" {
		return fmt.Errorf("no review generated for #%d", issueNumber)
	}

	// 7. Output.
	if dryRun {
		fmt.Println(review) //nolint:forbidigo // dry-run intentionally outputs to stdout

		return nil
	}

	return gh.PostReviewComment(ctx, issueNumber, review)
}

func renderGitHubReviewPrompt(input internal.GitHubReviewInput) (string, error) {
	if input.Lang == "" {
		input.Lang = "zh"
	}

	tmpl, err := template.ParseFS(internal.PromptFiles, "prompts/summary.txt")
	if err != nil {
		return "", fmt.Errorf("parse prompt: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, input); err != nil {
		return "", fmt.Errorf("render prompt: %w", err)
	}

	return buf.String(), nil
}

func parseReviewJSON(raw string) (string, error) {
	var result reviewJSON
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return "", fmt.Errorf("unmarshal JSON: %w", err)
	}

	if len(result.Reviews) == 0 {
		return "", fmt.Errorf("no reviews in response")
	}

	// Take the first review (single-issue mode).
	r := result.Reviews[0]

	var sections []md.ReviewSection
	if len(r.Progress) > 0 {
		sections = append(sections, md.ReviewSection{Heading: "progress", Items: r.Progress})
	}
	if len(r.Knowledge) > 0 {
		sections = append(sections, md.ReviewSection{Heading: "knowledge", Items: r.Knowledge})
	}
	if len(r.Review) > 0 {
		sections = append(sections, md.ReviewSection{Heading: "review", Items: r.Review})
	}

	return md.AIReviewItem(sections...).Markdown(), nil
}
