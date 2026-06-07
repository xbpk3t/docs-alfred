package wiki

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"codeberg.org/readeck/go-readability/v2"
	"github.com/PuerkitoBio/goquery"
	"github.com/google/go-github/v70/github"
	"github.com/xbpk3t/docs-alfred/pkg/htmlutil"
	"github.com/xbpk3t/docs-alfred/pkg/httputil"
	"github.com/xbpk3t/docs-alfred/pkg/urlutil"
)

// ContentFetchResult holds fetched content metadata and body.
type ContentFetchResult struct {
	Title     string `json:"title"`
	Body      string `json:"body"`
	SourceURL string `json:"sourceUrl"`
	Error     string `json:"error,omitempty"`
}

// Fetcher handles fetching content from various sources.
type Fetcher struct {
	GHClient    *http.Client
	HTTPClient  *http.Client
	GHBaseURL   string
	MaxBodySize int
}

// NewFetcher creates a new Fetcher with default settings.
func NewFetcher() *Fetcher {
	return &Fetcher{
		GHClient:    httputil.NewClient(30 * time.Second),
		HTTPClient:  httputil.NewClient(30 * time.Second),
		GHBaseURL:   "https://api.github.com",
		MaxBodySize: 5000,
	}
}

// FetchContent fetches content based on the URL pattern.
// Supports GitHub repos, YouTube, Bilibili, and generic HTTP pages.
func (f *Fetcher) FetchContent(ctx context.Context, urlStr, contentType string) *ContentFetchResult {
	slog.Info("FetchContent", "url", urlStr, "type", contentType)

	u := strings.ToLower(urlStr)
	if _, ok := urlutil.GitHubOwnerRepo(urlStr); ok {
		return f.fetchGitHubRepo(ctx, urlStr)
	}

	switch {
	case strings.Contains(u, "youtube.com") || strings.Contains(u, "youtu.be"):
		return &ContentFetchResult{Title: "YouTube Video", SourceURL: urlStr, Body: "Video content requires manual review."}
	case strings.Contains(u, "bilibili.com"):
		return &ContentFetchResult{Title: "Bilibili Video", SourceURL: urlStr, Body: "Video content requires manual review."}
	default:
		return f.fetchHTTPPage(ctx, urlStr)
	}
}

// fetchGitHubRepo fetches GitHub repository information via the API.
func (f *Fetcher) fetchGitHubRepo(ctx context.Context, rawURL string) *ContentFetchResult {
	repoRef, ok := urlutil.GitHubOwnerRepo(rawURL)
	if !ok {
		return &ContentFetchResult{SourceURL: rawURL, Error: "not a valid GitHub repo URL"}
	}
	owner, repoName := repoRef.Owner, repoRef.Name

	ghClient, err := f.githubClient()
	if err != nil {
		return &ContentFetchResult{SourceURL: rawURL, Error: err.Error()}
	}
	repoData, _, err := ghClient.Repositories.Get(ctx, owner, repoName)
	if err != nil {
		return &ContentFetchResult{SourceURL: rawURL, Error: err.Error()}
	}

	licenseName := noneVal
	if repoData.GetLicense() != nil && repoData.GetLicense().GetSPDXID() != "" {
		licenseName = repoData.GetLicense().GetSPDXID()
	}

	body := fmt.Sprintf(`Repository: %s/%s
		Stars: %d
		Language: %s
		License: %s
		Topics: %s
		Description: %s
		URL: %s`,
		owner, repoName,
		repoData.GetStargazersCount(),
		repoData.GetLanguage(),
		licenseName,
		strings.Join(repoData.Topics, ", "),
		repoData.GetDescription(),
		repoData.GetHTMLURL(),
	)

	title := fmt.Sprintf("%s/%s", owner, repoName)
	if repoData.GetDescription() != "" {
		title = fmt.Sprintf("%s/%s — %s", owner, repoName, repoData.GetDescription())
	}

	return &ContentFetchResult{Title: title, Body: body, SourceURL: rawURL}
}

func (f *Fetcher) githubClient() (*github.Client, error) {
	client := github.NewClient(f.GHClient)
	client.UserAgent = "rss2nl-wiki"

	baseURL := strings.TrimSpace(f.GHBaseURL)
	if baseURL != "" && baseURL != "https://api.github.com" && baseURL != "https://api.github.com/" {
		if !strings.HasSuffix(baseURL, "/") {
			baseURL += "/"
		}
		parsed, err := url.Parse(baseURL)
		if err != nil {
			return nil, fmt.Errorf("parse GitHub base URL: %w", err)
		}
		client.BaseURL = parsed
	}

	if token := getGHToken(); token != "" {
		client = client.WithAuthToken(token)
	}

	return client, nil
}

// fetchHTTPPage fetches a web page via plain HTTP GET.
// Falls back to opencli (browser-based) when the server blocks the request
// (e.g. zhihu ZSE anti-bot protection).
func (f *Fetcher) fetchHTTPPage(ctx context.Context, rawURL string) *ContentFetchResult {
	timeout := httputil.DefaultClientTimeout
	if f.HTTPClient != nil && f.HTTPClient.Timeout > 0 {
		timeout = f.HTTPClient.Timeout
	}
	data, err := httputil.GetBytes(ctx, rawURL, httputil.RequestOptions{Timeout: timeout})
	if err == nil {
		slog.Info("HTTP fetch succeeded", "url", rawURL, "bodyLen", len(data))

		if result := f.extractWithReadability(data, rawURL); result != nil {
			return result
		}

		// Fallback: goquery title + Markdown body from the original HTML.
		body := markdownFallbackBody(data)
		if len(body) > f.MaxBodySize {
			body = body[:f.MaxBodySize] + "..."
		}
		title := extractTitle(string(data))
		if title == "" {
			title = rawURL
		}

		return &ContentFetchResult{Title: title, Body: body, SourceURL: rawURL}
	}

	slog.Warn("HTTP fetch failed, trying opencli fallback", "url", rawURL, "error", err)

	// opencli fallback with a timeout so it can't hang
	// (e.g. if Chrome isn't running with the debug port).
	openCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	result := f.fetchWithOpenCLI(openCtx, rawURL)
	if result.Error == "" {
		return result
	}

	slog.Warn("opencli fallback also failed", "url", rawURL, "error", result.Error)

	// Distinguish between HTTP-level errors (anti-bot, 4xx -> resolve failure)
	// and network-level errors (DNS, timeout -> fetch failure).
	errorStr := err.Error()
	if isHTTPBlockError(err) {
		errorStr = "resolve: " + errorStr
	}

	return &ContentFetchResult{SourceURL: rawURL, Error: errorStr}
}

func markdownFallbackBody(data []byte) string {
	body, err := htmlutil.ToMarkdown(string(data))
	if err == nil && strings.TrimSpace(body) != "" {
		return body
	}

	return string(data)
}

// extractWithReadability uses go-readability to extract article content.
// Returns nil if extraction fails or produces empty content.
func (f *Fetcher) extractWithReadability(data []byte, rawURL string) *ContentFetchResult {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil
	}

	article, err := readability.FromReader(bytes.NewReader(data), parsedURL)
	if err != nil || article.Node == nil {
		return nil
	}

	var buf strings.Builder
	if err := article.RenderText(&buf); err != nil {
		return nil
	}

	body := buf.String()
	if strings.TrimSpace(body) == "" {
		return nil
	}

	if len(body) > f.MaxBodySize {
		body = body[:f.MaxBodySize] + "..."
	}

	title := article.Title()
	if title == "" {
		title = extractTitle(string(data))
	}
	if title == "" {
		title = rawURL
	}

	slog.Info("go-readability extraction succeeded", "url", rawURL, "bodyLen", len(body))

	return &ContentFetchResult{Title: title, Body: body, SourceURL: rawURL}
}

// isHTTPBlockError checks whether the error is from an HTTP status code
// (e.g. 403 anti-bot) rather than a network-level error.
func isHTTPBlockError(err error) bool {
	return strings.Contains(err.Error(), "HTTP ")
}

// fetchWithOpenCLI uses the opencli browser tool to extract page content.
// opencli drives a real Chrome window to handle JS-rendered pages.
func (f *Fetcher) fetchWithOpenCLI(ctx context.Context, rawURL string) *ContentFetchResult {
	// opencli web read writes to a file by default; --stdout pipes content.
	cmd := exec.CommandContext(ctx, "opencli", "web", "read",
		"--url", rawURL,
		"--stdout",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return &ContentFetchResult{SourceURL: rawURL,
			Error: fmt.Sprintf("opencli: %v (stderr: %s)", err, strings.TrimSpace(stderr.String()))}
	}

	body := stdout.String()
	if body == "" {
		return &ContentFetchResult{SourceURL: rawURL, Error: "opencli returned empty content"}
	}

	if len(body) > f.MaxBodySize*3 {
		body = body[:f.MaxBodySize*3] + "..."
	}

	// opencli web read returns Markdown; first heading line is the title.
	title := rawURL
	for line := range strings.SplitSeq(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			title = strings.TrimLeft(trimmed, "# ")

			break
		}
	}

	slog.Info("opencli fetch succeeded", "url", rawURL, "bodyLen", len(body))

	return &ContentFetchResult{Title: title, Body: body, SourceURL: rawURL}
}

// extractTitle extracts the <title> from HTML content using goquery.
func extractTitle(htmlContent string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return ""
	}

	return strings.TrimSpace(doc.Find("title").First().Text())
}

func getGHToken() string {
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token
	}

	return os.Getenv("GH_TOKEN")
}
