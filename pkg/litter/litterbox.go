package litter

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"strings"

	"github.com/xbpk3t/docs-alfred/pkg/httputil"
)

const (
	litterboxBaseURL = "https://litterbox.catbox.moe"
	litterboxAPIPath = "/resources/internals/api.php"
)

// Litterbox uploads files to litterbox.catbox.moe (temporary, up to 1 GB).
type Litterbox struct {
	Expiration string // "1h", "12h", "24h", "72h"
}

// NewLitterbox creates a Litterbox uploader. Empty expiration defaults to "72h".
func NewLitterbox(expiration string) *Litterbox {
	valid := map[string]bool{"1h": true, "12h": true, "24h": true, "72h": true}
	if !valid[expiration] {
		expiration = "72h"
	}

	return &Litterbox{Expiration: expiration}
}

func (l *Litterbox) Name() string { return "litterbox" }

func (l *Litterbox) Upload(ctx context.Context, filename, content string) (*Result, error) {
	buf, contentType, err := buildLitterboxMultipart(filename, content, l.Expiration)
	if err != nil {
		return nil, err
	}

	resp, err := httputil.NewRestyClient(httputil.DefaultClientTimeout, httputil.DefaultMaxRetries).
		R().
		SetContext(ctx).
		SetHeader("Content-Type", contentType).
		SetBody(buf).
		Post(litterboxBaseURL + litterboxAPIPath)
	if err != nil {
		return nil, fmt.Errorf("upload request: %w", err)
	}

	body := resp.Body()
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("upload failed (HTTP %d): %s", resp.StatusCode(), string(body))
	}

	url := strings.TrimSpace(string(body))
	if url == "" {
		return nil, errors.New("empty response from litterbox")
	}

	return &Result{URL: url, Driver: "litterbox", Expiration: l.Expiration}, nil
}

func buildLitterboxMultipart(filename, content, expiration string) (*bytes.Buffer, string, error) {
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
