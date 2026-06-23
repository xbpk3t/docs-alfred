package litter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestZeroX0_Name(t *testing.T) {
	z := &ZeroX0{}
	assert.Equal(t, "zerox0", z.Name())
}

func TestZeroX0_InterfaceAssertion(t *testing.T) {
	var _ Uploader = (*ZeroX0)(nil)
}

func TestZeroX0_Upload_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.Header.Get("Content-Type"), "multipart/form-data")

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("https://0x0.st/abc123"))
	}))
	defer srv.Close()

	z := &ZeroX0{BaseURL: srv.URL}
	result, err := z.Upload(context.Background(), "test.txt", "hello")

	require.NoError(t, err)
	assert.Equal(t, "https://0x0.st/abc123", result.URL)
	assert.Equal(t, "zerox0", result.Driver)
}

func TestZeroX0_Upload_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("service unavailable"))
	}))
	defer srv.Close()

	z := &ZeroX0{BaseURL: srv.URL}
	_, err := z.Upload(context.Background(), "test.txt", "hello")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 503")
}

func TestZeroX0_Upload_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(""))
	}))
	defer srv.Close()

	z := &ZeroX0{BaseURL: srv.URL}
	_, err := z.Upload(context.Background(), "test.txt", "hello")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty response from 0x0.st")
}

func TestZeroX0_Upload_WhitespaceResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("   \n  "))
	}))
	defer srv.Close()

	z := &ZeroX0{BaseURL: srv.URL}
	_, err := z.Upload(context.Background(), "test.txt", "hello")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty response from 0x0.st")
}

func TestZeroX0_Upload_ConnectionError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close() // close immediately to cause connection error

	z := &ZeroX0{BaseURL: srv.URL}
	_, err := z.Upload(context.Background(), "test.txt", "hello")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "upload request")
}

func TestZeroX0_Upload_DefaultBaseURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("https://0x0.st/default"))
	}))
	defer srv.Close()

	orig := zerox0BaseURL
	zerox0BaseURL = srv.URL
	t.Cleanup(func() { zerox0BaseURL = orig })

	z := &ZeroX0{} // empty BaseURL triggers default path
	result, err := z.Upload(context.Background(), "test.txt", "hello")

	require.NoError(t, err)
	assert.Equal(t, "https://0x0.st/default", result.URL)
}

func TestZeroX0_Upload_EmptyContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("https://0x0.st/empty"))
	}))
	defer srv.Close()

	z := &ZeroX0{BaseURL: srv.URL}
	result, err := z.Upload(context.Background(), "empty.txt", "")

	require.NoError(t, err)
	assert.Equal(t, "https://0x0.st/empty", result.URL)
	assert.Equal(t, "zerox0", result.Driver)
}

func TestZeroX0_Upload_WithContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("https://0x0.st/x"))
	}))
	defer srv.Close()

	ctx := context.WithValue(context.Background(), testContextKey("key"), "val")
	z := &ZeroX0{BaseURL: srv.URL}
	result, err := z.Upload(ctx, "test.txt", "content")

	require.NoError(t, err)
	assert.Equal(t, "https://0x0.st/x", result.URL)
}
