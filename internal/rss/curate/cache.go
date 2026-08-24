package curate

import (
	"fmt"
	"os"
	"time"

	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
)

// fetchCache saves and restores the Step 2 outcome ([]freqResult) so a re-run
// with the same cache file skips re-fetching and re-measuring frequency. It
// also pins observed dates, keeping a run deterministic.
type fetchCache struct {
	Written string       `json:"written"`
	Rows    []freqResult `json:"rows"`
}

func loadCache(path string) (*fetchCache, error) {
	if path == "" {
		return &fetchCache{}, nil
	}
	if _, err := os.Stat(path); err != nil {
		return &fetchCache{}, nil //nolint:nilerr // missing cache is a fresh start
	}
	c, err := fileutil.ReadJSONFile[fetchCache](path)
	if err != nil {
		return &fetchCache{}, nil //nolint:nilerr // corrupt cache is a fresh start
	}

	return &c, nil
}

func (c *fetchCache) save(path string) error {
	c.Written = time.Now().UTC().Format(time.RFC3339)
	if err := fileutil.AtomicWriteJSONFile(path, c, fileutil.FilePermPrivate); err != nil {
		return fmt.Errorf("write fetch cache: %w", err)
	}

	return nil
}
