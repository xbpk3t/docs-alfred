package transcript

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
)

// ErrCacheMiss is returned when a cache entry is not found.
var ErrCacheMiss = errors.New("cache miss")

// CacheEntry represents one cached transcript entry.
type CacheEntry struct {
	FetchedAt      time.Time `json:"fetchedAt"`
	ExpiresAt      time.Time `json:"expiresAt"`
	EpisodeTitle   string    `json:"episodeTitle"`
	EpisodeURL     string    `json:"episodeUrl,omitempty"`
	EpisodeGUID    string    `json:"episodeGuid,omitempty"`
	FeedTitle      string    `json:"feedTitle,omitempty"`
	FeedURL        string    `json:"feedUrl,omitempty"`
	Source         string    `json:"source"`
	ContentType    string    `json:"contentType"`
	TranscriptPath string    `json:"transcriptPath,omitempty"`
	TranscriptURL  string    `json:"transcriptUrl,omitempty"`
}

// Cache provides a key-based filesystem cache for transcripts.
type Cache struct {
	baseDir string
}

// NewCache creates a new transcript cache with the given base directory.
func NewCache(baseDir string) *Cache {
	return &Cache{baseDir: baseDir}
}

// Key generates a deterministic cache key from feed URL and episode identity.
func (c *Cache) Key(feedURL, guid, link, title string) string {
	idSource := guid
	if idSource == "" {
		idSource = link
	}
	if idSource == "" {
		idSource = title
	}

	h := sha256.New()
	h.Write([]byte(feedURL))
	h.Write([]byte(idSource))

	return hex.EncodeToString(h.Sum(nil))
}

// CacheFilePath returns the filesystem path for a transcript cache file.
func (c *Cache) CacheFilePath(key string) string {
	return filepath.Join(c.baseDir, key, "transcript.txt")
}

// MetaFilePath returns the filesystem path for a metadata file.
// TS convention: {key}/metadata.json.
func (c *Cache) MetaFilePath(key string) string {
	return filepath.Join(c.baseDir, key, "metadata.json")
}

// IndexFilePath returns the index file path.
func (c *Cache) IndexFilePath() string {
	return filepath.Join(c.baseDir, "index.json")
}

// Get retrieves a cached transcript entry.
// Returns nil if the cache doesn't exist or is empty.
func (c *Cache) Get(key string) (*CacheEntry, error) {
	metaPath := c.MetaFilePath(key)
	data, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrCacheMiss
		}

		return nil, err
	}

	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, err
	}

	// Check transcript file exists
	if entry.TranscriptPath != "" {
		if _, err := os.Stat(entry.TranscriptPath); os.IsNotExist(err) {
			return nil, ErrCacheMiss
		}
	}

	return &entry, nil
}

// Set stores a transcript in the cache.
func (c *Cache) Set(key string, entry *CacheEntry, content string) error {
	cacheDir := filepath.Join(c.baseDir, key)
	// Ensure cache subdirectory exists
	if err := os.MkdirAll(cacheDir, fileutil.DirPerm); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}

	// Write transcript content
	txtPath := c.CacheFilePath(key)
	if content != "" {
		if err := os.WriteFile(txtPath, []byte(content), fileutil.FilePermPrivate); err != nil {
			return fmt.Errorf("write transcript: %w", err)
		}
		entry.TranscriptPath = txtPath
	}

	// Write metadata
	entry.FetchedAt = time.Now()
	metaPath := c.MetaFilePath(key)
	metaData, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	if err := os.WriteFile(metaPath, metaData, fileutil.FilePermPrivate); err != nil {
		return fmt.Errorf("write metadata: %w", err)
	}

	return nil
}

// ReadTranscript reads the cached transcript content.
func (c *Cache) ReadTranscript(key string) (string, error) {
	txtPath := c.CacheFilePath(key)
	data, err := os.ReadFile(txtPath)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(data)), nil
}
