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
	contentType, err := buildMultipartTo(&buf, fieldName, filename, content, extraFields)
	if err != nil {
		return nil, "", err
	}

	return &buf, contentType, nil
}

// buildMultipartTo writes a multipart/form-data body into dst. It is the
// testable core of buildMultipart: callers can supply a failing writer to
// exercise error paths that bytes.Buffer never triggers.
func buildMultipartTo(dst io.Writer, fieldName, filename, content string, extraFields map[string]string) (string, error) {
	w := multipart.NewWriter(dst)

	for key, val := range extraFields {
		if err := w.WriteField(key, val); err != nil {
			return "", fmt.Errorf("write field %s: %w", key, err)
		}
	}

	fw, err := w.CreateFormFile(fieldName, filename)
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}
	if _, copyErr := io.Copy(fw, strings.NewReader(content)); copyErr != nil {
		return "", fmt.Errorf("copy content: %w", copyErr)
	}
	if closeErr := w.Close(); closeErr != nil {
		return "", fmt.Errorf("close multipart: %w", closeErr)
	}

	return w.FormDataContentType(), nil
}
