package litter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"strings"

	"github.com/xbpk3t/docs-alfred/pkg/httputil"
)

const fileioBaseURL = "https://file.io"

// FileIO uploads files to file.io (temporary, up to 4 GB, 14-day default expiry).
type FileIO struct{}

func (f *FileIO) Name() string { return "fileio" }

func (f *FileIO) Upload(ctx context.Context, filename, content string) (*Result, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, copyErr := io.Copy(fw, strings.NewReader(content)); copyErr != nil {
		return nil, fmt.Errorf("copy content: %w", copyErr)
	}
	if closeErr := w.Close(); closeErr != nil {
		return nil, fmt.Errorf("close multipart: %w", closeErr)
	}

	resp, err := httputil.NewRestyClient(httputil.DefaultClientTimeout, httputil.DefaultMaxRetries).
		R().
		SetContext(ctx).
		SetHeader("Content-Type", w.FormDataContentType()).
		SetBody(&buf).
		Post(fileioBaseURL)
	if err != nil {
		return nil, fmt.Errorf("upload request: %w", err)
	}

	body := resp.Body()
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("upload failed (HTTP %d): %s", resp.StatusCode(), string(body))
	}

	var result struct {
		Link    string `json:"link"`
		Message string `json:"message,omitempty"`
		Error   int    `json:"error,omitempty"`
		Success bool   `json:"success"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w (body: %s)", err, strings.TrimSpace(string(body)))
	}
	if !result.Success || result.Link == "" {
		return nil, fmt.Errorf("file.io error: %s", result.Message)
	}

	return &Result{URL: result.Link, Driver: "fileio"}, nil
}
