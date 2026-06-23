package litter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLitterbox_ValidExpiration(t *testing.T) {
	tests := []struct {
		name       string
		expiration string
		want       string
	}{
		{"1h", "1h", "1h"},
		{"12h", "12h", "12h"},
		{"24h", "24h", "24h"},
		{"72h", "72h", "72h"},
		{"empty defaults to 72h", "", "72h"},
		{"invalid defaults to 72h", "5h", "72h"},
		{"random string defaults to 72h", "foobar", "72h"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := NewLitterbox(tt.expiration)
			assert.Equal(t, tt.want, l.Expiration)
		})
	}
}

func TestLitterbox_Name(t *testing.T) {
	l := NewLitterbox("1h")
	assert.Equal(t, "litterbox", l.Name())
}

func TestLitterbox_InterfaceAssertion(t *testing.T) {
	var _ Uploader = (*Litterbox)(nil)
}

func TestLitterbox_Upload_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.Header.Get("Content-Type"), "multipart/form-data")

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("https://litter.catbox.moe/abc123.txt"))
	}))
	defer srv.Close()

	l := &Litterbox{Expiration: "1h", BaseURL: srv.URL}
	result, err := l.Upload(context.Background(), "test.txt", "hello")

	require.NoError(t, err)
	assert.Equal(t, "https://litter.catbox.moe/abc123.txt", result.URL)
	assert.Equal(t, "litterbox", result.Driver)
	assert.Equal(t, "1h", result.Expiration)
}

func TestLitterbox_Upload_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("service unavailable"))
	}))
	defer srv.Close()

	l := &Litterbox{Expiration: "24h", BaseURL: srv.URL}
	_, err := l.Upload(context.Background(), "test.txt", "hello")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 503")
}

func TestLitterbox_Upload_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(""))
	}))
	defer srv.Close()

	l := &Litterbox{Expiration: "72h", BaseURL: srv.URL}
	_, err := l.Upload(context.Background(), "test.txt", "hello")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty response from litterbox")
}

func TestLitterbox_Upload_WhitespaceOnlyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("   \n  "))
	}))
	defer srv.Close()

	l := &Litterbox{Expiration: "12h", BaseURL: srv.URL}
	_, err := l.Upload(context.Background(), "test.txt", "hello")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty response from litterbox")
}

func TestLitterbox_Upload_ConnectionError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close() // close immediately to cause connection error

	l := &Litterbox{Expiration: "1h", BaseURL: srv.URL}
	_, err := l.Upload(context.Background(), "test.txt", "hello")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "upload request")
}

func TestLitterbox_Upload_DefaultBaseURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("https://litter.catbox.moe/default.txt"))
	}))
	defer srv.Close()

	origBase := litterboxBaseURL
	origPath := litterboxAPIPath
	litterboxBaseURL = srv.URL
	litterboxAPIPath = ""
	t.Cleanup(func() {
		litterboxBaseURL = origBase
		litterboxAPIPath = origPath
	})

	l := &Litterbox{Expiration: "1h"} // empty BaseURL triggers default path
	result, err := l.Upload(context.Background(), "test.txt", "hello")

	require.NoError(t, err)
	assert.Equal(t, "https://litter.catbox.moe/default.txt", result.URL)
}

func TestLitterbox_Upload_EmptyContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("https://litter.catbox.moe/empty.txt"))
	}))
	defer srv.Close()

	l := &Litterbox{Expiration: "1h", BaseURL: srv.URL}
	result, err := l.Upload(context.Background(), "empty.txt", "")

	require.NoError(t, err)
	assert.Equal(t, "https://litter.catbox.moe/empty.txt", result.URL)
}
