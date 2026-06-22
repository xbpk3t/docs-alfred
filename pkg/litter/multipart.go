package litter

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"strings"
)

// buildMultipart constructs a multipart/form-data body with an optional file
// field and any number of extra text fields. It returns the buffer, the
// Content-Type header value, and any error encountered during construction.
func buildMultipart(fieldName, filename, content string, extraFields map[string]string) (*bytes.Buffer, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	for key, val := range extraFields {
		if err := w.WriteField(key, val); err != nil {
			return nil, "", fmt.Errorf("write field %s: %w", key, err)
		}
	}

	fw, err := w.CreateFormFile(fieldName, filename)
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
