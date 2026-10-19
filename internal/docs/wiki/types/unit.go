package types

import (
	"github.com/xbpk3t/docs-alfred/pkg/opencli"
)

// UnitType classifies a URL into a content-processing unit. Each unit has its
// own prompt template and digest shape; the mapping lives in the classifier's
// prompt registry (see internal/docs/wiki/classify promptForUnit).
type UnitType string

const (
	// UnitArticle is the default unit: a single piece of content (article,
	// video, podcast, repo review) digested as one block.
	UnitArticle UnitType = "article"
	// UnitQnA is a question with multiple answers (zhihu.com/question, and
	// future rss-style Q&A sites): digested as a synthesis of the thread, with
	// each answer becoming its own UnitAnswer entry.
	UnitQnA UnitType = "qna"
	// UnitAnswer is one answer inside a Q&A thread, addressed by its own
	// permalink. It is digested as a standalone post that inherits the
	// thread's topic rather than choosing one.
	UnitAnswer UnitType = "answer"
)

// DetectUnit determines the processing unit for a URL.
// Pure URL classification — no network, no content inspection. Which zhihu
// page a URL is comes from the opencli route table, the single place that maps
// URLs to behavior; re-testing the path here would be a second, silently
// divergent definition of the same taxonomy.
func DetectUnit(rawURL string) UnitType {
	adapter, _ := opencli.CommandForURL(rawURL)
	if adapter != opencli.AdapterZhihu {
		return UnitArticle
	}

	switch opencli.SubcommandForURL(rawURL) {
	case opencli.SubcmdZhihuAnswerDetail:
		return UnitAnswer
	case opencli.SubcmdZhihuQuestion:
		return UnitQnA
	default:
		return UnitArticle
	}
}
