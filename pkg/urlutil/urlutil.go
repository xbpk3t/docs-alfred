package urlutil

import (
	"net/url"
	"slices"
	"strings"

	"github.com/PuerkitoBio/purell"
	"golang.org/x/net/publicsuffix"
)

// GitHubRepo identifies a GitHub repository by owner and repository name.
type GitHubRepo struct {
	Owner string
	Name  string
}

// Normalize canonicalizes a URL for comparison and persistence keys.
func Normalize(rawURL string) string {
	normalized, err := purell.NormalizeURLString(rawURL,
		purell.FlagLowercaseScheme|
			purell.FlagLowercaseHost|
			purell.FlagRemoveDefaultPort|
			purell.FlagRemoveDotSegments|
			purell.FlagRemoveFragment|
			purell.FlagRemoveDuplicateSlashes|
			purell.FlagSortQuery)
	if err != nil {
		return strings.TrimRight(rawURL, "/")
	}

	return strings.TrimRight(normalized, "/")
}

// Equal compares URLs using Normalize.
func Equal(a, b string) bool {
	return strings.EqualFold(Normalize(a), Normalize(b))
}

// Domain returns the URL hostname without port.
func Domain(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	return strings.ToLower(u.Hostname())
}

// RegistrableDomain returns the eTLD+1 for a hostname.
func RegistrableDomain(domain string) string {
	regDomain, err := publicsuffix.EffectiveTLDPlusOne(strings.ToLower(domain))
	if err != nil {
		return ""
	}

	return regDomain
}

// DomainBlocked checks exact, registrable-domain, and suffix-style block rules.
func DomainBlocked(domain string, blockedSet map[string]bool) bool {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return false
	}
	if blockedSet[domain] {
		return true
	}
	if regDomain := RegistrableDomain(domain); regDomain != "" {
		return blockedSet[regDomain]
	}

	parts := strings.Split(domain, ".")
	for i := range slices.Backward(parts) {
		if blockedSet[strings.Join(parts[i:], ".")] {
			return true
		}
	}

	return false
}

// RepoName returns the last path segment, trimming a trailing .git suffix.
func RepoName(rawURL string) string {
	if rawURL == "" {
		return ""
	}

	u, err := url.Parse(rawURL)
	if err == nil && u.Path != "" {
		return repoNameFromPath(u.Path)
	}

	cleaned := strings.TrimPrefix(rawURL, "https://")
	cleaned = strings.TrimPrefix(cleaned, "http://")

	return repoNameFromPath(cleaned)
}

// GitHubOwnerRepo parses a github.com repository URL.
func GitHubOwnerRepo(rawURL string) (GitHubRepo, bool) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return GitHubRepo{}, false
	}
	if !strings.EqualFold(u.Hostname(), "github.com") {
		return GitHubRepo{}, false
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return GitHubRepo{}, false
	}

	return GitHubRepo{Owner: parts[0], Name: strings.TrimSuffix(parts[1], ".git")}, true
}

func repoNameFromPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}

	return strings.TrimSuffix(parts[len(parts)-1], ".git")
}
