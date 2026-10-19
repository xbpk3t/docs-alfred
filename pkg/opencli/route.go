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

// Subcommand constants for routes whose subcommand the caller must recognize.
// They live beside the routes that use them so the driver and the unit
// detector switch on the same names the table dispatches to.
const (
	SubcmdZhihuQuestion     = "question"
	SubcmdZhihuAnswerDetail = "answer-detail"
)

// formatYAML asks opencli for structured output; see route.format.
const formatYAML = "yaml"

// CommandForURL returns the opencli adapter name and arguments for the given URL.
// The caller uses these to build: opencli <adapter> <args...>.
func CommandForURL(rawURL string) (string, []string) {
	route, found := routeFor(rawURL)
	if !found {
		return AdapterWeb, []string{subcmdRead, urlFlag, rawURL, "--stdout"}
	}

	return route.adapter, argsForRoute(&route, rawURL)
}

// SubcommandForURL returns the opencli subcommand the given URL routes to
// (argsForRoute's first element), or "" when no route matches. Callers that
// branch on *which kind* of page a URL is — the wiki fetch driver, and the
// unit detector that picks its prompt — read it from here so the taxonomy
// stays in the route table instead of being re-derived from URL substrings.
func SubcommandForURL(rawURL string) string {
	route, found := routeFor(rawURL)
	if !found {
		return ""
	}

	return route.subcmd
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
// as `--limit N`; 0 = omit the flag. format overrides the default markdown
// rendering for adapters whose output is parsed structurally; "" means
// defaultFormat. Adapter knobs are declared here, next to their domain,
// instead of in wiki config — add a column, not a config key.
//
// pathContains, when set, additionally requires the URL path to contain that
// substring. It lets one domain carry several routes (zhihu.com serves both
// question pages and individual answer permalinks) without a second table.
type route struct {
	adapter      string
	subcmd       string
	pathContains string
	format       string
	domains      []string
	limit        int
}

// defaultFormat is the opencli output format when a route does not override it.
const defaultFormat = "md"

// routes is the URL-to-opencli-command mapping table.
// Ordered by specificity — more precise domains come first.
var routes = []route{
	{adapter: AdapterYoutube, subcmd: subcmdVideo, domains: []string{"youtube.com", "youtu.be"}},
	{adapter: AdapterTwitter, subcmd: "thread", domains: []string{"x.com", "twitter.com", "mobile.twitter.com"}},
	{adapter: AdapterWeb, subcmd: subcmdRead, domains: []string{"zhuanlan.zhihu.com"}},
	// Answer permalinks before the question route: /question/<qid>/answer/<aid>
	// matches both, and only answer-detail returns the full answer body
	// (question truncates every answer to a 200-char preview).
	// Both zhihu routes ask for yaml: the driver parses the rows structurally,
	// and md would mangle a long answer body into one table cell.
	//
	// The question route is path-gated too: the subcommand takes a numeric
	// question id, so a zhihu URL that carries none (zhihu.com/hot) is not a
	// question page and falls through to web read instead of running
	// `zhihu question <url>`.
	{adapter: AdapterZhihu, subcmd: SubcmdZhihuAnswerDetail, domains: []string{"zhihu.com"}, pathContains: "/answer/", format: formatYAML},
	{adapter: AdapterZhihu, subcmd: SubcmdZhihuQuestion, domains: []string{"zhihu.com"}, pathContains: "/question/", format: formatYAML, limit: 3},
	{adapter: AdapterBilibili, subcmd: subcmdVideo, domains: []string{"bilibili.com", "b23.tv"}},
	{adapter: AdapterWeixin, subcmd: "download", domains: []string{"mp.weixin.qq.com"}},
	{adapter: AdapterReddit, subcmd: subcmdRead, domains: []string{"reddit.com"}},
	{adapter: AdapterHN, subcmd: "item", domains: []string{"news.ycombinator.com"}},
}

// routeFor returns the matching route for a URL, if any.
func routeFor(rawURL string) (route, bool) {
	return lo.Find(routes, func(r route) bool {
		if !urlMatchesDomain(rawURL, r.domains) {
			return false
		}
		if r.pathContains == "" {
			return true
		}

		return urlPathContains(rawURL, r.pathContains)
	})
}

// urlPathContains reports whether rawURL's path contains sub.
func urlPathContains(rawURL, sub string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	return strings.Contains(parsed.Path, sub)
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
func argsForRoute(r *route, rawURL string) []string {
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
	// by default so we need an explicit format for stdout-friendly output.
	if r.adapter == AdapterWeixin {
		return []string{r.subcmd, urlFlag, cleanURL, "--format", r.outputFormat()}
	}

	return []string{r.subcmd, cleanURL, "--format", r.outputFormat()}
}

// outputFormat returns the route's declared format, or the default.
func (r *route) outputFormat() string {
	if r.format == "" {
		return defaultFormat
	}

	return r.format
}

// zhihuQuestionArgs builds the zhihu question command from a numeric question
// ID in the path, or nil when the URL is not a zhihu question.
func zhihuQuestionArgs(r *route, cleanURL string) []string {
	if r.adapter != AdapterZhihu || r.subcmd != SubcmdZhihuQuestion {
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

	args := []string{r.subcmd, id, "--format", r.outputFormat()}
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
