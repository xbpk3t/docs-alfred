package ghindex

import (
	"path"
	"strings"

	"github.com/xbpk3t/docs-alfred/pkg/urlutil"
	"github.com/xbpk3t/docs-alfred/service/content"
)

// TopicCandidate is a wiki-ready topic path extracted from gh.yml.
type TopicCandidate struct {
	Path    string `json:"path"`
	Display string `json:"display,omitempty"`
	Source  string `json:"source,omitempty"`
}

// TopicCatalog extracts all topic paths from config-level, using-repo, and repo topics.
func (cr ConfigRepos) TopicCatalog() []TopicCandidate {
	seen := make(map[string]bool)
	var candidates []TopicCandidate
	for _, cfg := range cr {
		if cfg == nil {
			continue
		}
		base := joinPath(cfg.Tag, cfg.Type)
		candidates = appendTopicCandidates(candidates, seen, cfg.Topics, base, "gh:config")
		candidates = appendTopicCandidates(candidates, seen, cfg.Using.Topics, base, "gh:using")
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
		base := joinPath(tag, typeName, repoName)
		*candidates = appendTopicCandidates(*candidates, seen, repo.Topics, base, "gh:repo")
		appendRepoTopicCandidates(candidates, seen, repo.SubRepos, tag, typeName)
		appendRepoTopicCandidates(candidates, seen, repo.ReplacedRepos, tag, typeName)
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
		candidates = appendTopicCandidates(candidates, seen, topic.Sub, topicPath, source)
	}

	return candidates
}

func canonicalTopicPath(topic *content.Topic, base string) string {
	if topic == nil {
		return cleanCatalogPath(base)
	}
	if topic.PicDir != "" {
		return cleanCatalogPath(topic.PicDir)
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
