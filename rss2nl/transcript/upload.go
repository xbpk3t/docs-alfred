package transcript

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/xbpk3t/docs-alfred/pkg/httputil"
)

// LitterboxUploader uploads transcripts to litterbox for temporary sharing.
type LitterboxUploader struct {
	HTTPClient *http.Client
	BaseURL    string
	Expiration string
}

// NewLitterboxUploader creates an uploader with the given expiration duration.
func NewLitterboxUploader(expiration string) *LitterboxUploader {
	if expiration == "" {
		expiration = "24h"
	}
	// Validate expiration
	valid := map[string]bool{"1h": true, "12h": true, "24h": true, "72h": true}
	if !valid[expiration] {
		expiration = "24h"
	}

	return &LitterboxUploader{
		BaseURL:    "https://litterbox.catbox.moe",
		Expiration: expiration,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// UploadResult holds the result of a Litterbox upload.
type UploadResult struct {
	URL        string `json:"url"`
	FileID     string `json:"fileId"`
	Expiration string `json:"expiration"`
}

// Upload uploads content to Litterbox.
func (u *LitterboxUploader) Upload(ctx context.Context, filename, content string) (*UploadResult, error) {
	buf, contentType, err := buildMultipart(filename, content, u.Expiration)
	if err != nil {
		return nil, err
	}

	timeout := httputil.DefaultClientTimeout
	if u.HTTPClient != nil && u.HTTPClient.Timeout > 0 {
		timeout = u.HTTPClient.Timeout
	}

	resp, err := httputil.NewRestyClient(timeout, httputil.DefaultMaxRetries).
		R().
		SetContext(ctx).
		SetHeader("Content-Type", contentType).
		SetBody(buf).
		Post(u.BaseURL + "/resources/litterbox/upload.php")
	if err != nil {
		return nil, fmt.Errorf("upload request: %w", err)
	}
	body := resp.Body()
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("upload failed (HTTP %d): %s", resp.StatusCode(), string(body))
	}

	url := strings.TrimSpace(string(body))
	if url == "" {
		return nil, errors.New("empty response from litterbox")
	}

	return &UploadResult{
		URL:        url,
		FileID:     parseFileID(url),
		Expiration: u.Expiration,
	}, nil
}

func buildMultipart(filename, content, expiration string) (*bytes.Buffer, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	if err := w.WriteField("reqtype", "fileupload"); err != nil {
		return nil, "", fmt.Errorf("write reqtype: %w", err)
	}
	if err := w.WriteField("time", expiration); err != nil {
		return nil, "", fmt.Errorf("write time: %w", err)
	}
	fw, err := w.CreateFormFile("fileToUpload", filename)
	if err != nil {
		return nil, "", fmt.Errorf("create form file: %w", err)
	}
	if _, copyErr := io.Copy(fw, strings.NewReader(content)); copyErr != nil {
		return nil, "", fmt.Errorf("copy content: %w", copyErr)
	}
	if closeErr := w.Close(); closeErr != nil {
		return nil, "", fmt.Errorf("close multipart: %w", closeErr)
	}

	return &buf, w.FormDataContentType(), nil
}

func parseFileID(url string) string {
	parts := strings.Split(strings.TrimRight(url, "/"), "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}

	return ""
}
