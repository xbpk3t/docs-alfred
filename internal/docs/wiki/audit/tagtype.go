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

// defaultTagTypeLimit is the default number of "类型<i>" columns per tag.
const defaultTagTypeLimit = 7

// researchTagTypeName is the section heading for the per-tag matrix, both the
// Markdown/terminal heading and the JSON "section" key.
const researchTagTypeName = "research tag·type"

// RunTagTypeStats is a self-contained per-tag research breakdown rendered as a
// SINGLE table: one row per top-level tag, columns "tag" + "类型<i>", each cell
// that tag's i-th top research type as "name (count)".
//
// It is intentionally isolated from RunStats: it performs its own single walk
// over the tree and returns its own section, so it can be stripped with no
// change to RunStats if the feature is no longer wanted. Shared section
// plumbing (newSection / rowKV) and frontmatter reading (fileType) are reused.
func RunTagTypeStats(root string, typeLimit int) ([]Section, error) {
	if root == "" {
		return nil, fmt.Errorf("tagtype stats: wiki root required")
	}
	if typeLimit <= 0 {
		typeLimit = defaultTagTypeLimit
	}

	// counts: tag → type → research count. Every <tag>/<type> dir is recorded
	// (count 0) when the walk meets it, so 0-research types still get columns;
	// a research md file then increments its own bucket. One map, no drift.
	counts := map[string]map[string]int{}
	ex := &StatsOptions{}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("tag type walk %s: %w", path, walkErr)
		}
		rel := slashRel(root, path)
		if d.IsDir() {
			recordDirType(counts, rel)
			return nil
		}
		return recordType(counts, ex, path, rel, d.Name())
	})
	if err != nil {
		return nil, err
	}

	return []Section{buildTagSection(counts, typeLimit)}, nil
}

// recordDirType seeds a (tag, type) bucket with 0 so the type still appears as
// a column even when it holds no research files. Seeding is first-visit-wins:
// a nested dir (tag/type/topic/...) maps to the same bucket as its parent type
// dir and must not re-zero research counts already accumulated from files
// walked earlier.
func recordDirType(counts map[string]map[string]int, rel string) {
	tag, typ, ok := tagTypeSegs(rel)
	if !ok {
		return
	}
	if counts[tag] == nil {
		counts[tag] = map[string]int{}
	}
	if _, seen := counts[tag][typ]; seen {
		return
	}
	counts[tag][typ] = 0
}

// recordType counts a single md file into the right tag/type bucket if it is
// a research entry. Mirrors RunStats eligibility (exclusions + artifact dirs)
// so this section agrees with the sibling research/type sections.
func recordType(counts map[string]map[string]int, ex *StatsOptions, path, rel, name string) error {
	if !strings.HasSuffix(name, ".md") || ex.excluded(name) {
		return nil
	}
	typ, ok, err := fileType(path)
	if err != nil {
		return nil //nolint:nilerr // unreadable file is not research; keep walking
	}
	if typ == transcriptArtifactType || checkutil.HasSegmentDir(rel, types.ArtifactDir) {
		return nil
	}
	if !ok || typ != string(types.TypeDeepDive) {
		return nil
	}
	tag, typName, ok := tagTypeSegs(rel)
	if !ok {
		return nil
	}
	if counts[tag] == nil {
		counts[tag] = map[string]int{}
	}
	counts[tag][typName]++
	return nil
}

// tagTypeSegs splits a slash-rel path into its first two segments — the
// top-level tag and the second-level type — requiring both present and not
// dot-hidden (so .blockchain / .java-style dirs are never columns).
func tagTypeSegs(rel string) (tag, typ string, ok bool) {
	s := strings.Split(rel, "/")
	if len(s) < 2 || s[0] == "" || s[1] == "" {
		return "", "", false
	}
	if strings.HasPrefix(s[0], ".") || strings.HasPrefix(s[1], ".") {
		return "", "", false
	}
	return s[0], s[1], true
}

// typeColName returns the i-th (0-based) column header, "类型<i+1>".
func typeColName(i int) string {
	return fmt.Sprintf("类型%d", i+1)
}

// buildTagSection renders the single table: one row per tag, with a "tag"
// column plus "类型<i>" columns (i = 1..typeLimit). Each cell holds that tag's
// i-th top research type as "name (count)"; a tag with fewer than typeLimit
// types leaves the remaining cells empty. Rows are ordered by the tag's total
// research count (desc), ties resolved alphabetically, so the busiest tags
// lead. Types beyond typeLimit are dropped — displayed cost, not the full list.
func buildTagSection(counts map[string]map[string]int, typeLimit int) Section {
	type cell struct {
		name  string
		count int
	}
	type tagRow struct {
		name  string
		cells []cell
		total int
	}

	var tags []tagRow
	for tag, typCounts := range counts {
		ordered := sortedTypeNames(typCounts)
		if len(ordered) > typeLimit {
			ordered = ordered[:typeLimit]
		}
		tr := tagRow{name: tag}
		for _, name := range ordered {
			tr.total += typCounts[name]
			tr.cells = append(tr.cells, cell{name: name, count: typCounts[name]})
		}
		tags = append(tags, tr)
	}

	// Busiest tags first; ties broken alphabetically for determinism.
	sort.Slice(tags, func(i, j int) bool {
		if tags[i].total != tags[j].total {
			return tags[i].total > tags[j].total
		}
		return tags[i].name < tags[j].name
	})

	// Every row is padded to the full "类型1..类型<N>" width so the JSON
	// contract is uniform (all rows emit the same key set); DataTable would
	// otherwise union sparse keys per row. Missing ranks render as "".
	data := make([]StatRow, 0, len(tags))
	for _, tr := range tags {
		var pairs []string
		pairs = append(pairs, "tag", tr.name)
		for col := 0; col < typeLimit; col++ {
			v := ""
			if col < len(tr.cells) {
				c := tr.cells[col]
				v = fmt.Sprintf("%s (%d)", c.name, c.count)
			}
			pairs = append(pairs, typeColName(col), v)
		}
		data = append(data, rowKV(pairs...))
	}
	return newSection(researchTagTypeName, data...)
}

// sortedTypeNames sorts type names by research count descending, then
// alphabetically, so the largest bucket leads and ties are deterministic.
func sortedTypeNames(counts map[string]int) []string {
	names := make([]string, 0, len(counts))
	for n := range counts {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool {
		if counts[names[i]] != counts[names[j]] {
			return counts[names[i]] > counts[names[j]]
		}
		return names[i] < names[j]
	})
	return names
}
