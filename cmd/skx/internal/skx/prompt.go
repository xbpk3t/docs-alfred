// Package skx routes, checks, and graphs zzz prompt markdown files.
package skx

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	yaml "github.com/goccy/go-yaml"
)

// BodyCharWarn is the soft budget for a prompt body (frontmatter excluded).
const BodyCharWarn = 3000

// Prompt is a single parsed zzz prompt markdown file.
type Prompt struct {
	Path string
	// Name is the routing stem (brk.md -> "brk").
	Name     string
	Role     string
	Desc     string
	Pipeline []string
	// UnknownKeys lists frontmatter keys outside role/desc/pipeline.
	UnknownKeys []string
	// FrontmatterLines is the number of lines before the body (both
	// delimiters included), so body line N is file line N+FrontmatterLines.
	FrontmatterLines int
	Body             string
}

// MarkdownInfo is the structural summary used by skx check.
type MarkdownInfo struct {
	Chars       int
	H1          []Heading
	DuplicateH2 []string
	EmptyH2     []string
	FenceIssues []string
}

// Heading is one markdown heading outside a fenced code block. Line is
// file-relative when produced through analyzeMarkdownFrom with an offset.
type Heading struct {
	Line int
	Text string
}

// frontmatter is the machine-readable envelope of a prompt file. Unknown keys
// land in Extra so check can report them without rejecting the file at load.
type frontmatter struct {
	Role     string         `yaml:"role"`
	Desc     string         `yaml:"desc"`
	Pipeline []string       `yaml:"pipeline"`
	Extra    map[string]any `yaml:",inline"`
}

// LoadPrompt parses a zzz prompt markdown file: YAML frontmatter + body.
func LoadPrompt(path string) (*Prompt, error) {
	p, err := loadPrompt(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return p, nil
}

// loadPrompt is LoadPrompt without the path prefix. Callers that carry the
// path separately (indexDir -> Issue.Path) use it so CLI output does not
// render the path twice.
func loadPrompt(path string) (*Prompt, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	fmBytes, body, fmLines, err := splitFrontmatter(data)
	if err != nil {
		return nil, err
	}
	var fm frontmatter
	if err := yaml.Unmarshal(fmBytes, &fm); err != nil {
		return nil, fmt.Errorf("parse frontmatter: %w", err)
	}
	p := &Prompt{
		Path:             path,
		Name:             PromptStem(path),
		Role:             strings.TrimSpace(fm.Role),
		Desc:             strings.TrimSpace(fm.Desc),
		Pipeline:         fm.Pipeline,
		Body:             body,
		FrontmatterLines: fmLines,
	}
	for k := range fm.Extra {
		switch k {
		case "role", "desc", "pipeline":
		default:
			p.UnknownKeys = append(p.UnknownKeys, k)
		}
	}
	sort.Strings(p.UnknownKeys)
	return p, nil
}

func splitFrontmatter(data []byte) (fm []byte, body string, frontmatterLines int, err error) {
	norm := strings.ReplaceAll(string(data), "\r\n", "\n")
	norm = strings.TrimPrefix(norm, "\uFEFF") // UTF-8 BOM from user editors
	lines := strings.Split(norm, "\n")
	// Frontmatter delimiters must start at column 0; trailing whitespace is
	// tolerated. An indented "---" is content (e.g. inside a block scalar),
	// not a delimiter.
	if strings.TrimRight(lines[0], " \t") != "---" {
		return nil, "", 0, fmt.Errorf("missing frontmatter: first line must be ---")
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], " \t") == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return nil, "", 0, fmt.Errorf("unclosed frontmatter")
	}
	fm = []byte(strings.Join(lines[1:end], "\n"))
	body = strings.Join(lines[end+1:], "\n")
	return fm, body, end + 1, nil
}

// PromptStem returns the routing name for a prompt file: its base file name
// without extension. Hidden files (dot-prefixed) return "".
func PromptStem(path string) string {
	stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if strings.HasPrefix(stem, ".") {
		return ""
	}
	return stem
}

// CollectMarkdown returns every *.md file under dir (recursively, sorted).
func CollectMarkdown(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", dir, err)
	}
	sort.Strings(files)
	return files, nil
}

// skipHidden reports whether a relative path contains a dot-segment
// (e.g. repo/.TableCate.md): hidden files are cross-ref targets, not
// routing entries.
func skipHidden(rel string) bool {
	for _, seg := range strings.Split(filepath.ToSlash(rel), "/") {
		if strings.HasPrefix(seg, ".") && seg != "." && seg != ".." {
			return true
		}
	}
	return false
}

// promptEntry is one markdown file under a prompt directory, parsed once.
type promptEntry struct {
	path   string
	prompt *Prompt
	err    error
}

// dirIndex is the single-walk, single-parse view of a prompt directory.
type dirIndex struct {
	entries    []promptEntry
	routable   map[string]*Prompt // non-hidden, parseable, keyed by routing name
	skipped    []string           // non-hidden files that failed to parse
	duplicates []duplicateStem
}

// duplicateStem records a parseable, non-hidden prompt whose routing name is
// already taken by an earlier (sorted) file. Routing keeps the winner; the
// shadowed file is still checked but is not addressable or part of the graph.
type duplicateStem struct {
	name     string
	winner   string
	shadowed string
}

// indexDir walks dir and parses every *.md file exactly once. Hidden files are
// parsed too, because check validates them, but they are never routable. When
// several non-hidden files share a stem, the first sorted path wins and the
// rest are recorded in duplicates.
func indexDir(dir string) (*dirIndex, error) {
	files, err := CollectMarkdown(dir)
	if err != nil {
		return nil, err
	}
	idx := &dirIndex{routable: map[string]*Prompt{}}
	for _, f := range files {
		rel, relErr := filepath.Rel(dir, f)
		hidden := relErr != nil || skipHidden(rel)
		p, loadErr := loadPrompt(f)
		idx.entries = append(idx.entries, promptEntry{
			path:   f,
			prompt: p,
			err:    loadErr,
		})
		if hidden {
			continue
		}
		if loadErr != nil {
			idx.skipped = append(idx.skipped, f)
			continue
		}
		// First sorted path wins, matching route. Later files with the same
		// stem are reported by CheckDir instead of silently disappearing.
		if prev, exists := idx.routable[p.Name]; exists {
			idx.duplicates = append(idx.duplicates, duplicateStem{
				name:     p.Name,
				winner:   prev.Path,
				shadowed: p.Path,
			})
			continue
		}
		idx.routable[p.Name] = p
	}
	return idx, nil
}

// ResolvePrompt finds the prompt whose file stem equals name under dir.
// Hidden files are skipped.
func ResolvePrompt(dir, name string) (rel, mdAbs string, ok bool, err error) {
	idx, err := indexDir(dir)
	if err != nil {
		return "", "", false, err
	}
	p, found := idx.routable[name]
	if !found {
		return "", "", false, nil
	}
	relPath, relErr := filepath.Rel(dir, p.Path)
	if relErr != nil {
		return "", "", false, relErr
	}
	return strings.TrimSuffix(relPath, filepath.Ext(relPath)), p.Path, true, nil
}

// AvailableNames returns all routable prompt names under dir (sorted).
func AvailableNames(dir string) ([]string, error) {
	idx, err := indexDir(dir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(idx.routable))
	for n := range idx.routable {
		names = append(names, n)
	}
	sort.Strings(names)
	return names, nil
}

type rawHeading struct {
	Level int
	Line  int
	Text  string
}

// leadingIndentWidth returns the visual width of the leading spaces and tabs
// in line, using 4-column tab stops as CommonMark does.
func leadingIndentWidth(line string) int {
	col := 0
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case ' ':
			col++
		case '\t':
			col += 4 - col%4
		default:
			return col
		}
	}
	return col
}

func headingAt(line string, lineNo int) (rawHeading, bool) {
	// ATX headings may be indented at most 3 columns; 4+ starts an indented
	// code block, where a "#" line is content, not a heading.
	if leadingIndentWidth(line) >= 4 {
		return rawHeading{}, false
	}
	t := strings.TrimLeft(line, " \t")
	n := 0
	for n < len(t) && t[n] == '#' {
		n++
	}
	if n == 0 || n > 6 {
		return rawHeading{}, false
	}
	if n >= len(t) || t[n] != ' ' {
		return rawHeading{}, false
	}
	return rawHeading{Level: n, Line: lineNo, Text: strings.TrimSpace(t[n:])}, true
}

// fenceLine trims up to 3 columns of indentation from a line that may open or
// close a fenced code block. A line indented 4+ columns is an indented code
// block, not a fence.
func fenceLine(line string) (string, bool) {
	if leadingIndentWidth(line) >= 4 {
		return "", false
	}
	return strings.TrimLeft(line, " \t"), true
}

func openingFence(t string) (byte, int, bool) {
	if len(t) < 3 {
		return 0, 0, false
	}
	c := t[0]
	if c != '`' && c != '~' {
		return 0, 0, false
	}
	n := 0
	for n < len(t) && t[n] == c {
		n++
	}
	if n < 3 {
		return 0, 0, false
	}
	return c, n, true
}

func isClosingFence(t string, c byte, n int) bool {
	m := 0
	for m < len(t) && t[m] == c {
		m++
	}
	if m < n {
		return false
	}
	return strings.TrimSpace(t[m:]) == ""
}

func fenceName(c byte) string {
	if c == '~' {
		return "~~~"
	}
	return "```"
}

// analyzeMarkdown reports H1 headings outside fences, empty or duplicate H2
// sections, unbalanced fences, and body size. Reported line numbers are
// body-relative; use analyzeMarkdownFrom to translate them to file lines.
func analyzeMarkdown(body string) MarkdownInfo {
	return analyzeMarkdownFrom(body, 0)
}

// analyzeMarkdownFrom is analyzeMarkdown with lineOffset added to every
// reported line number, so callers that pass the number of frontmatter lines
// get file-relative diagnostics.
func analyzeMarkdownFrom(body string, lineOffset int) MarkdownInfo {
	info := MarkdownInfo{Chars: utf8.RuneCountInString(body)}
	lines := strings.Split(body, "\n")
	var fenceChar byte
	fenceLen, fenceStart := 0, 0
	var curH2 string
	h2Set, h2Content := false, false
	h2Counts := map[string]int{}
	flushH2 := func() {
		if h2Set && !h2Content {
			info.EmptyH2 = append(info.EmptyH2, curH2)
		}
	}
	for i, line := range lines {
		if fenceLen > 0 {
			if t, ok := fenceLine(line); ok && isClosingFence(t, fenceChar, fenceLen) {
				fenceLen = 0
			}
			continue
		}
		if t, ok := fenceLine(line); ok {
			if c, n, isFence := openingFence(t); isFence {
				fenceChar, fenceLen, fenceStart = c, n, i+1+lineOffset
				if h2Set {
					h2Content = true
				}
				continue
			}
		}
		if h, ok := headingAt(line, i+1+lineOffset); ok {
			switch h.Level {
			case 1:
				info.H1 = append(info.H1, Heading{Line: h.Line, Text: h.Text})
			case 2:
				flushH2()
				h2Counts[h.Text]++
				if h2Counts[h.Text] == 2 {
					info.DuplicateH2 = append(info.DuplicateH2, h.Text)
				}
				curH2, h2Set, h2Content = h.Text, true, false
			}
			continue
		}
		if strings.TrimSpace(line) != "" && h2Set {
			h2Content = true
		}
	}
	flushH2()
	if fenceLen > 0 {
		info.FenceIssues = append(info.FenceIssues, fmt.Sprintf("unclosed %s fence opened at line %d", fenceName(fenceChar), fenceStart))
	}
	sort.Strings(info.DuplicateH2)
	sort.Strings(info.EmptyH2)
	return info
}
