package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestD1DatabaseIDFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wrangler.toml")
	content := `[[d1_databases]]
binding = "DB"
database_name = "xzb"
database_id = "abc123def456"
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))

	id, err := D1DatabaseID(path, "DB")
	require.NoError(t, err)
	require.Equal(t, "abc123def456", id)
}

func TestD1DatabaseIDBindingNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wrangler.toml")
	content := `[[d1_databases]]
binding = "DB"
database_name = "xzb"
database_id = "abc123def456"
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))

	_, err := D1DatabaseID(path, "OTHER")
	require.Error(t, err)
	require.Contains(t, err.Error(), "OTHER")
	require.Contains(t, err.Error(), "not found")
}

func TestD1DatabaseIDEmptyID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wrangler.toml")
	content := `[[d1_databases]]
binding = "DB"
database_name = "xzb"
database_id = ""
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))

	_, err := D1DatabaseID(path, "DB")
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty database_id")
}

func TestD1DatabaseIDPlaceholderID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wrangler.toml")
	content := `[[d1_databases]]
binding = "DB"
database_name = "xzb"
database_id = "TODO-abc123"
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))

	_, err := D1DatabaseID(path, "DB")
	require.Error(t, err)
	require.Contains(t, err.Error(), "placeholder database_id")
}

func TestD1DatabaseIDMultipleBindings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wrangler.toml")
	content := `[[d1_databases]]
binding = "DB"
database_name = "xzb"
database_id = "id1"

[[d1_databases]]
binding = "STAGING"
database_name = "xzb-staging"
database_id = "id2"
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))

	id, err := D1DatabaseID(path, "STAGING")
	require.NoError(t, err)
	require.Equal(t, "id2", id)
}

func TestD1DatabaseIDFileNotFound(t *testing.T) {
	_, err := D1DatabaseID("/nonexistent/wrangler.toml", "DB")
	require.Error(t, err)
}

func TestD1DatabaseIDInvalidTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wrangler.toml")
	require.NoError(t, os.WriteFile(path, []byte("{{invalid toml"), 0600))

	_, err := D1DatabaseID(path, "DB")
	require.Error(t, err)
}
