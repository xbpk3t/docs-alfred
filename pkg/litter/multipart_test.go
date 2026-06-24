package litter

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildMultipart_BasicFile(t *testing.T) {
	buf, contentType, err := buildMultipart("file", "test.txt", "hello world", nil)

	require.NoError(t, err)
	assert.NotEmpty(t, buf)
	assert.Contains(t, contentType, "multipart/form-data")
	assert.Contains(t, buf.String(), "hello world")
	assert.Contains(t, buf.String(), "test.txt")
}

func TestBuildMultipart_WithExtraFields(t *testing.T) {
	extra := map[string]string{
		"reqtype": "fileupload",
		"time":    "1h",
	}
	buf, contentType, err := buildMultipart("fileToUpload", "data.csv", "col1,col2", extra)

	require.NoError(t, err)
	assert.Contains(t, contentType, "multipart/form-data")
	body := buf.String()
	assert.Contains(t, body, "reqtype")
	assert.Contains(t, body, "fileupload")
	assert.Contains(t, body, "time")
	assert.Contains(t, body, "1h")
	assert.Contains(t, body, "data.csv")
	assert.Contains(t, body, "col1,col2")
}

func TestBuildMultipart_EmptyContent(t *testing.T) {
	buf, contentType, err := buildMultipart("file", "empty.txt", "", nil)

	require.NoError(t, err)
	assert.NotEmpty(t, buf)
	assert.Contains(t, contentType, "multipart/form-data")
	assert.Contains(t, buf.String(), "empty.txt")
}

func TestBuildMultipart_EmptyFilename(t *testing.T) {
	buf, contentType, err := buildMultipart("file", "", "content", nil)

	require.NoError(t, err)
	assert.NotEmpty(t, buf)
	assert.Contains(t, contentType, "multipart/form-data")
}

func TestBuildMultipart_ContentTypeHasBoundary(t *testing.T) {
	_, contentType, err := buildMultipart("file", "test.txt", "content", map[string]string{"key": "val"})
	require.NoError(t, err)
	assert.Contains(t, contentType, "boundary=")
}

func TestBuildMultipart_LargeContent(t *testing.T) {
	large := strings.Repeat("x", 1024*100) // 100KB
	buf, contentType, err := buildMultipart("file", "large.bin", large, nil)

	require.NoError(t, err)
	assert.NotEmpty(t, buf)
	assert.Contains(t, contentType, "multipart/form-data")
	assert.Contains(t, buf.String(), large)
}

// writeLimitWriter succeeds for the first N writes, then returns an error.
type writeLimitWriter struct {
	limit   int
	written int
}

func (w *writeLimitWriter) Write(p []byte) (int, error) {
	w.written++
	if w.written > w.limit {
		return 0, errors.New("write failed")
	}

	return len(p), nil
}

func TestBuildMultipartTo_WriteFieldError(t *testing.T) {
	w := &writeLimitWriter{limit: 0} // first write fails
	_, err := buildMultipartTo(w, "file", "test.txt", "content", map[string]string{"key": "val"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "write field")
}

func TestBuildMultipartTo_CreateFormFileError(t *testing.T) {
	w := &writeLimitWriter{limit: 0} // first write fails
	_, err := buildMultipartTo(w, "file", "test.txt", "content", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create form file")
}

func TestBuildMultipartTo_CopyError(t *testing.T) {
	// CreateFormFile emits 1 write (all headers combined);
	// limit=1 lets it succeed but the content write in io.Copy fails.
	w := &writeLimitWriter{limit: 1}
	_, err := buildMultipartTo(w, "file", "test.txt", "content", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "copy content")
}

func TestBuildMultipartTo_CloseError(t *testing.T) {
	// CreateFormFile (1 write) + content write (1 write) = 2; Close fails on 3rd.
	w := &writeLimitWriter{limit: 2}
	_, err := buildMultipartTo(w, "file", "test.txt", "content", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "close multipart")
}
