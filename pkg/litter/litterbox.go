package litter

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/xbpk3t/docs-alfred/pkg/httputil"
)

var (
	litterboxBaseURL = "https://litterbox.catbox.moe"
	litterboxAPIPath = "/resources/internals/api.php"
)

// Litterbox uploads files to litterbox.catbox.moe (temporary, up to 1 GB).
type Litterbox struct {
	Expiration string // "1h", "12h", "24h", "72h"
	// BaseURL overrides the default litterbox endpoint. Empty uses the production URL.
	BaseURL string
}

// NewLitterbox creates a Litterbox uploader. Empty expiration defaults to "72h".
func NewLitterbox(expiration string) *Litterbox {
	valid := map[string]bool{"1h": true, "12h": true, "24h": true, "72h": true}
	if !valid[expiration] {
		expiration = "72h"
	}

	return &Litterbox{Expiration: expiration}
}

func (l *Litterbox) Name() string { return driverLitterbox }

func (l *Litterbox) Upload(ctx context.Context, filename, content string) (*Result, error) {
	extra := map[string]string{
		"reqtype": "fileupload",
		"time":    l.Expiration,
	}
	buf, contentType, err := buildMultipart("fileToUpload", filename, content, extra)
	if err != nil {
		return nil, err
	}

	base := l.BaseURL
	if base == "" {
		base = litterboxBaseURL + litterboxAPIPath
	}

	resp, err := httputil.NewRestyClient(httputil.DefaultClientTimeout, httputil.DefaultMaxRetries).
		R().
		SetContext(ctx).
		SetHeader("Content-Type", contentType).
		SetBody(buf).
		Post(base)
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

	return &Result{URL: url, Driver: driverLitterbox, Expiration: l.Expiration}, nil
}
