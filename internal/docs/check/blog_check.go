package workspaceops

import (
	"fmt"
	"io/fs"
	"path/filepath"

	wikitypes "github.com/xbpk3t/docs-alfred/internal/docs/wiki/types"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
)

// RunBlogCheckOKF validates OKF v0.1 frontmatter on all blog posts under
// blogRoot (blog/<tag>/<type>/<file>.md). Blog is a sibling entity of wiki/,
// so it walks its own root and reuses the shared OKF field validators
// (checkRequiredFields / checkDateFormat / checkTypeValidity) via checkOKFFile.
func RunBlogCheckOKF(blogRoot string) ([]checkutil.Issue, error) {
	var issues []checkutil.Issue
	err := filepath.WalkDir(blogRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			issues = append(issues, checkutil.Issue{
				File:     slashRel(blogRoot, path),
				Severity: checkutil.SeverityError,
				Message:  fmt.Sprintf("walk error: %v", walkErr),
			})

			return nil
		}
		if d.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		rel := slashRel(blogRoot, path)
		// blog/public/ holds build artifacts (redirects.json), never posts.
		if checkutil.HasSegmentDir(rel, wikitypes.BlogArtifactDir) {
			return nil
		}
		issues = append(issues, checkOKFFile(path, rel, blogPostType)...)

		return nil
	})

	return issues, err
}

// blogPostType binds every blog post under blog/ to type=blog, regardless of
// location: the blog tree's only OKF entry type is blog.
func blogPostType(string) string { return string(wikitypes.TypeBlog) }
