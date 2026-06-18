package wiki

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"codeberg.org/readeck/go-readability/v2"
	"github.com/PuerkitoBio/goquery"
	"github.com/xbpk3t/docs-alfred/pkg/htmlutil"
	"github.com/xbpk3t/docs-alfred/pkg/httputil"
	"github.com/xbpk3t/docs-alfred/pkg/textutil"
)

// httpDriver fetches content via plain HTTP GET + readability extraction.
// Suitable for VPS environments without a local browser.
type httpDriver struct {
	httpClient  *http.Client
	maxBodySize int
}

func newHTTPDriver(opts DriverOptions) *httpDriver {
	maxBody := opts.MaxBodySize
	if maxBody <= 0 {
		maxBody = 5000
	}

	return &httpDriver{
		maxBodySize: maxBody,
		httpClient:  httputil.NewClient(30 * time.Second),
	}
}

func (d *httpDriver) Name() string { return "http-readability" }

func (d *httpDriver) FetchContent(ctx context.Context, urlStr, contentType string) *ContentFetchResult {
	timeout := httputil.DefaultClientTimeout
	if d.httpClient != nil && d.httpClient.Timeout > 0 {
		timeout = d.httpClient.Timeout
	}
	data, err := httputil.GetBytes(ctx, urlStr, httputil.RequestOptions{Timeout: timeout})
	if err != nil {
		failureKind := FailureFetch
		errorStr := err.Error()
		if isHTTPBlockError(err) {
			failureKind = FailureResolve
			errorStr = "resolve: " + errorStr
		}

		return &ContentFetchResult{SourceURL: urlStr, Error: errorStr, FailureKind: failureKind}
	}

	return d.handlePageData(ctx, urlStr, data)
}

func (d *httpDriver) handlePageData(ctx context.Context, rawURL string, data []byte) *ContentFetchResult {
	slog.Info("HTTP fetch succeeded", "url", rawURL, "bodyLen", len(data))
	if result := d.extractWithReadability(data, rawURL); result != nil {
		return d.ensureContentQuality(ctx, result)
	}

	body := textutil.TruncateUTF8(markdownFallbackBody(data), d.maxBodySize)
	title := extractTitleFromHTML(string(data))
	if title == "" {
		title = rawURL
	}

	return d.ensureContentQuality(ctx, &ContentFetchResult{Title: title, Body: body, SourceURL: rawURL})
}

func (d *httpDriver) extractWithReadability(data []byte, rawURL string) *ContentFetchResult {
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

	body = textutil.TruncateUTF8(body, d.maxBodySize)

	title := article.Title()
	if title == "" {
		title = extractTitleFromHTML(string(data))
	}
	if title == "" {
		title = rawURL
	}

	slog.Info("go-readability extraction succeeded", "url", rawURL, "bodyLen", len(body))

	return &ContentFetchResult{Title: title, Body: body, SourceURL: rawURL}
}

func (d *httpDriver) ensureContentQuality(_ context.Context, result *ContentFetchResult) *ContentFetchResult {
	if result == nil || result.Error != "" {
		return result
	}
	quality := assessContentQuality(result.Title, result.Body, result.SourceURL)
	if quality.OK {
		return result
	}

	return extractFailure(result.SourceURL, "low quality HTTP content: "+quality.Reason)
}

// extractTitleFromHTML extracts the <title> from HTML content using goquery.
func extractTitleFromHTML(htmlContent string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return ""
	}

	return strings.TrimSpace(doc.Find("title").First().Text())
}

func isHTTPBlockError(err error) bool {
	return strings.Contains(err.Error(), "HTTP ")
}

func markdownFallbackBody(data []byte) string {
	body, err := htmlutil.ToMarkdown(string(data))
	if err == nil && strings.TrimSpace(body) != "" {
		return body
	}

	return string(data)
}
