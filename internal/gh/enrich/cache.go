package enrich

import (
	"encoding/json"
	"log/slog"
	"os"
	"sync"

	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
)

const (
	defaultCacheFlushInterval = 20 // flush every N writes
	cacheDir                  = "/tmp"
)

// Cache is a concurrency-safe key-value store backed by a JSON file.
// It is used to avoid redundant API calls across multiple runs.
type Cache struct {
	data     map[string]json.RawMessage
	path     string
	dirty    int
	interval int
	mu       sync.RWMutex
}

// NewCache creates or loads a cache file at the given path.
func NewCache(path string) *Cache {
	c := &Cache{
		data:     make(map[string]json.RawMessage),
		path:     path,
		interval: defaultCacheFlushInterval,
	}
	c.load()

	return c
}

// Get returns the cached response for a key, or nil if not found.
func (c *Cache) Get(key string) json.RawMessage {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.data[key]
}

// Set stores a response for the given key and flushes periodically.
func (c *Cache) Set(key string, value json.RawMessage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = value
	c.dirty++
	if c.interval > 0 && c.dirty >= c.interval {
		c.flushLocked()
	}
}

// Flush writes the cache to disk immediately.
func (c *Cache) Flush() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.flushLocked()
}

func (c *Cache) load() {
	data, err := os.ReadFile(c.path)
	if err != nil {
		return // cache miss or first run; start empty
	}
	if err := json.Unmarshal(data, &c.data); err != nil {
		slog.Warn("corrupt cache file, starting fresh", "path", c.path, "error", err)
		c.data = make(map[string]json.RawMessage)
	}
}

func (c *Cache) flushLocked() {
	if len(c.data) == 0 {
		return
	}
	data, err := json.Marshal(c.data)
	if err != nil {
		slog.Warn("cache marshal failed", "error", err)

		return
	}
	if err := fileutil.AtomicWriteFile(c.path, data, fileutil.FilePermPrivate); err != nil {
		slog.Warn("cache write failed", "path", c.path, "error", err)

		return
	}
	c.dirty = 0
	slog.Debug("cache flushed", "path", c.path, "entries", len(c.data))
}
