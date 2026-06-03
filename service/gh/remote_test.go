package gh

import (
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

func TestLoad_NonExistentFile(t *testing.T) {
	m := NewManager("/tmp/nonexistent-config-99999.yml", "https://nonexistent.example.com/config.yml")
	err := m.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config file not found and sync failed")
}

func TestLoad_CacheMiss(t *testing.T) {
	m := NewManager("/tmp/nonexistent-cache-88888.yml", "https://nonexistent.example.com/config.yml")
	err := m.LoadWithCacheTTL()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config not cached and sync failed")
}

func TestGetRepos_Empty(t *testing.T) {
	m := NewManager("", "")
	repos := m.GetRepos()
	assert.Nil(t, repos)
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

	m := NewManager(configPath, "")
	err := m.Load()
	require.Error(t, err)
}
