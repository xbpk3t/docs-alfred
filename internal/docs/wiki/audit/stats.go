package audit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/adrg/frontmatter"
	yaml "github.com/goccy/go-yaml"
	wikitypes "github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
	"github.com/xbpk3t/docs-alfred/pkg/md"
)

// --- Wiki stats: full-tree scan → ordered record sections ---

// StatRow is one record in a stats section. Keys are dynamic and ordered
// (first-seen defines the Markdown column order, preservable in JSON).
type StatRow = yaml.MapSlice

// Section is one named record group in the stats output.
type Section struct {
	Section string    `json:"section"`
	Data    []StatRow `json:"data"`
}

// Stats is the ordered section list. JSON output is exactly this slice;
// Markdown is a derived view (one table per section).
type Stats []Section

// StatsOptions controls a stats scan.
type StatsOptions struct {
	WikiRoot     string
	ExcludeNames []string
	TopN         int
}

// DefaultExcludeNames are operation artifacts that are not wiki content:
// digest pipelines logs and root scratch files. Applied when the caller does
// not override ExcludeNames.
var DefaultExcludeNames = []string{
	"digest-success.jsonl", "digest-ai-error.jsonl",
	"digest-classify-rejected.jsonl", "digest-extract-error.jsonl",
	"digest-fetch-error.jsonl",
	"temp.md", "inbox.md", "uncat.md", "success.md",
	// summary.md is a per-topic digest page (type=digest) and IS part of the
	// content census; archive-summary.md stays excluded as an aggregate rollup.
	"archive-summary.md",
}

// transcriptArtifactType is the frontmatter type-tag the transcript pipeline
// writes (equal to the artifact directory name). Transcript files are pipeline
// artifacts, not OKF wiki content: never a valid entry type, censused
// separately.
const transcriptArtifactType = wikitypes.ArtifactDir

// excludedWithDefault reports whether the file's base name is excluded,
// using DefaultExcludeNames when the caller provided none.
func (o *StatsOptions) excluded(name string) bool {
	list := o.ExcludeNames
	if len(list) == 0 {
		list = DefaultExcludeNames
	}
	for _, x := range list {
		if name == x {
			return true
		}
	}
	return false
}

// RunStats scans the wiki tree and returns ordered sections:
//
//   - overview    : files / md / size (one row per metric)
//   - research topN: topics with the most type=research md files, desc
//
// Operation artifacts (jsonl logs, temp files) are excluded from the tree.
func RunStats(opts StatsOptions) (Stats, error) {
	if opts.WikiRoot == "" {
		return nil, fmt.Errorf("stats: wiki root required")
	}
	topN := opts.TopN
	if topN <= 0 {
		topN = 10 // default: research top 10
	}

	var (
		files  int
		mdN    int
		bytesN int64
		// researchByTopic: topic path (dir) → count of type=research md files.
		researchByTopic = make(map[string]int)
		researchTotal   int
		// typeCounts: every frontmatter type seen (enum + unknown), so no
		// file silently disappears from the stats (P0 blind spot).
		typeCounts = make(map[string]int)
		artifactN  int // md files that are pipeline artifacts (transcript), not content
		noFM       int // md files without parseable type field
		unreadable int // md files that failed to read/parse (degraded, not aborting)
	)

	err := filepath.WalkDir(opts.WikiRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk %s: %w", path, walkErr)
		}
		if d.IsDir() {
			return nil
		}
		if opts.excluded(d.Name()) {
			return nil
		}

		files++
		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("stat %s: %w", path, err)
		}
		bytesN += info.Size()

		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		mdN++
		typ, ok, err := fileType(path)
		if err != nil {
			// A single bad file must not abort the whole scan (counts would be
			// silently wrong). Degrade to the unreadable bucket instead.
			unreadable++
			return nil //nolint:nilerr // degraded: count it, keep walking
		}
		// Pipeline/artifact md (transcript productions) are not OKF content: a
		// file sits under an artifact dir (e.g. transcript/) or is tagged type:
		// transcript. `typ` is only non-empty when frontmatter parsed, so an
		// untagged file is captured by the dir test below. Keep these out of
		// the type census; report them as their own artifact bucket.
		rel := slashRel(opts.WikiRoot, path)
		if typ == transcriptArtifactType || checkutil.HasSegmentDir(rel, wikitypes.ArtifactDir) {
			artifactN++
			return nil
		}
		if !ok {
			noFM++
			return nil
		}
		typeCounts[typ]++
		if typ == string(wikitypes.TypeDeepDive) {
			researchByTopic[topicDir(rel)]++
			researchTotal++
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	stats := Stats{
		overviewSection(files, mdN, bytesN),
		typeSection(typeCounts, artifactN, noFM, unreadable),
		researchSection(researchByTopic, researchTotal, topN),
	}
	return stats, nil
}

// fileType reads the frontmatter type field of a markdown file.
// ok=false when the file has no parseable frontmatter or no type.
func fileType(path string) (typ string, ok bool, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false, fmt.Errorf("read %s: %w", path, err)
	}
	var fm struct {
		Type string `yaml:"type"`
	}
	body, err := frontmatter.Parse(strings.NewReader(string(data)), &fm)
	if err != nil {
		return "", false, nil //nolint:nilerr // unparseable frontmatter: not a fatal error, file is not research
	}
	if len(body) == len(data) || strings.TrimSpace(fm.Type) == "" {
		return "", false, nil // missing frontmatter or type
	}
	return fm.Type, true, nil
}

// topicDir returns the frontmatter topic path: rel's directory, or "" for
// files at the root.
func topicDir(rel string) string {
	dir := filepath.Dir(rel)
	if dir == "." {
		return ""
	}
	return dir
}

// newSection builds a Section in one call.
func newSection(name string, rows ...StatRow) Section {
	return Section{Section: name, Data: rows}
}

func overviewSection(files, mdN int, bytesN int64) Section {
	return newSection("overview",
		rowKV("metric", "files", "value", fmt.Sprintf("%d", files)),
		rowKV("metric", "md", "value", fmt.Sprintf("%d", mdN)),
		rowKV("metric", "size", "value", humanBytes(bytesN)),
	)
}

// typeSection reports every frontmatter type count (enum values + any
// unknown types) plus md files without a type field. Nothing is dropped.
func typeSection(typeCounts map[string]int, artifactN, noFM, unreadable int) Section {
	rows := []StatRow{}
	if artifactN > 0 {
		rows = append(rows, rowKV("type", "artifact", "count", fmt.Sprintf("%d", artifactN)))
	}
	for _, t := range wikitypes.KnownTypes {
		if n := typeCounts[string(t)]; n > 0 {
			rows = append(rows, rowKV("type", string(t), "count", fmt.Sprintf("%d", n)))
		}
	}
	// Unknown types (outside the enum) are still counted — no silent loss.
	known := make(map[string]bool, len(wikitypes.KnownTypes))
	for _, k := range wikitypes.KnownTypes {
		known[string(k)] = true
	}
	var unknowns []string
	for t := range typeCounts {
		if !known[t] {
			unknowns = append(unknowns, t)
		}
	}
	sort.Strings(unknowns)
	for _, t := range unknowns {
		rows = append(rows, rowKV("type", t, "count", fmt.Sprintf("%d", typeCounts[t]), "unknown", "true"))
	}
	if noFM > 0 {
		rows = append(rows, rowKV("type", "none", "count", fmt.Sprintf("%d", noFM)))
	}
	if unreadable > 0 {
		rows = append(rows, rowKV("type", "unreadable", "count", fmt.Sprintf("%d", unreadable)))
	}
	return newSection("types", rows...)
}

func researchSection(byTopic map[string]int, total, topN int) Section {
	topics := make([]string, 0, len(byTopic))
	for t := range byTopic {
		topics = append(topics, t)
	}
	sort.Slice(topics, func(i, j int) bool {
		if byTopic[topics[i]] != byTopic[topics[j]] {
			return byTopic[topics[i]] > byTopic[topics[j]]
		}
		return topics[i] < topics[j] // deterministic tie-break
	})
	if topN > 0 && len(topics) > topN {
		topics = topics[:topN]
	}
	rows := make([]StatRow, 0, len(topics))
	for _, t := range topics {
		rows = append(rows, rowKV("topic", t, "count", fmt.Sprintf("%d", byTopic[t])))
	}
	return newSection(fmt.Sprintf("research top%d", topN), rows...)
}

// rowKV builds a record from alternating key/value pairs, preserving order.
func rowKV(pairs ...string) StatRow {
	var out StatRow
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, yaml.MapItem{Key: pairs[i], Value: pairs[i+1]})
	}
	return out
}

// humanBytes formats a byte count as a human-readable string.
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

// --- Output views ---

// Terminal renders Stats as terminal box tables: "## <section>" per section.
func (s Stats) Terminal() string {
	return s.sections(false)
}

// Markdown renders Stats as Markdown source tables (go-pretty RenderMarkdown).
func (s Stats) Markdown() string {
	return s.sections(true)
}

func (s Stats) sections(markdownSource bool) string {
	var sb strings.Builder
	for _, sec := range s {
		if len(sec.Data) == 0 {
			continue
		}
		sb.WriteString(md.NamedDataTable(sec.Section, sec.Data, markdownSource))
		sb.WriteString("\n")
	}
	return sb.String()
}

// JSON emits the section list verbatim (ordered keys) — the raw contract.
func (s Stats) JSON() ([]byte, error) {
	// yaml.MapSlice has no MarshalJSON (would emit {Key,Value} structs), so
	// each section's rows go through the order-preserving emitter.
	var buf bytes.Buffer
	buf.WriteByte('[')
	for i, sec := range s {
		if i > 0 {
			buf.WriteByte(',')
		}
		nameJSON, err := json.Marshal(sec.Section)
		if err != nil {
			return nil, err
		}
		rowsJSON, err := md.NewDataTable(sec.Data).JSON()
		if err != nil {
			return nil, err
		}
		buf.WriteString(`{"section":`)
		buf.Write(nameJSON)
		buf.WriteString(`,"data":`)
		buf.Write(rowsJSON)
		buf.WriteByte('}')
	}
	buf.WriteByte(']')
	return buf.Bytes(), nil
}
