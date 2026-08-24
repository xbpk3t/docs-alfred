package curate

import (
	"fmt"
	"strings"

	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
)

// SeedFiles groups the raw inputs Step 1 reads. No parsing happens here: the
// seed file's own format (a roster, a table, a paste) is deliberately handed
// to the AI, which extracts Candidate{}. The only accommodations are reading
// files and assembling them into one prompt-ready blob.
type SeedFiles struct {
	// Content is the concatenated, newline-separated content of every --seed
	// file, trimmed.
	Content string
	// Sources lists the original file paths (for provenance in reports).
	Sources []string
}

// LoadSeeds reads one or more seed files. Each is treated as free text; the
// AI does the schema-inference, so we do not try to guess a column layout.
func LoadSeeds(paths []string) (*SeedFiles, error) {
	out := &SeedFiles{Sources: paths}
	var parts []string
	for _, p := range paths {
		data, err := fileutil.ReadSingleFile(p, nil)
		if err != nil {
			return nil, fmt.Errorf("read seed %s: %w", p, err)
		}
		parts = append(parts, string(data))
	}
	out.Content = strings.Join(parts, "\n")
	out.Content = strings.TrimSpace(out.Content)

	return out, nil
}
