package ghindex

import (
	"path"
	"strings"

	"github.com/xbpk3t/docs-alfred/internal/gh/content"
	"github.com/xbpk3t/docs-alfred/pkg/urlutil"
)

// TopicCandidate is a wiki-ready topic path extracted from gh.yml.
type TopicCandidate struct {
	Path    string `json:"path"`
	Display string `json:"display,omitempty"`
	Source  string `json:"source,omitempty"`
}

// TopicCatalog extracts all topic paths from config-level topics.
func (cr ConfigRepos) TopicCatalog() []TopicCandidate {
	seen := make(map[string]bool)
	var candidates []TopicCandidate
	for _, cfg := range cr {
		if cfg == nil {
			continue
		}
		base := joinPath(cfg.Tag, cfg.Type)
		candidates = appendTopicCandidates(candidates, seen, cfg.Topics, base, "gh:config")
		appendRepoTopicCandidates(&candidates, seen, cfg.Repos, cfg.Tag, cfg.Type)
	}

	return candidates
}

func appendRepoTopicCandidates(
	candidates *[]TopicCandidate,
	seen map[string]bool,
	repos Repos,
	tag,
	typeName string,
) {
	for _, repo := range repos {
		if repo == nil {
			continue
		}
		repoName := urlutil.RepoName(repo.URL)
		_ = repoName
		appendRepoTopicCandidates(candidates, seen, repo.RelatedRepos, tag, typeName)
	}
}

func appendTopicCandidates(
	candidates []TopicCandidate,
	seen map[string]bool,
	topics content.Topics,
	base,
	source string,
) []TopicCandidate {
	for i := range topics {
		topic := &topics[i]
		topicPath := canonicalTopicPath(topic, base)
		if isCatalogPathSafe(topicPath) && !seen[topicPath] {
			seen[topicPath] = true
			candidates = append(candidates, TopicCandidate{
				Path:    topicPath,
				Display: topic.Topic,
				Source:  source,
			})
		}
	}

	return candidates
}

func canonicalTopicPath(topic *content.Topic, base string) string {
	if topic == nil {
		return cleanCatalogPath(base)
	}

	return cleanCatalogPath(joinPath(base, topicDirName(topic)))
}

func cleanCatalogPath(candidate string) string {
	candidate = strings.TrimSpace(strings.ReplaceAll(candidate, "\\", "/"))
	if candidate == "" {
		return ""
	}

	return strings.Trim(path.Clean(candidate), "/")
}

func isCatalogPathSafe(candidate string) bool {
	if candidate == "" || strings.HasPrefix(candidate, "/") {
		return false
	}
	for segment := range strings.SplitSeq(candidate, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
		if strings.ContainsAny(segment, "\x00\n\r") {
			return false
		}
	}

	return true
}
