package litter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileIO_Name(t *testing.T) {
	f := &FileIO{}
	assert.Equal(t, "fileio", f.Name())
}

func TestFileIO_InterfaceAssertion(t *testing.T) {
	var _ Uploader = (*FileIO)(nil)
}

func TestFileIO_Upload_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.Header.Get("Content-Type"), "multipart/form-data")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true,"link":"https://file.io/abc123"}`))
	}))
	defer srv.Close()

	f := &FileIO{BaseURL: srv.URL}
	result, err := f.Upload(context.Background(), "test.txt", "hello")

	require.NoError(t, err)
	assert.Equal(t, "https://file.io/abc123", result.URL)
	assert.Equal(t, "fileio", result.Driver)
}

func TestFileIO_Upload_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	f := &FileIO{BaseURL: srv.URL}
	_, err := f.Upload(context.Background(), "test.txt", "hello")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 500")
}

func TestFileIO_Upload_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	f := &FileIO{BaseURL: srv.URL}
	_, err := f.Upload(context.Background(), "test.txt", "hello")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse response")
}

func TestFileIO_Upload_SuccessFalse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":false,"message":"file too large"}`))
	}))
	defer srv.Close()

	f := &FileIO{BaseURL: srv.URL}
	_, err := f.Upload(context.Background(), "test.txt", "hello")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "file.io error")
	assert.Contains(t, err.Error(), "file too large")
}

func TestFileIO_Upload_EmptyLink(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true,"link":""}`))
	}))
	defer srv.Close()

	f := &FileIO{BaseURL: srv.URL}
	_, err := f.Upload(context.Background(), "test.txt", "hello")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "file.io error")
}

func TestFileIO_Upload_WithContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true,"link":"https://file.io/x"}`))
	}))
	defer srv.Close()

	ctx := context.WithValue(context.Background(), testContextKey("key"), "val")
	f := &FileIO{BaseURL: srv.URL}
	result, err := f.Upload(ctx, "test.txt", "content")

	require.NoError(t, err)
	assert.Equal(t, "https://file.io/x", result.URL)
}

func TestFileIO_Upload_EmptyContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true,"link":"https://file.io/empty"}`))
	}))
	defer srv.Close()

	f := &FileIO{BaseURL: srv.URL}
	result, err := f.Upload(context.Background(), "empty.txt", "")

	require.NoError(t, err)
	assert.Equal(t, "https://file.io/empty", result.URL)
}

func TestFileIO_Upload_ConnectionError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close() // close immediately to cause connection error

	f := &FileIO{BaseURL: srv.URL}
	_, err := f.Upload(context.Background(), "test.txt", "hello")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "upload request")
}

func TestFileIO_Upload_DefaultBaseURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true,"link":"https://file.io/default"}`))
	}))
	defer srv.Close()

	orig := fileioBaseURL
	fileioBaseURL = srv.URL
	t.Cleanup(func() { fileioBaseURL = orig })

	f := &FileIO{} // empty BaseURL triggers default path
	result, err := f.Upload(context.Background(), "test.txt", "hello")

	require.NoError(t, err)
	assert.Equal(t, "https://file.io/default", result.URL)
}

// testContextKey is a custom type for context keys in tests to avoid collisions.
type testContextKey string
