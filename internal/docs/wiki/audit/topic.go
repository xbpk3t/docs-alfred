package audit

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

// TopicMetric is one topic's content census in the wiki tree — the programmatic
// data source for compact's 清零 ranking (a full-state snapshot; no time window,
// because the weekly run resets and treats current state as the batch).
//
// Share the same walk semantics as RunStats (exclusions + artifact handling +
// frontmatter type) so compact's ranking agrees with the human `wiki stats`.
type TopicMetric struct {
	Path     string `json:"path"`
	Files    int    `json:"files"`
	Size     int64  `json:"size"` // bytes of counted md content
	Research int    `json:"research"`
}

// recordTopicFile records one non-directory walk entry into byTopic if it is
// md content (non-excluded, non-artifact, parseable type). Artifact (dir or
// explicit tag) is excluded entirely — same semantics as RunStats, so the
// ranking agrees with `wiki stats`. Degraded files are skipped.
func recordTopicFile(root, path string, d fs.DirEntry, byTopic map[string]*TopicMetric, ex *StatsOptions) error {
	if !strings.HasSuffix(d.Name(), ".md") || ex.excluded(d.Name()) {
		return nil
	}
	rel := slashRel(root, path)
	if checkutil.HasSegmentDir(rel, types.ArtifactDir) {
		return nil
	}
	typ, ok, err := fileType(path)
	if err != nil || typ == transcriptArtifactType {
		return nil //nolint:nilerr // degraded / tagged artifact: not content
	}
	info, err := d.Info()
	if err != nil {
		return fmt.Errorf("topic stat %s: %w", path, err)
	}
	td := topicDir(rel)
	m := byTopic[td]
	if m == nil {
		m = &TopicMetric{Path: td}
		byTopic[td] = m
	}
	m.Files++
	m.Size += info.Size()
	if ok && typ == string(types.TypeDeepDive) {
		m.Research++
	}
	return nil
}

// TopicMetrics scans the wiki tree and returns per-topic content metrics,
// sorted by research desc, then files desc, then path asc (deterministic).
// md content = non-excluded, non-artifact files with a parseable type is counted
// into its topic dir; research counts type:research files.
func TopicMetrics(root string) ([]TopicMetric, error) {
	if root == "" {
		return nil, fmt.Errorf("topic metrics: wiki root required")
	}

	ex := &StatsOptions{}
	byTopic := map[string]*TopicMetric{}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("topic walk %s: %w", path, walkErr)
		}
		if d.IsDir() {
			return nil
		}
		return recordTopicFile(root, path, d, byTopic, ex)
	})
	if err != nil {
		return nil, err
	}

	out := make([]TopicMetric, 0, len(byTopic))
	for _, m := range byTopic {
		out = append(out, *m)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := &out[i], &out[j]
		if a.Research != b.Research {
			return a.Research > b.Research
		}
		if a.Files != b.Files {
			return a.Files > b.Files
		}
		return a.Path < b.Path
	})
	return out, nil
}
