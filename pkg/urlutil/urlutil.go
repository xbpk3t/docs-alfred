package urlutil

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
	"unicode"

	"github.com/PuerkitoBio/purell"
	"golang.org/x/net/publicsuffix"
)

// GitHubRepo identifies a GitHub repository by owner and repository name.
type GitHubRepo struct {
	Owner string
	Name  string
}

// Repo identifies a hosted source-code repository by host, owner/group, and name.
type Repo struct {
	Host  string
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

// ValidateURL checks a URL for common agent-hallucinated or adversarial patterns.
// It rejects: non-http(s) schemes, embedded userinfo (@), and control characters.
// Returns nil if the URL is safe to use.
func ValidateURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("empty URL")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("unsupported scheme %q (only http/https allowed)", parsed.Scheme)
	}

	if parsed.User != nil {
		return fmt.Errorf("URL must not contain userinfo (embedded @)")
	}

	for _, r := range rawURL {
		if unicode.IsControl(r) && r != '\t' {
			return fmt.Errorf("URL contains control character U+%04X", r)
		}
	}

	return nil
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
	repo, ok := SourceRepo(rawURL)
	if !ok || repo.Host != "github.com" {
		return GitHubRepo{}, false
	}

	return GitHubRepo{Owner: repo.Owner, Name: repo.Name}, true
}

// SourceRepo parses a supported source-code repository URL.
func SourceRepo(rawURL string) (Repo, bool) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return Repo{}, false
	}

	host := strings.ToLower(u.Hostname())
	if host != "github.com" && host != "gitlab.com" {
		return Repo{}, false
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return Repo{}, false
	}

	return Repo{Host: host, Owner: parts[0], Name: strings.TrimSuffix(parts[1], ".git")}, true
}

// IsSourceRepo returns true for supported source-code repository URLs.
func IsSourceRepo(rawURL string) bool {
	_, ok := SourceRepo(rawURL)

	return ok
}

func repoNameFromPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}

	return strings.TrimSuffix(parts[len(parts)-1], ".git")
}
