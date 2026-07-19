package blog

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/adrg/frontmatter"
	carbon "github.com/dromara/carbon/v2"
	"github.com/samber/lo"
)

// BlogMeta is a type:blog file under a topic.
type BlogMeta struct {
	ModTime time.Time
	Path    string
	Title   string
	Date    string
}

// ListTopicBlogs lists markdown files with type: blog under topicDir.
func ListTopicBlogs(topicDir string) ([]BlogMeta, error) {
	entries, err := os.ReadDir(topicDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []BlogMeta
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			continue
		}
		// Skip log/summary by name.
		base := strings.ToLower(e.Name())
		if base == "log.md" || base == "summary.md" {
			continue
		}
		full := filepath.Join(topicDir, e.Name())
		meta, ok, err := readBlogMeta(full)
		if err != nil || !ok {
			continue
		}
		out = append(out, meta)
	}
	return out, nil
}

func readBlogMeta(path string) (BlogMeta, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return BlogMeta{}, false, err
	}
	var fm struct {
		Title string `yaml:"title"`
		Date  string `yaml:"date"`
		Type  string `yaml:"type"`
	}
	if _, err := frontmatter.Parse(strings.NewReader(string(data)), &fm); err != nil {
		return BlogMeta{}, false, err
	}
	if strings.TrimSpace(strings.ToLower(fm.Type)) != "blog" {
		return BlogMeta{}, false, nil
	}
	info, _ := os.Stat(path)
	mod := time.Time{}
	if info != nil {
		mod = info.ModTime()
	}
	title := strings.TrimSpace(fm.Title)
	if title == "" {
		title = filepath.Base(path)
	}
	return BlogMeta{
		Path:    path,
		Title:   title,
		Date:    strings.TrimSpace(fm.Date),
		ModTime: mod,
	}, true, nil
}

// TopicHasNewBlogInWindow reports whether any type:blog under topicAbsDir was
// authored in the cooling window. Primary signals are frontmatter date and
// filename YYYY-MM-DD- prefix (aligned with commit-first hot scoring). File
// mtime is last resort only when both dates are absent.
func TopicHasNewBlogInWindow(topicAbsDir string, since time.Time) (bool, []BlogMeta, error) {
	blogs, err := ListTopicBlogs(topicAbsDir)
	if err != nil {
		return false, nil, err
	}
	var recent []BlogMeta
	for _, b := range blogs {
		if blogInWindow(b, since) {
			recent = append(recent, b)
		}
	}
	return len(recent) > 0, recent, nil
}

// blogInWindow prefers authoring dates over mtime so repo reorg/checkout does
// not treat old blogs as "compacted this week".
func blogInWindow(b BlogMeta, since time.Time) bool {
	sinceDay := carbon.CreateFromStdTime(since).StartOfDay()

	if d, ok := parseDateOnly(b.Date); ok {
		return !d.Lt(sinceDay)
	}
	// Filename prefix YYYY-MM-DD-
	base := filepath.Base(b.Path)
	if len(base) >= 10 {
		if d, ok := parseDateOnly(base[:10]); ok {
			return !d.Lt(sinceDay)
		}
	}
	// Last resort: mtime only when no authoring date is available.
	if !b.ModTime.IsZero() && !b.ModTime.Before(since) {
		return true
	}
	return false
}

// parseDateOnly parses YYYY-MM-DD using carbon (Asia/Shanghai after carboninit.Setup).
func parseDateOnly(s string) (*carbon.Carbon, bool) {
	s = strings.TrimSpace(s)
	if len(s) < 10 {
		return nil, false
	}
	c := carbon.ParseByLayout(s[:10], carbon.DateLayout)
	if c.HasError() || c.IsInvalid() {
		return nil, false
	}
	return c, true
}

// BlogTitles formats blog list for AI/mail.
func BlogTitles(blogs []BlogMeta) []string {
	return lo.Map(blogs, func(b BlogMeta, _ int) string {
		if b.Date != "" {
			return b.Date + " — " + b.Title
		}
		return b.Title
	})
}
