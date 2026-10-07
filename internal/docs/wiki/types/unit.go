package types

import (
	"net/url"
	"strings"

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
	// future rss-style Q&A sites): digested per-answer with a synthesis header.
	UnitQnA UnitType = "qna"
)

// DetectUnit determines the processing unit for a URL.
// Pure URL classification — no network, no content inspection. Zhihu's
// question threads are the one QnA unit today; host classification is
// delegated to the opencli route table (zhuanlan.zhihu.com resolves to
// AdapterWeb, zhihu.com to AdapterZhihu), which is the single place that
// maps hosts to behavior.
func DetectUnit(rawURL string) UnitType {
	adapter, _ := opencli.CommandForURL(rawURL)
	if adapter == opencli.AdapterZhihu {
		u, err := url.Parse(rawURL)
		if err == nil && strings.Contains(u.Path, "/question/") {
			return UnitQnA
		}
	}

	return UnitArticle
}
