// Package opencli provides URL-to-opencli-command routing for various site adapters.
// Each adapter (twitter, youtube, zhihu, etc.) extracts structured content more
// reliably than the generic browser-based web read.
package opencli

import (
	"net/url"
	"strings"

	"github.com/samber/lo"
)

// Adapter constants for known site adapters.
const (
	AdapterTwitter  = "twitter"
	AdapterYoutube  = "youtube"
	AdapterZhihu    = "zhihu"
	AdapterBilibili = "bilibili"
	AdapterWeixin   = "weixin"
	AdapterReddit   = "reddit"
	AdapterHN       = "hackernews"
	AdapterWeb      = "web"
)

// CommandForURL returns the opencli adapter name and arguments for the given URL.
// The caller uses these to build: opencli <adapter> <args...>.
//nolint:gocritic
func CommandForURL(rawURL string) (string, []string) {
	route, found := lo.Find(routes, func(r route) bool {
		return urlMatchesDomain(rawURL, r.domains)
	})
	if !found {
		return AdapterWeb, []string{"read", "--url", rawURL, "--stdout"}
	}

	return route.adapter, argsForRoute(route, rawURL)
}

// urlMatchesDomain checks whether rawURL's hostname matches any of the given
// domain patterns.  It parses the URL so substring matches (e.g. "t.co"
// matching "list.content") cannot happen.
func urlMatchesDomain(rawURL string, domains []string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	hostname := strings.ToLower(parsed.Hostname())

	return lo.SomeBy(domains, func(d string) bool {
		// Exact match or subdomain match: d is "example.com" and hostname
		// is "www.example.com" or "example.com".
		return hostname == d || strings.HasSuffix(hostname, "."+d)
	})
}

// route maps a group of domain patterns to an opencli adapter + subcommand.
type route struct {
	adapter string
	subcmd  string
	domains []string
}

// routes is the URL-to-opencli-command mapping table.
// Ordered by specificity — more precise domains come first.
var routes = []route{
	{AdapterYoutube, "video", []string{"youtube.com", "youtu.be"}},
	{AdapterTwitter, "article", []string{"x.com", "twitter.com", "mobile.twitter.com", "t.co"}},
	{AdapterZhihu, "question", []string{"zhuanlan.zhihu.com", "zhihu.com"}},
	{AdapterBilibili, "video", []string{"bilibili.com", "b23.tv"}},
	{AdapterWeixin, "article", []string{"mp.weixin.qq.com"}},
	{AdapterReddit, "read", []string{"reddit.com"}},
	{AdapterHN, "item", []string{"news.ycombinator.com"}},
}

// HasAdapter reports whether the URL matches a known site-specific adapter.
func HasAdapter(rawURL string) bool {
	_, found := lo.Find(routes, func(r route) bool {
		return urlMatchesDomain(rawURL, r.domains)
	})

	return found
}

// argsForRoute builds the opencli command arguments for a matched route.
func argsForRoute(r route, rawURL string) []string {
	if r.adapter == AdapterWeb {
		return []string{r.subcmd, "--url", rawURL, "--stdout"}
	}

	// Strip query parameters for site-specific adapters — they expect a clean URL.
	// e.g. bilibili/video extracts BV ID from the path; ?spm_id_from= breaks parsing.
	cleanURL := rawURL
	if parsed, err := url.Parse(rawURL); err == nil && parsed.RawQuery != "" {
		parsed.RawQuery = ""
		cleanURL = parsed.String()
	}

	return []string{r.subcmd, cleanURL, "--format", "md"}
}
