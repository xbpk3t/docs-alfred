package litter

import (
	"context"
	"errors"
	"fmt"
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
	buf, contentType, err := buildMultipart("file", filename, content, nil)
	if err != nil {
		return nil, err
	}

	resp, err := httputil.NewRestyClient(httputil.DefaultClientTimeout, httputil.DefaultMaxRetries).
		R().
		SetContext(ctx).
		SetHeader("Content-Type", contentType).
		SetBody(buf).
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
