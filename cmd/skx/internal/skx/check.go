package skx

import (
	"fmt"
	"sort"
	"strings"
)

// Severity distinguishes blocking errors from advisory warnings.
type Severity int

const (
	// Error fails the check.
	Error Severity = iota
	// Warning is printed but does not fail the check.
	Warning
)

func (s Severity) String() string {
	if s == Warning {
		return "warning"
	}
	return "error"
}

// Issue is one validation finding for a single prompt file.
type Issue struct {
	Path     string
	Message  string
	Severity Severity
}

// CheckResult aggregates validation findings across a directory.
type CheckResult struct {
	Issues   []Issue
	Files    int
	Errors   int
	Warnings int
}

// HasErrors reports whether any blocking issue was found.
func (r *CheckResult) HasErrors() bool { return r.Errors > 0 }

// CheckDir validates every *.md under dir: frontmatter, no H1, body budget,
// pipeline references, duplicate/empty H2, and fence balance. Warnings do not
// fail the check; errors do.
func CheckDir(dir string) (*CheckResult, error) {
	idx, err := indexDir(dir)
	if err != nil {
		return nil, err
	}

	res := &CheckResult{Files: len(idx.entries)}
	for _, e := range idx.entries {
		if e.err != nil {
			res.Issues = append(res.Issues, Issue{Path: e.path, Message: e.err.Error(), Severity: Error})
			continue
		}
		res.Issues = append(res.Issues, checkPrompt(e.prompt, idx.routable)...)
	}
	for _, d := range idx.duplicates {
		res.Issues = append(res.Issues, Issue{
			Path:     d.shadowed,
			Message:  fmt.Sprintf("duplicate routing name %q: %s wins routing", d.name, d.winner),
			Severity: Warning,
		})
	}

	g, _ := buildGraph(idx)
	for _, cyc := range g.Cycles {
		if len(cyc) == 2 && cyc[0] == cyc[1] {
			continue // self-reference is reported per file
		}
		res.Issues = append(res.Issues, Issue{
			Path:     idx.routable[cyc[0]].Path,
			Message:  "pipeline cycle: " + strings.Join(cyc, " → "),
			Severity: Error,
		})
	}

	for _, iss := range res.Issues {
		if iss.Severity == Error {
			res.Errors++
		} else {
			res.Warnings++
		}
	}

	sort.SliceStable(res.Issues, func(i, j int) bool {
		if res.Issues[i].Path != res.Issues[j].Path {
			return res.Issues[i].Path < res.Issues[j].Path
		}
		if res.Issues[i].Severity != res.Issues[j].Severity {
			return res.Issues[i].Severity == Error
		}
		return res.Issues[i].Message < res.Issues[j].Message
	})
	return res, nil
}

// checkPrompt validates one already-parsed prompt. routable is the set of
// routing names that pipeline entries may reference.
func checkPrompt(p *Prompt, routable map[string]*Prompt) []Issue {
	var issues []Issue
	add := func(sev Severity, format string, a ...any) {
		issues = append(issues, Issue{Path: p.Path, Message: fmt.Sprintf(format, a...), Severity: sev})
	}

	for _, k := range p.UnknownKeys {
		add(Error, "unknown frontmatter key %q", k)
	}

	switch p.Role {
	case "atom":
		if len(p.Pipeline) > 0 {
			add(Error, "atom must not declare pipeline")
		}
	case "composite":
		if len(p.Pipeline) == 0 {
			add(Error, "composite requires non-empty pipeline")
		}
	default:
		add(Error, "role must be atom|composite, got %q", p.Role)
	}

	seen := map[string]bool{}
	for _, dep := range p.Pipeline {
		// Compare the resolved path, not the bare stem: a nested file whose
		// stem is shadowed references the winning file, not itself.
		if rp, ok := routable[dep]; ok && rp.Path == p.Path {
			add(Error, "pipeline includes itself")
		}
		if seen[dep] {
			add(Warning, "duplicate pipeline entry %q", dep)
		}
		seen[dep] = true
		if _, ok := routable[dep]; !ok {
			add(Error, "pipeline target %q has no routable prompt", dep)
		}
	}

	md := analyzeMarkdownFrom(p.Body, p.FrontmatterLines)
	for _, h := range md.H1 {
		add(Error, "H1 heading at line %d: %q", h.Line, h.Text)
	}
	for _, d := range md.DuplicateH2 {
		add(Warning, "duplicate H2 %q", d)
	}
	for _, e := range md.EmptyH2 {
		add(Warning, "empty H2 %q", e)
	}
	for _, f := range md.FenceIssues {
		add(Warning, "%s", f)
	}
	if md.Chars > BodyCharWarn {
		add(Warning, "body %d chars > %d", md.Chars, BodyCharWarn)
	}
	return issues
}
