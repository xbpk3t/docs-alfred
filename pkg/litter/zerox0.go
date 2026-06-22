package litter

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/xbpk3t/docs-alfred/pkg/httputil"
)

const zerox0BaseURL = "https://0x0.st"

// ZeroX0 uploads files to 0x0.st (temporary, up to 512 MB).
// Small files are retained longer; large files expire sooner.
type ZeroX0 struct{}

func (z *ZeroX0) Name() string { return "zerox0" }

func (z *ZeroX0) Upload(ctx context.Context, filename, content string) (*Result, error) {
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
		Post(zerox0BaseURL)
	if err != nil {
		return nil, fmt.Errorf("upload request: %w", err)
	}

	body := resp.Body()
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("upload failed (HTTP %d): %s", resp.StatusCode(), string(body))
	}

	url := strings.TrimSpace(string(body))
	if url == "" {
		return nil, errors.New("empty response from 0x0.st")
	}

	return &Result{URL: url, Driver: "zerox0"}, nil
}
