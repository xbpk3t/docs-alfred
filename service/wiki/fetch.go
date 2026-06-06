package wiki

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"codeberg.org/readeck/go-readability/v2"
	"github.com/PuerkitoBio/goquery"
	"github.com/xbpk3t/docs-alfred/pkg/httputil"
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

	switch {
	case strings.HasPrefix(u, "https://github.com"):
		return f.fetchGitHubRepo(ctx, urlStr)
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
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return &ContentFetchResult{SourceURL: rawURL, Error: err.Error()}
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 {
		return &ContentFetchResult{SourceURL: rawURL, Error: "not a valid GitHub repo URL"}
	}
	owner, repo := parts[0], parts[1]

	apiURL := fmt.Sprintf("%s/repos/%s/%s", f.GHBaseURL, owner, repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, http.NoBody)
	if err != nil {
		return &ContentFetchResult{SourceURL: rawURL, Error: err.Error()}
	}

	token := getGHToken()
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "rss2nl-wiki")

	resp, err := f.GHClient.Do(req)
	if err != nil {
		return &ContentFetchResult{SourceURL: rawURL, Error: err.Error()}
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return &ContentFetchResult{SourceURL: rawURL, Error: fmt.Sprintf("GitHub API: HTTP %d: %s", resp.StatusCode, string(body))}
	}

	var repoData struct {
		License *struct {
			SPDXID string `json:"spdx_id"`
		} `json:"license"`
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Language    string   `json:"language"`
		URL         string   `json:"html_url"`
		Topics      []string `json:"topics"`
		Stars       int      `json:"stargazers_count"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&repoData); err != nil {
		return &ContentFetchResult{SourceURL: rawURL, Error: err.Error()}
	}

	licenseName := noneVal
	if repoData.License != nil {
		licenseName = repoData.License.SPDXID
	}

	body := fmt.Sprintf(`Repository: %s/%s
		Stars: %d
		Language: %s
		License: %s
		Topics: %s
		Description: %s
		URL: %s`,
		owner, repo,
		repoData.Stars,
		repoData.Language,
		licenseName,
		strings.Join(repoData.Topics, ", "),
		repoData.Description,
		repoData.URL,
	)

	title := fmt.Sprintf("%s/%s", owner, repo)
	if repoData.Description != "" {
		title = fmt.Sprintf("%s/%s — %s", owner, repo, repoData.Description)
	}

	return &ContentFetchResult{Title: title, Body: body, SourceURL: rawURL}
}

// fetchHTTPPage fetches a web page via plain HTTP GET.
// Falls back to opencli (browser-based) when the server blocks the request
// (e.g. zhihu ZSE anti-bot protection).
func (f *Fetcher) fetchHTTPPage(ctx context.Context, rawURL string) *ContentFetchResult {
	data, err := httputil.Get(f.HTTPClient, rawURL)
	if err == nil {
		slog.Info("HTTP fetch succeeded", "url", rawURL, "bodyLen", len(data))

		if result := f.extractWithReadability(data, rawURL); result != nil {
			return result
		}

		// Fallback: goquery title + truncated raw body
		body := string(data)
		if len(body) > f.MaxBodySize {
			body = body[:f.MaxBodySize] + "..."
		}
		title := extractTitle(body)
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
