package httputil

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
