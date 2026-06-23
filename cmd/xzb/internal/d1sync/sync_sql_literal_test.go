package d1sync

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cloudflare/cloudflare-go/v7"
	"github.com/cloudflare/cloudflare-go/v7/option"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// sqlLiteral direct tests (unexported, only reachable from same package)
// ---------------------------------------------------------------------------

func TestSQLLiteralNil(t *testing.T) {
	require.Equal(t, "NULL", sqlLiteral(nil))
}

func TestSQLLiteralBoolTrue(t *testing.T) {
	require.Equal(t, "1", sqlLiteral(true))
}

func TestSQLLiteralBoolFalse(t *testing.T) {
	require.Equal(t, "0", sqlLiteral(false))
}

func TestSQLLiteralDefaultType(t *testing.T) {
	require.Equal(t, "'3.14'", sqlLiteral(3.14))
}

func TestSQLLiteralDefaultTypeWithQuote(t *testing.T) {
	require.Equal(t, "'it''s a float'", sqlLiteral("it's a float"))
}

// ---------------------------------------------------------------------------
// renderSQL direct tests
// ---------------------------------------------------------------------------

func TestRenderSQLExcessPlaceholders(t *testing.T) {
	result := renderSQL("SELECT ?, ?, ?", []any{"a"})
	require.Equal(t, "SELECT 'a', ?, ?", result)
}

func TestRenderSQLNoParams(t *testing.T) {
	result := renderSQL("SELECT 1", nil)
	require.Equal(t, "SELECT 1", result)
}

// ---------------------------------------------------------------------------
// Query method via mock HTTP server
// ---------------------------------------------------------------------------

func TestQuerySuccessWithMockServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{
			"result": [{
				"meta": {
					"rows_written": 5,
					"rows_read": 10,
					"duration": 0.5
				},
				"results": [],
				"success": true
			}],
			"success": true,
			"errors": [],
			"messages": []
		}`)
	}))
	defer srv.Close()

	client := cloudflare.NewClient(
		option.WithAPIToken("test-token"),
		option.WithBaseURL(srv.URL+"/"),
		option.WithMaxRetries(0),
	)
	q := &CloudflareQueryer{
		client:     client,
		AccountID:  "test-account",
		DatabaseID: "test-db",
	}

	result, err := q.Query(context.Background(), "INSERT INTO t VALUES (?)", []any{"hello"})
	require.NoError(t, err)
	require.Equal(t, int64(5), result.RowsWritten)
}

func TestQueryMultipleResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{
			"result": [
				{"meta": {"rows_written": 2}, "results": [], "success": true},
				{"meta": {"rows_written": 3}, "results": [], "success": true}
			],
			"success": true,
			"errors": [],
			"messages": []
		}`)
	}))
	defer srv.Close()

	client := cloudflare.NewClient(
		option.WithAPIToken("test-token"),
		option.WithBaseURL(srv.URL+"/"),
		option.WithMaxRetries(0),
	)
	q := &CloudflareQueryer{
		client:     client,
		AccountID:  "test-account",
		DatabaseID: "test-db",
	}

	result, err := q.Query(context.Background(), "SELECT 1", nil)
	require.NoError(t, err)
	require.Equal(t, int64(5), result.RowsWritten) // 2 + 3
}

func TestQueryServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprint(w, `{"success": false, "errors": [{"code": 503, "message": "service unavailable"}]}`)
	}))
	defer srv.Close()

	client := cloudflare.NewClient(
		option.WithAPIToken("test-token"),
		option.WithBaseURL(srv.URL+"/"),
		option.WithMaxRetries(0),
	)
	q := &CloudflareQueryer{
		client:     client,
		AccountID:  "test-account",
		DatabaseID: "test-db",
	}

	_, err := q.Query(context.Background(), "SELECT 1", nil)
	require.Error(t, err)
}
