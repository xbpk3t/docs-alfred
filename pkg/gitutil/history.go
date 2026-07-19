package gitutil

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"
	"unicode"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/sergi/go-diff/diffmatchpatch"
)

// LogEdit is one substantive content edit of a **/log.md path in a non-bulk commit.
type LogEdit struct {
	When       time.Time
	Path       string
	CommitHash string
	Diff       string
	DeltaChars int
	DeltaLines int
}

// CollectLogEditOptions controls commit-first log.md heat collection.
type CollectLogEditOptions struct {
	// Since is exclusive lower bound on committer time (git log --since).
	// Commits with When <= Since are excluded (half-open lower bound).
	Since time.Time
	// Until is exclusive upper bound on committer time (git log --until).
	// Zero means open-ended (no upper bound). When set, commits with
	// When >= Until are excluded (half-open [Since, Until)).
	Until time.Time
	// PathPrefix limits to paths under this slash-separated prefix (e.g. "wiki").
	// Empty means any **/log.md.
	PathPrefix string
	// BulkLogThreshold: if a single commit touches this many **/log.md paths, the
	// whole commit is ignored. Default 10.
	BulkLogThreshold int
	// MinDeltaChars: non-whitespace rune delta threshold. Default 40.
	MinDeltaChars int
	// MinDeltaLines: non-empty line ± threshold. Default 2.
	MinDeltaLines int
	// MaxDiffRunes caps Diff text per edit (0 = 4000).
	MaxDiffRunes int
}

// CollectLogEdits walks commits in (opts.Since, opts.Until) once (commit-first),
// drops bulk commits, and returns substantive content edits of **/log.md paths.
// Until zero means open-ended. 100% renames and whitespace-only changes are discarded.
func CollectLogEdits(repoPath string, opts *CollectLogEditOptions) ([]LogEdit, error) {
	if opts == nil {
		opts = &CollectLogEditOptions{}
	}
	normalizeCollectOpts(opts)

	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("open repo %s: %w", repoPath, err)
	}

	logOpts := &git.LogOptions{
		Order: git.LogOrderCommitterTime,
		Since: &opts.Since,
	}
	if !opts.Until.IsZero() {
		until := opts.Until
		logOpts.Until = &until
	}

	iter, err := repo.Log(logOpts)
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}
	defer iter.Close()

	var edits []LogEdit
	err = iter.ForEach(func(c *object.Commit) error {
		when := c.Committer.When
		if !when.After(opts.Since) {
			return nil
		}
		if !opts.Until.IsZero() && !when.Before(opts.Until) {
			return nil
		}
		commitEdits, cErr := collectCommitLogEdits(c, opts)
		if cErr != nil {
			return cErr
		}
		edits = append(edits, commitEdits...)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return edits, nil
}

func normalizeCollectOpts(opts *CollectLogEditOptions) {
	if opts.BulkLogThreshold <= 0 {
		opts.BulkLogThreshold = 10
	}
	if opts.MinDeltaChars <= 0 {
		opts.MinDeltaChars = 40
	}
	if opts.MinDeltaLines <= 0 {
		opts.MinDeltaLines = 2
	}
	if opts.MaxDiffRunes <= 0 {
		opts.MaxDiffRunes = 4000
	}
	opts.PathPrefix = strings.Trim(strings.ReplaceAll(opts.PathPrefix, "\\", "/"), "/")
}

type logChange struct {
	change *object.Change
	toPath string
}

func collectCommitLogEdits(c *object.Commit, opts *CollectLogEditOptions) ([]LogEdit, error) {
	toTree, err := c.Tree()
	if err != nil {
		return nil, fmt.Errorf("commit %s tree: %w", c.Hash.String()[:8], err)
	}

	fromTree, err := parentTree(c)
	if err != nil {
		return nil, err
	}

	changes, err := diffCommitTrees(fromTree, toTree, c.Hash.String()[:8])
	if err != nil {
		return nil, err
	}

	logChanges := filterLogChanges(changes, opts)
	if len(logChanges) >= opts.BulkLogThreshold {
		return nil, nil
	}

	return buildLogEdits(c, logChanges, opts)
}

func parentTree(c *object.Commit) (*object.Tree, error) {
	if c.NumParents() == 0 {
		return nil, nil
	}
	parent, err := c.Parent(0)
	if err != nil {
		return nil, fmt.Errorf("commit %s parent: %w", c.Hash.String()[:8], err)
	}
	fromTree, err := parent.Tree()
	if err != nil {
		return nil, fmt.Errorf("commit %s parent tree: %w", c.Hash.String()[:8], err)
	}
	return fromTree, nil
}

func diffCommitTrees(fromTree, toTree *object.Tree, shortHash string) (object.Changes, error) {
	// Rename detection helps attribute renames; empty content still filtered later.
	changes, err := object.DiffTreeWithOptions(context.Background(), fromTree, toTree, &object.DiffTreeOptions{
		DetectRenames:    true,
		OnlyExactRenames: true,
	})
	if err != nil {
		// Fallback without rename detection.
		changes, err = object.DiffTree(fromTree, toTree)
		if err != nil {
			return nil, fmt.Errorf("commit %s difftree: %w", shortHash, err)
		}
	}
	return changes, nil
}

func filterLogChanges(changes object.Changes, opts *CollectLogEditOptions) []logChange {
	var logChanges []logChange
	for _, ch := range changes {
		toPath := ch.To.Name
		fromPath := ch.From.Name
		pathName := toPath
		if pathName == "" {
			pathName = fromPath
		}
		if !isLogMD(pathName) {
			// Rename: either side may be log.md
			if !isLogMD(fromPath) && !isLogMD(toPath) {
				continue
			}
			if toPath == "" {
				continue // pure delete of log.md — not a hot topic signal
			}
			pathName = toPath
		}
		if opts.PathPrefix != "" && !pathUnderPrefix(pathName, opts.PathPrefix) {
			continue
		}
		// Prefer To path for heat attribution.
		if toPath == "" {
			continue
		}
		logChanges = append(logChanges, logChange{change: ch, toPath: toPath})
	}
	return logChanges
}

func buildLogEdits(c *object.Commit, logChanges []logChange, opts *CollectLogEditOptions) ([]LogEdit, error) {
	var edits []LogEdit
	hash := c.Hash.String()
	when := c.Committer.When.UTC()
	for _, lc := range logChanges {
		edit, ok, err := editFromChange(lc, hash, when, opts)
		if err != nil {
			return nil, err
		}
		if ok {
			edits = append(edits, edit)
		}
	}
	return edits, nil
}

func editFromChange(lc logChange, hash string, when time.Time, opts *CollectLogEditOptions) (LogEdit, bool, error) {
	fromFile, toFile, err := lc.change.Files()
	if err != nil {
		return LogEdit{}, false, fmt.Errorf("commit %s files %s: %w", hash[:8], lc.toPath, err)
	}
	var oldContent, newContent string
	if fromFile != nil {
		oldContent, err = fromFile.Contents()
		if err != nil {
			return LogEdit{}, false, err
		}
	}
	if toFile != nil {
		newContent, err = toFile.Contents()
		if err != nil {
			return LogEdit{}, false, err
		}
	}
	deltaChars, deltaLines, diffText := contentDelta(oldContent, newContent, opts.MaxDiffRunes)
	if !isSubstantive(deltaChars, deltaLines, opts.MinDeltaChars, opts.MinDeltaLines) {
		return LogEdit{}, false, nil
	}
	return LogEdit{
		Path:       lc.toPath,
		CommitHash: hash,
		When:       when,
		DeltaChars: deltaChars,
		DeltaLines: deltaLines,
		Diff:       diffText,
	}, true, nil
}

func isLogMD(p string) bool {
	p = strings.ReplaceAll(p, "\\", "/")
	return p == "log.md" || strings.HasSuffix(p, "/log.md")
}

func pathUnderPrefix(p, prefix string) bool {
	p = strings.ReplaceAll(p, "\\", "/")
	return p == prefix || strings.HasPrefix(p, prefix+"/")
}

// isSubstantive reports whether a content change clears the char/line thresholds.
func isSubstantive(deltaChars, deltaLines, minChars, minLines int) bool {
	return deltaChars >= minChars || deltaLines >= minLines
}

// contentDelta returns non-whitespace char delta, non-empty line ± count, and a
// short diff summary. Identical / whitespace-only content yields zeros.
func contentDelta(oldContent, newContent string, maxDiffRunes int) (deltaChars, deltaLines int, diffText string) {
	oldNorm := stripInsignificant(oldContent)
	newNorm := stripInsignificant(newContent)
	if oldNorm == newNorm {
		return 0, 0, ""
	}

	// Char delta: absolute difference in non-whitespace runes between sides,
	// plus Levenshtein-ish via diffmatchpatch on compact form.
	oldCompact := removeWhitespace(oldContent)
	newCompact := removeWhitespace(newContent)
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(oldCompact, newCompact, false)
	for _, d := range diffs {
		n := len([]rune(d.Text))
		switch d.Type {
		case diffmatchpatch.DiffInsert, diffmatchpatch.DiffDelete:
			deltaChars += n
		}
	}

	oldLines := nonEmptyLines(oldContent)
	newLines := nonEmptyLines(newContent)
	// Line-oriented diff count.
	chars1, chars2, lineArray := dmp.DiffLinesToChars(
		strings.Join(oldLines, "\n")+"\n",
		strings.Join(newLines, "\n")+"\n",
	)
	lineDiffs := dmp.DiffMain(chars1, chars2, false)
	lineDiffs = dmp.DiffCharsToLines(lineDiffs, lineArray)
	for _, d := range lineDiffs {
		if d.Type == diffmatchpatch.DiffEqual {
			continue
		}
		deltaLines += countLines(d.Text)
	}

	diffText = buildDiffSummary(oldContent, newContent, maxDiffRunes)
	return deltaChars, deltaLines, diffText
}

func stripInsignificant(s string) string {
	lines := splitLines(s)
	var b strings.Builder
	for _, line := range lines {
		b.WriteString(strings.TrimRightFunc(line, unicode.IsSpace))
		b.WriteByte('\n')
	}
	return b.String()
}

func removeWhitespace(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, line := range splitLines(s) {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}

func buildDiffSummary(oldContent, newContent string, maxRunes int) string {
	// Prefer showing added non-empty lines (common for log appends).
	oldSet := make(map[string]int)
	for _, line := range nonEmptyLines(oldContent) {
		oldSet[line]++
	}
	var added []string
	for _, line := range nonEmptyLines(newContent) {
		if oldSet[line] > 0 {
			oldSet[line]--
			continue
		}
		added = append(added, "+ "+line)
	}
	var removed []string
	for line, n := range oldSet {
		for i := 0; i < n; i++ {
			removed = append(removed, "- "+line)
		}
	}
	// Cap line counts for summary.
	const maxLines = 40
	parts := make([]string, 0, len(removed)+len(added))
	parts = append(parts, removed...)
	parts = append(parts, added...)
	if len(parts) > maxLines {
		parts = append(parts[:maxLines], fmt.Sprintf("... (%d more lines)", len(parts)-maxLines))
	}
	text := strings.Join(parts, "\n")
	runes := []rune(text)
	if maxRunes > 0 && len(runes) > maxRunes {
		text = string(runes[:maxRunes]) + "\n... (diff truncated)"
	}
	return text
}

// TopicDirFromLogPath returns wiki/<folder>/<type>/<topic> from a log.md path.
func TopicDirFromLogPath(logPath string) string {
	logPath = strings.ReplaceAll(logPath, "\\", "/")
	if !isLogMD(logPath) {
		return ""
	}
	return path.Dir(logPath)
}
