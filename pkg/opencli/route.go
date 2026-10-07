// Package opencli provides URL-to-opencli-command routing for various site adapters.
// Each adapter (twitter, youtube, zhihu, etc.) extracts structured content more
// reliably than the generic browser-based web read.
package opencli

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/samber/lo"
)

// tcoDomain is the t.co URL shortener domain used by Twitter/X.
const tcoDomain = "t.co"

// urlFlag is the --url flag used for opencli commands that take a URL argument.
const urlFlag = "--url"

// subcmdVideo is the "video" subcommand used for video site adapters.
const subcmdVideo = "video"

// subcmdRead is the "read" subcommand used for web/fallback reading.
const subcmdRead = "read"

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
func CommandForURL(rawURL string) (string, []string) {
	route, found := routeFor(rawURL)
	if !found {
		return AdapterWeb, []string{subcmdRead, urlFlag, rawURL, "--stdout"}
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
// limit is an adapter-specific argument (e.g. zhihu's answer count) appended
// as `--limit N`; 0 = omit the flag. Adapter knobs are declared here, next to
// their domain, instead of in wiki config — add a column, not a config key.
type route struct {
	adapter string
	subcmd  string
	domains []string
	limit   int
}

// routes is the URL-to-opencli-command mapping table.
// Ordered by specificity — more precise domains come first.
var routes = []route{
	{adapter: AdapterYoutube, subcmd: subcmdVideo, domains: []string{"youtube.com", "youtu.be"}},
	{adapter: AdapterTwitter, subcmd: "thread", domains: []string{"x.com", "twitter.com", "mobile.twitter.com"}},
	{adapter: AdapterWeb, subcmd: subcmdRead, domains: []string{"zhuanlan.zhihu.com"}},
	{adapter: AdapterZhihu, subcmd: "question", domains: []string{"zhihu.com"}, limit: 5},
	{adapter: AdapterBilibili, subcmd: subcmdVideo, domains: []string{"bilibili.com", "b23.tv"}},
	{adapter: AdapterWeixin, subcmd: "download", domains: []string{"mp.weixin.qq.com"}},
	{adapter: AdapterReddit, subcmd: subcmdRead, domains: []string{"reddit.com"}},
	{adapter: AdapterHN, subcmd: "item", domains: []string{"news.ycombinator.com"}},
}

// routeFor returns the matching route for a URL, if any.
func routeFor(rawURL string) (route, bool) {
	return lo.Find(routes, func(r route) bool {
		return urlMatchesDomain(rawURL, r.domains)
	})
}

// HasAdapter reports whether the URL matches a known site-specific adapter.
func HasAdapter(rawURL string) bool {
	_, found := routeFor(rawURL)

	return found
}

// extractZhihuQuestionID extracts the zhihu question ID from a zhihu URL path.
// The path is expected to be /question/{id} or /question/{id}/answer/{answer_id}.
func extractZhihuQuestionID(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 2 && parts[0] == "question" && isNumeric(parts[1]) {
		return parts[1]
	}

	return ""
}

func isNumeric(s string) bool {
	_, err := strconv.Atoi(s)

	return err == nil
}

// argsForRoute builds the opencli command arguments for a matched route.
func argsForRoute(r route, rawURL string) []string {
	if r.adapter == AdapterWeb {
		return []string{r.subcmd, urlFlag, rawURL, "--stdout"}
	}

	// Strip query parameters for site-specific adapters — they expect a clean URL.
	// e.g. bilibili/video extracts BV ID from the path; ?spm_id_from= breaks parsing.
	cleanURL := rawURL
	if parsed, err := url.Parse(rawURL); err == nil && parsed.RawQuery != "" {
		parsed.RawQuery = ""
		cleanURL = parsed.String()
	}

	// zhihu question subcommand expects a numeric question ID, not a full URL.
	if args := zhihuQuestionArgs(r, cleanURL); args != nil {
		return args
	}

	// weixin download uses --url <url> (not positional), and outputs to file
	// by default so we need --format md for stdout-friendly output.
	if r.adapter == AdapterWeixin {
		return []string{r.subcmd, urlFlag, cleanURL, "--format", "md"}
	}

	return []string{r.subcmd, cleanURL, "--format", "md"}
}

// zhihuQuestionArgs builds the zhihu question command from a numeric question
// ID in the path, or nil when the URL is not a zhihu question.
func zhihuQuestionArgs(r route, cleanURL string) []string {
	if r.adapter != AdapterZhihu || r.subcmd != "question" {
		return nil
	}
	parsed, err := url.Parse(cleanURL)
	if err != nil {
		return nil
	}
	id := extractZhihuQuestionID(parsed.Path)
	if id == "" {
		return nil
	}

	args := []string{r.subcmd, id, "--format", "md"}
	// Adapter knob: cap the fetched answers (limit 0 = omit flag).
	if r.limit > 0 {
		args = append(args, "--limit", strconv.Itoa(r.limit))
	}

	return args
}

// IsTcoURL reports whether the URL is a t.co shortlink.
func IsTcoURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	return strings.ToLower(parsed.Hostname()) == tcoDomain
}

// CleanXMediaSuffix removes trailing media path segments (/photo/N, /video/N)
// from resolved X.com/Twitter URLs, returning a clean status URL.
func CleanXMediaSuffix(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	host := strings.ToLower(parsed.Hostname())
	if host != "x.com" && host != "twitter.com" && host != "mobile.twitter.com" {
		return rawURL
	}

	path := strings.TrimRight(parsed.Path, "/")
	parts := strings.Split(path, "/")
	// Expect: ["", "user", "status", "<id>", "photo"|"video", "<n>"]
	if len(parts) >= 6 && parts[len(parts)-4] == "status" &&
		(strings.HasPrefix(parts[len(parts)-2], "photo") || strings.HasPrefix(parts[len(parts)-2], "video")) {
		parts = parts[:len(parts)-2]
		parsed.RawPath = ""
		parsed.Path = strings.Join(parts, "/") + "/"

		return parsed.String()
	}

	return rawURL
}
