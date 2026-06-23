package enrich

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCache_NewCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	c := NewCache(path)
	require.NotNil(t, c)
	assert.Equal(t, path, c.path)
	assert.Equal(t, defaultCacheFlushInterval, c.interval)
	assert.NotNil(t, c.data)
}

func TestCache_SetAndGet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	c := NewCache(path)

	c.Set("key1", json.RawMessage(`{"value":1}`))
	got := c.Get("key1")
	require.NotNil(t, got)
	assert.Contains(t, string(got), `"value":1`)

	// Non-existent key
	assert.Nil(t, c.Get("nonexistent"))
}

func TestCache_Flush(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	c := NewCache(path)

	c.Set("k1", json.RawMessage(`"v1"`))
	c.Set("k2", json.RawMessage(`"v2"`))
	c.Flush()

	// Load from disk and verify
	c2 := NewCache(path)
	assert.NotNil(t, c2.Get("k1"))
	assert.NotNil(t, c2.Get("k2"))
}

func TestCache_AutoFlush(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	c := NewCache(path)
	c.interval = 3 // flush every 3 writes

	c.Set("a", json.RawMessage(`"1"`))
	c.Set("b", json.RawMessage(`"2"`))
	// Not flushed yet (2 < 3)

	c.Set("c", json.RawMessage(`"3"`))
	// Auto-flushed (3 >= 3)

	c2 := NewCache(path)
	assert.NotNil(t, c2.Get("a"))
	assert.NotNil(t, c2.Get("b"))
	assert.NotNil(t, c2.Get("c"))
}

func TestCache_OverwriteKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	c := NewCache(path)

	c.Set("key", json.RawMessage(`"old"`))
	c.Set("key", json.RawMessage(`"new"`))
	c.Flush()

	c2 := NewCache(path)
	assert.Equal(t, json.RawMessage(`"new"`), c2.Get("key"))
}

func TestCache_LoadFromExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	c := NewCache(path)
	c.Set("existing", json.RawMessage(`"data"`))
	c.Flush()

	// Create new cache from same file
	c2 := NewCache(path)
	assert.NotNil(t, c2.Get("existing"))
}

func TestCache_LoadFromNonExistentFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent", "cache.json")
	c := NewCache(path)
	assert.NotNil(t, c)
	assert.Nil(t, c.Get("any"))
}

func TestCache_LoadFromCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	// Write corrupt data
	require.NoError(t, os.WriteFile(path, []byte("not-json{{{"), 0o644))

	c := NewCache(path)
	assert.NotNil(t, c)
	assert.Nil(t, c.Get("any"))
}

func TestCache_EmptyDataNoFlush(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	c := NewCache(path)
	c.Flush() // Should not write empty cache

	// File should not exist
	assert.NoFileExists(t, path)
}

func TestCache_ConcurrentAccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	c := NewCache(path)

	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			c.Set("key", json.RawMessage(`"value"`))
		}
		close(done)
	}()

	for i := 0; i < 100; i++ {
		_ = c.Get("key")
	}
	<-done
}

func TestCache_FlushToInvalidPath(t *testing.T) {
	// Cache with a path that can't be written to
	path := filepath.Join(t.TempDir(), "nonexistent", "dir", "cache.json")
	c := NewCache(path)
	c.Set("key", json.RawMessage(`"value"`))
	// Flush should not panic even if write fails
	c.Flush()
}

func TestCache_FlushWithData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	c := NewCache(path)
	c.data["key"] = json.RawMessage(`"value"`)
	c.Flush()

	// Verify file was written
	c2 := NewCache(path)
	assert.NotNil(t, c2.Get("key"))
}

func TestCache_FlushEmptyData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	c := NewCache(path)
	// Flush with no data should be a no-op
	c.Flush()
	assert.NoFileExists(t, path, "empty cache should not create file")
}

func TestCache_SetIntervalZero(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	c := NewCache(path)
	c.interval = 0
	// Should not auto-flush even after many writes
	c.Set("k", json.RawMessage(`"v"`))
	assert.NoFileExists(t, path)
}
