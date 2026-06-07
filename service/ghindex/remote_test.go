package ghindex

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager_Defaults(t *testing.T) {
	m := NewManager("", "")
	require.NotNil(t, m)
	assert.Equal(t, DefaultConfigPath, m.configPath)
	assert.Equal(t, DefaultConfigURL, m.configURL)
	assert.Equal(t, "https://cdn.lucc.dev/gh.yml", m.configURL)
	assert.Equal(t, DefaultMaxAge, m.maxAge)
}

func TestNewManager_CustomPaths(t *testing.T) {
	m := NewManager("/custom/path.yml", "https://custom.url/config.yml")
	require.NotNil(t, m)
	assert.Equal(t, "/custom/path.yml", m.configPath)
	assert.Equal(t, "https://custom.url/config.yml", m.configURL)
}

func TestSetTTL(t *testing.T) {
	m := NewManager("", "")
	m.SetTTL(1 * time.Hour)
	assert.Equal(t, 1*time.Hour, m.maxAge)
}

func TestLoad_CacheMiss(t *testing.T) {
	tmpDir := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusBadGateway)
	}))
	t.Cleanup(server.Close)

	m := NewManager(filepath.Join(tmpDir, "missing.yml"), server.URL)
	err := m.LoadWithCacheTTL()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config not cached and sync failed")
}

func TestManager_ConfigPath(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "gh.yml")
	m := NewManager(configPath, "https://nonexistent.example.com/config.yml")
	assert.Equal(t, configPath, m.configPath)
}

func TestManager_Filter(t *testing.T) {
	m := NewManager("", "")
	result := m.Filter("test")
	assert.Nil(t, result, "empty repos should return nil")
}

func TestLoadFromFile_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "gh.yml")
	require.NoError(t, os.WriteFile(configPath, []byte("invalid: [yaml\n"), 0644))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusBadGateway)
	}))
	t.Cleanup(server.Close)

	m := NewManager(configPath, server.URL)
	err := m.LoadWithCacheTTL()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cached config invalid and sync failed")
}

func TestLoadWithCacheTTLRefreshesInvalidFreshCache(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "gh.yml")
	require.NoError(t, os.WriteFile(configPath, []byte("<!doctype html><title>Redirecting</title>"), 0644))
	remote := validRemoteConfigYAML("refreshed")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(remote)
	}))
	t.Cleanup(server.Close)

	m := NewManager(configPath, server.URL)
	require.NoError(t, m.LoadWithCacheTTL())

	actual, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Equal(t, string(remote), string(actual))

	result := m.Filter("refreshed")
	require.Len(t, result, 1)
	assert.Equal(t, "https://github.com/acme/refreshed", result[0].URL)
}

func TestSyncRejectsInvalidRemoteWithoutOverwritingCache(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "gh.yml")
	existing := validRemoteConfigYAML("existing")
	require.NoError(t, os.WriteFile(configPath, existing, 0644))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("invalid: [yaml\n"))
	}))
	t.Cleanup(server.Close)

	m := NewManager(configPath, server.URL)
	err := m.Sync()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid remote config")

	actual, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Equal(t, string(existing), string(actual))
}

func TestSyncWritesValidatedRemoteConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "gh.yml")
	remote := validRemoteConfigYAML("remote")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(remote)
	}))
	t.Cleanup(server.Close)

	m := NewManager(configPath, server.URL)
	require.NoError(t, m.Sync())

	actual, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Equal(t, string(remote), string(actual))
}

func TestLoadWithCacheTTLUsesValidatedStaleCacheWhenRemoteFails(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "gh.yml")
	require.NoError(t, os.WriteFile(configPath, validRemoteConfigYAML("stale"), 0644))
	old := time.Now().Add(-2 * time.Hour)
	require.NoError(t, os.Chtimes(configPath, old, old))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusBadGateway)
	}))
	t.Cleanup(server.Close)

	m := NewManager(configPath, server.URL)
	m.SetTTL(0)
	require.NoError(t, m.LoadWithCacheTTL())

	result := m.Filter("stale")
	require.Len(t, result, 1)
	assert.Equal(t, "https://github.com/acme/stale", result[0].URL)
}

func validRemoteConfigYAML(name string) []byte {
	return []byte(`- type: tool
  tag: test
  repo:
    - url: https://github.com/acme/` + name + `
      des: ` + name + ` repository
`)
}
