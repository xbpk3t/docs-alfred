package httputil

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetBytesSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "token", r.Header.Get("X-Test"))
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	body, err := GetBytes(context.Background(), server.URL, RequestOptions{
		Headers: map[string]string{"X-Test": "token"},
	})

	require.NoError(t, err)
	assert.Equal(t, "ok", string(body))
}

func TestGetBytesHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "bad", http.StatusTeapot)
	}))
	defer server.Close()

	_, err := GetBytes(context.Background(), server.URL, RequestOptions{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 418")
	assert.Contains(t, err.Error(), "bad")
}

func TestPostJSONWithResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		assert.Equal(t, "hello", payload["message"])
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	var result struct {
		OK bool `json:"ok"`
	}
	body, err := PostJSONWithResult(context.Background(), server.URL, map[string]string{"message": "hello"}, &result, RequestOptions{})

	require.NoError(t, err)
	assert.JSONEq(t, `{"ok":true}`, string(body))
	assert.True(t, result.OK)
}

func TestPostJSONWithResultRetries5xx(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts == 1 {
			http.Error(w, "try again", http.StatusBadGateway)

			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	var result struct {
		OK bool `json:"ok"`
	}
	_, err := PostJSONWithResult(context.Background(), server.URL, map[string]string{"message": "hello"}, &result, RequestOptions{MaxRetries: 2})

	require.NoError(t, err)
	assert.Equal(t, 2, attempts)
	assert.True(t, result.OK)
}

func TestStdHTTPClient_DefaultTimeout(t *testing.T) {
	c := StdHTTPClient(0)
	assert.NotNil(t, c)
	assert.Equal(t, DefaultClientTimeout, c.Timeout)
}

func TestStdHTTPClient_CustomTimeout(t *testing.T) {
	c := StdHTTPClient(5 * time.Second)
	assert.NotNil(t, c)
	assert.Equal(t, 5*time.Second, c.Timeout)
}

func TestNewRestyClient_Defaults(t *testing.T) {
	c := NewRestyClient(0, 0)
	assert.NotNil(t, c)
	assert.Equal(t, DefaultClientTimeout, c.GetClient().Timeout)
}

func TestNewRestyClient_CustomValues(t *testing.T) {
	c := NewRestyClient(10*time.Second, 5)
	assert.NotNil(t, c)
	assert.Equal(t, 10*time.Second, c.GetClient().Timeout)
}

func TestGetBytes_ConnectionRefused(t *testing.T) {
	// Use a port that's very unlikely to be in use
	_, err := GetBytes(context.Background(), "http://127.0.0.1:1", RequestOptions{MaxRetries: 0, Timeout: time.Second})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get")
}

func TestPostJSONWithResult_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer server.Close()

	_, err := PostJSONWithResult(context.Background(), server.URL, map[string]string{"key": "val"}, nil, RequestOptions{MaxRetries: 0})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 400")
	assert.Contains(t, err.Error(), "bad request")
}

func TestPostJSONWithResult_NilResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	body, err := PostJSONWithResult(context.Background(), server.URL, map[string]string{"key": "val"}, nil, RequestOptions{})
	require.NoError(t, err)
	assert.JSONEq(t, `{"ok":true}`, string(body))
}

func TestPostJSONWithResult_ConnectionRefused(t *testing.T) {
	_, err := PostJSONWithResult(context.Background(), "http://127.0.0.1:1", "payload", nil, RequestOptions{MaxRetries: 0, Timeout: time.Second})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "post")
}

func TestGetBytes_RetriesOn5xx(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts <= 1 {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	body, err := GetBytes(context.Background(), server.URL, RequestOptions{MaxRetries: 2})
	require.NoError(t, err)
	assert.Equal(t, "ok", string(body))
	assert.Equal(t, 2, attempts)
}

func TestGetBytes_WithTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("data"))
	}))
	defer server.Close()

	body, err := GetBytes(context.Background(), server.URL, RequestOptions{Timeout: 5 * time.Second})
	require.NoError(t, err)
	assert.Equal(t, "data", string(body))
}
