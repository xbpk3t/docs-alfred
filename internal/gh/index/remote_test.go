package ghindex

import (
	"errors"
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

func TestNewManager_AppendsGhYMLToBaseURL(t *testing.T) {
	m := NewManager("", "https://cdn.lucc.dev/")
	require.NotNil(t, m)
	assert.Equal(t, "https://cdn.lucc.dev/gh.yml", m.configURL)
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
	require.Len(t, m.ConfigRepos(), 1)
	assert.Equal(t, "test", m.ConfigRepos()[0].Tag)
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

func TestLoadWithBackgroundSyncStartsProcessForStaleCache(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "gh.yml")
	require.NoError(t, os.WriteFile(configPath, validRemoteConfigYAML("cached"), 0644))
	old := time.Now().Add(-2 * time.Hour)
	require.NoError(t, os.Chtimes(configPath, old, old))

	previousStarter := backgroundSyncStarter
	t.Cleanup(func() { backgroundSyncStarter = previousStarter })

	var called bool
	backgroundSyncStarter = func(m *Manager) error {
		called = true
		assert.Equal(t, configPath, m.configPath)
		assert.Equal(t, "https://example.com/gh.yml", m.configURL)

		return nil
	}

	m := NewManager(configPath, "https://example.com/gh.yml")
	m.SetTTL(time.Hour)
	require.NoError(t, m.LoadWithBackgroundSync())
	assert.True(t, called)

	result := m.Filter("cached")
	require.Len(t, result, 1)
	assert.Equal(t, "https://github.com/acme/cached", result[0].URL)
}

func TestLoadWithBackgroundSyncUsesCacheWhenStarterFails(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "gh.yml")
	require.NoError(t, os.WriteFile(configPath, validRemoteConfigYAML("cached"), 0644))
	old := time.Now().Add(-2 * time.Hour)
	require.NoError(t, os.Chtimes(configPath, old, old))

	previousStarter := backgroundSyncStarter
	t.Cleanup(func() { backgroundSyncStarter = previousStarter })
	backgroundSyncStarter = func(m *Manager) error {
		return errors.New("starter unavailable")
	}

	m := NewManager(configPath, "https://example.com/gh.yml")
	m.SetTTL(time.Hour)
	require.NoError(t, m.LoadWithBackgroundSync())

	result := m.Filter("cached")
	require.Len(t, result, 1)
	assert.Equal(t, "https://github.com/acme/cached", result[0].URL)
}

func TestLoadWithBackgroundSync_FreshCache(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "gh.yml")
	require.NoError(t, os.WriteFile(configPath, validRemoteConfigYAML("fresh"), 0644))

	previousStarter := backgroundSyncStarter
	t.Cleanup(func() { backgroundSyncStarter = previousStarter })
	backgroundSyncStarter = func(m *Manager) error {
		t.Fatal("should not be called for fresh cache")
		return nil
	}

	m := NewManager(configPath, "https://example.com/gh.yml")
	m.SetTTL(24 * time.Hour)
	require.NoError(t, m.LoadWithBackgroundSync())

	result := m.Filter("fresh")
	require.Len(t, result, 1)
}

func TestIsBackgroundSyncAvailable(t *testing.T) {
	// "ls" should be available on macOS
	assert.True(t, IsBackgroundSyncAvailable("ls"))
	assert.False(t, IsBackgroundSyncAvailable("/nonexistent-binary-99999"))
}

func TestLoadWithCacheTTL_StaleSyncSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "gh.yml")
	// Write initial stale config
	require.NoError(t, os.WriteFile(configPath, validRemoteConfigYAML("stale"), 0644))
	old := time.Now().Add(-2 * time.Hour)
	require.NoError(t, os.Chtimes(configPath, old, old))

	remote := validRemoteConfigYAML("refreshed")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(remote)
	}))
	t.Cleanup(server.Close)

	m := NewManager(configPath, server.URL)
	m.SetTTL(0)
	require.NoError(t, m.LoadWithCacheTTL())

	result := m.Filter("refreshed")
	require.Len(t, result, 1)
	assert.Equal(t, "https://github.com/acme/refreshed", result[0].URL)
}

func TestLoadWithCacheTTL_FreshCacheInvalidSyncFails(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "gh.yml")
	// Fresh but invalid cache
	require.NoError(t, os.WriteFile(configPath, []byte("string_value"), 0644))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusBadGateway)
	}))
	t.Cleanup(server.Close)

	m := NewManager(configPath, server.URL)
	m.SetTTL(24 * time.Hour)
	err := m.LoadWithCacheTTL()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cached config invalid and sync failed")
}

func TestLoadFromFile_InvalidContent(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "gh.yml")
	// Write something that is valid YAML but not valid gh config
	require.NoError(t, os.WriteFile(configPath, []byte("string_value"), 0644))

	m := NewManager(configPath, "https://example.com/gh.yml")
	err := m.loadFromFile()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid cached config")
}

func TestLoadFromFile_UnreadableFile(t *testing.T) {
	m := NewManager("/tmp/nonexistent-manager-file-99999.yml", "https://example.com/gh.yml")
	err := m.loadFromFile()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read config")
}

func TestManager_ConfigRepos_Empty(t *testing.T) {
	m := NewManager("", "")
	assert.Nil(t, m.ConfigRepos())
}

func TestNormalizeConfigURL_WithSuffix(t *testing.T) {
	assert.Equal(t, "https://cdn.lucc.dev/gh.yml", normalizeConfigURL("https://cdn.lucc.dev/"))
	assert.Equal(t, "https://cdn.lucc.dev/gh.yml", normalizeConfigURL("https://cdn.lucc.dev/gh.yml"))
	assert.Equal(t, "https://custom.url/config.yml", normalizeConfigURL("https://custom.url/config.yml"))
}

func TestNewManager_EmptyURL(t *testing.T) {
	m := NewManager("/tmp/test.yml", "")
	assert.Equal(t, DefaultConfigURL, m.configURL)
}

func TestNewManager_EmptyPath(t *testing.T) {
	m := NewManager("", "https://example.com/gh.yml")
	assert.Equal(t, DefaultConfigPath, m.configPath)
}

func TestLoadWithBackgroundSync_NoCacheFallsBackToCacheTTL(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "gh.yml")
	// No cache file exists - should fall back to LoadWithCacheTTL

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(validRemoteConfigYAML("remote"))
	}))
	t.Cleanup(server.Close)

	m := NewManager(configPath, server.URL)
	require.NoError(t, m.LoadWithBackgroundSync())

	result := m.Filter("remote")
	require.Len(t, result, 1)
}

func TestLoadFromFile_UnmarshalError(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "gh.yml")
	// Valid YAML but not a ConfigRepos structure
	require.NoError(t, os.WriteFile(configPath, []byte(`"just a string"`), 0644))

	m := NewManager(configPath, "https://example.com/gh.yml")
	err := m.loadFromFile()
	// This is valid YAML that passes ValidateConfigYAML but fails unmarshal
	// Actually "just a string" will fail ValidateConfigYAML because it's not a slice
	require.Error(t, err)
}

func TestWriteCache_InvalidPath(t *testing.T) {
	m := NewManager("/nonexistent/dir/path.yml", "https://example.com/gh.yml")
	err := m.writeCache([]byte("data"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write config cache")
}

func TestStartBackgroundSyncProcess_UsesRealExecutable(t *testing.T) {
	// Test the actual startBackgroundSyncProcess function by calling it directly
	// This will use os.Executable() to find the test binary and try to run it
	// RunBackground doesn't wait for completion, so this should return nil
	m := NewManager("/tmp/test-gh.yml", "https://example.com/gh.yml")
	err := startBackgroundSyncProcess(m)
	// It should not error since RunBackground is fire-and-forget
	require.NoError(t, err)
}

func validRemoteConfigYAML(name string) []byte {
	return []byte(`- type: tool
  tag: test
  repo:
    - url: https://github.com/acme/` + name + `
      des: ` + name + ` repository
`)
}
