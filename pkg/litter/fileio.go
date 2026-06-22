package litter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/xbpk3t/docs-alfred/pkg/httputil"
)

const fileioBaseURL = "https://file.io"

// FileIO uploads files to file.io (temporary, up to 4 GB, 14-day default expiry).
type FileIO struct{}

func (f *FileIO) Name() string { return "fileio" }

func (f *FileIO) Upload(ctx context.Context, filename, content string) (*Result, error) {
	buf, contentType, err := buildMultipart("file", filename, content, nil)
	if err != nil {
		return nil, err
	}

	resp, err := httputil.NewRestyClient(httputil.DefaultClientTimeout, httputil.DefaultMaxRetries).
		R().
		SetContext(ctx).
		SetHeader("Content-Type", contentType).
		SetBody(buf).
		Post(fileioBaseURL)
	if err != nil {
		return nil, fmt.Errorf("upload request: %w", err)
	}

	body := resp.Body()
	if resp.StatusCode() != http.StatusOK {
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
