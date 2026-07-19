package compact

import (
	"path"
	"sort"
	"strings"
	"time"

	carbon "github.com/dromara/carbon/v2"
	"github.com/samber/lo"
	"github.com/xbpk3t/docs-alfred/pkg/gitutil"
)

// HotTopic is a ranked wiki topic with substantive log.md edits in the window.
type HotTopic struct {
	LastEdit     time.Time
	LogPath      string
	TopicDir     string
	TopicPath    string
	Diffs        []string
	CommitHashes []string
	EditDays     int
	EditCommits  int
	DeltaChars   int
	DeltaLines   int
	Score        int
}

// AggregateHotTopics merges log edits into topics, scores, and returns all
// topics sorted by score desc (then last edit desc).
// EditDays buckets use Asia/Shanghai calendar days (via carbon defaults).
func AggregateHotTopics(edits []gitutil.LogEdit, wikiRoot string) []HotTopic {
	wikiRoot = strings.Trim(strings.ReplaceAll(wikiRoot, "\\", "/"), "/")
	type acc struct {
		last     time.Time
		days     map[string]struct{}
		commits  map[string]struct{}
		logPath  string
		diffs    []string
		hashList []string
		chars    int
		lines    int
	}
	byPath := make(map[string]*acc)
	for _, e := range edits {
		a := byPath[e.Path]
		if a == nil {
			a = &acc{
				logPath: e.Path,
				days:    make(map[string]struct{}),
				commits: make(map[string]struct{}),
			}
			byPath[e.Path] = a
		}
		day := carbon.CreateFromStdTime(e.When).ToDateString()
		a.days[day] = struct{}{}
		if _, ok := a.commits[e.CommitHash]; !ok {
			a.commits[e.CommitHash] = struct{}{}
			a.hashList = append(a.hashList, e.CommitHash)
		}
		a.chars += e.DeltaChars
		a.lines += e.DeltaLines
		if e.When.After(a.last) {
			a.last = e.When
		}
		if e.Diff != "" {
			a.diffs = append(a.diffs, e.Diff)
		}
	}

	out := make([]HotTopic, 0, len(byPath))
	for _, a := range byPath {
		ht := HotTopic{
			LogPath:      a.logPath,
			TopicDir:     gitutil.TopicDirFromLogPath(a.logPath),
			EditDays:     len(a.days),
			EditCommits:  len(a.commits),
			DeltaChars:   a.chars,
			DeltaLines:   a.lines,
			LastEdit:     a.last,
			Diffs:        a.diffs,
			CommitHashes: a.hashList,
		}
		ht.TopicPath = topicPathFromLog(a.logPath, wikiRoot)
		ht.Score = hotScore(ht.EditDays, ht.EditCommits, ht.DeltaChars)
		out = append(out, ht)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].LastEdit.After(out[j].LastEdit)
	})

	return out
}

// TopNHot returns the first n hot topics (or all if fewer / n<=0).
func TopNHot(topics []HotTopic, n int) []HotTopic {
	if n <= 0 || len(topics) <= n {
		return topics
	}
	return lo.Subset(topics, 0, uint(n))
}

func hotScore(editDays, editCommits, deltaChars int) int {
	commits := editCommits
	if commits > 5 {
		commits = 5
	}
	return 4*editDays + 2*commits + deltaCharBucket(deltaChars)
}

func deltaCharBucket(n int) int {
	switch {
	case n >= 2000:
		return 5
	case n >= 500:
		return 4
	case n >= 150:
		return 3
	case n >= 40:
		return 2
	case n > 0:
		return 1
	default:
		return 0
	}
}

func topicPathFromLog(logPath, wikiRoot string) string {
	logPath = strings.ReplaceAll(logPath, "\\", "/")
	dir := path.Dir(logPath)
	if wikiRoot == "" {
		return dir
	}
	prefix := wikiRoot + "/"
	if strings.HasPrefix(dir, prefix) {
		return strings.TrimPrefix(dir, prefix)
	}
	if dir == wikiRoot {
		return ""
	}
	return dir
}

// MergedDiff joins per-edit diffs for AI/mail, capped by maxRunes.
func MergedDiff(ht *HotTopic, maxRunes int) string {
	if maxRunes <= 0 {
		maxRunes = 8000
	}
	return truncateRunes(strings.Join(ht.Diffs, "\n---\n"), maxRunes, "\n... (merged diff truncated)")
}

// truncateRunes caps s to maxRunes and appends suffix when truncated (head keep).
func truncateRunes(s string, maxRunes int, suffix string) string {
	if maxRunes <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + suffix
}
