// Package opencli provides URL-to-opencli-command routing for various site adapters.
// Each adapter (twitter, youtube, zhihu, etc.) extracts structured content more
// reliably than the generic browser-based web read.
package opencli

import (
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
	lower := strings.ToLower(rawURL)
	route, found := lo.Find(routes, func(r route) bool {
		return lo.SomeBy(r.domains, func(d string) bool { return strings.Contains(lower, d) })
	})
	if !found {
		return AdapterWeb, []string{"read", "--url", rawURL, "--stdout"}
	}

	return route.adapter, argsForRoute(route, rawURL)
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

// argsForRoute builds the opencli command arguments for a matched route.
func argsForRoute(r route, rawURL string) []string {
	if r.adapter == AdapterWeb {
		return []string{r.subcmd, "--url", rawURL, "--stdout"}
	}

	return []string{r.subcmd, rawURL, "--format", "md"}
}
