package ghindex

import (
	"path"
	"strings"

	"github.com/xbpk3t/docs-alfred/internal/gh/content"
	"github.com/xbpk3t/docs-alfred/internal/gh/ghcheck"
	"github.com/xbpk3t/docs-alfred/pkg/urlutil"
)

// DefaultTopicKinds is the formal topic set shared by TopicCatalog, dump, and
// wiki classify/write/check. kind=temp is intentionally excluded.
var DefaultTopicKinds = []string{
	ghcheck.KindMechanism,
	ghcheck.KindType,
	ghcheck.KindRepo,
	ghcheck.KindTools,
}

// TopicCandidate is a wiki-ready topic path extracted from gh.yml.
type TopicCandidate struct {
	Path    string `json:"path"`
	Display string `json:"display,omitempty"`
	Source  string `json:"source,omitempty"`
}

// TopicCatalog extracts formal topic paths (DefaultTopicKinds only).
func (cr ConfigRepos) TopicCatalog() []TopicCandidate {
	return cr.TopicCatalogWithKinds(DefaultTopicKinds)
}

// TopicCatalogWithKinds extracts topic paths whose kind is in kinds.
// Empty kinds yields an empty catalog.
func (cr ConfigRepos) TopicCatalogWithKinds(kinds []string) []TopicCandidate {
	allow := kindSet(kinds)
	seen := make(map[string]bool)
	var candidates []TopicCandidate
	for _, cfg := range cr {
		if cfg == nil {
			continue
		}
		base := joinPath(cfg.Tag, cfg.Type)
		candidates = appendTopicCandidates(candidates, seen, cfg.Topics, base, "gh:config", allow)
		appendRepoTopicCandidates(&candidates, seen, cfg.Repos, cfg.Tag, cfg.Type, allow)
	}

	return candidates
}

// KindAllowed reports whether kind is in the allow set (trim applied).
func KindAllowed(kind string, allow map[string]struct{}) bool {
	if len(allow) == 0 {
		return false
	}
	_, ok := allow[strings.TrimSpace(kind)]

	return ok
}

// KindSet builds an allow-set from kind names.
func KindSet(kinds []string) map[string]struct{} {
	return kindSet(kinds)
}

func kindSet(kinds []string) map[string]struct{} {
	set := make(map[string]struct{}, len(kinds))
	for _, k := range kinds {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		set[k] = struct{}{}
	}

	return set
}

func appendRepoTopicCandidates(
	candidates *[]TopicCandidate,
	seen map[string]bool,
	repos Repos,
	tag,
	typeName string,
	allow map[string]struct{},
) {
	for _, repo := range repos {
		if repo == nil {
			continue
		}
		repoName := urlutil.RepoName(repo.URL)
		_ = repoName
		appendRepoTopicCandidates(candidates, seen, repo.RelatedRepos, tag, typeName, allow)
	}
}

func appendTopicCandidates(
	candidates []TopicCandidate,
	seen map[string]bool,
	topics content.Topics,
	base,
	source string,
	allow map[string]struct{},
) []TopicCandidate {
	for i := range topics {
		topic := &topics[i]
		if !KindAllowed(topic.Kind, allow) {
			continue
		}
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
