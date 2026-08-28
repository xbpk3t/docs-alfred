package urlutil

import (
	_ "embed"
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
	"sync"
)

// ClearURLs community rule data.
//
// Source: https://rules2.clearurls.xyz/data.minify.json
// (ClearURLs rules: https://github.com/ClearURLs/Rules)
// and its alongside digest https://rules2.clearurls.xyz/rules.minify.hash.
//
// The rule regexes and the matching semantics below follow the reference
// engine in walterl/uroute (https://github.com/walterl/uroute, MIT) and the
// ClearURLs author's snippet. Only the *parameter names yielded* are acted on;
// the regexes themselves remain ClearURLs' own (GPL-3.0 rules source).
//
//go:embed data/data.minify.json
var ruleDataJSON string

//go:embed data/rules.minify.hash
var ruleDataHash string

// RulesDataHash returns the SHA-256 digest of the embedded rule data as
// published by the upstream rules.minify.hash endpoint, for provenance and
// reproducibility checks. The trailing newline in the file is trimmed.
func RulesDataHash() string { return strings.TrimSpace(ruleDataHash) }

// maxRedirectDepth bounds redirection expansion (e.g. a google /url that points
// at a chain of further redirects), guarding against cycles.
const maxRedirectDepth = 5

// guardParams are content-bearing query parameters that must never be
// stripped, even when a rule matches them. Dropping these wrongly merges two
// distinct pages (e.g. ?v= a video, ?bvid= a bilibili video, ?q= a search).
// They always take precedence over ClearURLs rules.
var guardParams = map[string]bool{
	"v": true, "bvid": true, "q": true, "id": true,
	"page": true, "offset": true, "limit": true, "tag": true,
	"tab": true, "sort": true, "order": true, "view": true,
	"lang": true, "lang_code": true, "u": true, "tif": true,
}

// guarded reports whether name is a content param that must never be stripped.
func guarded(name string) bool { return guardParams[name] }

// provider is one compiled ClearURLs rules entry for a host (or the global
// catch-all).
type provider struct {
	urlPattern *regexp.Regexp
	// hostTokens are the literal dotted tokens appearing in urlPattern, used
	// as a cheap pre-filter: a URL that shares none of them cannot match and is
	// skipped without a regex run.
	hostTokens   []string
	exceptions   []*regexp.Regexp
	redirections []*regexp.Regexp // expand a URL to its real target (e.g. google /url)
	paramNames   []*regexp.Regexp // matched against a single query-parameter name
	rawRules     []*regexp.Regexp // whole-URL rewrite, unanchored replace
	complete     bool             // completeProvider: strip every parameter on this domain
	isGlobal     bool             // urlPattern matches any host (built-in catch-all)
}

// compileRe compiles a ClearURLs regex. URL-matching patterns (host, exceptions,
// redirections, param names) are case-insensitive to mirror the reference
// engine's re.Match(..., re.IGNORECASE); raw URL rewrites stay case-sensitive
// (plain re.sub). A syntax error yields a nil regexp, so a single broken rule
// never kills the whole set.
func compileRe(pat string, caseSensitive bool) *regexp.Regexp {
	if !caseSensitive && !strings.HasPrefix(pat, "(?i)") {
		pat = "(?i)" + pat
	}
	re, err := regexp.Compile(pat)
	if err != nil {
		return nil
	}
	return re
}

func compileAll(pats []string, caseSensitive bool) []*regexp.Regexp {
	if len(pats) == 0 {
		return nil
	}
	out := make([]*regexp.Regexp, 0, len(pats))
	for _, pat := range pats {
		if re := compileRe(pat, caseSensitive); re != nil {
			out = append(out, re)
		}
	}
	return out
}

type rawProvider struct {
	URLPattern        string   `json:"urlPattern"`
	Rules             []string `json:"rules"`
	ReferralMarketing []string `json:"referralMarketing"`
	Exceptions        []string `json:"exceptions"`
	Redirections      []string `json:"redirections"`
	RawRules          []string `json:"rawRules"`
	CompleteProvider  bool     `json:"completeProvider"`
}

var (
	providersOnce sync.Once
	compiled      []*provider
)

// compiledProviders lazily parses the embedded ClearURLs data once. Safe for
// concurrent use; the result is treated as immutable.
func compiledProviders() []*provider {
	providersOnce.Do(func() { compiled = parseProviders(ruleDataJSON) })
	return compiled
}

func parseProviders(data string) []*provider {
	var raw struct {
		Providers map[string]rawProvider `json:"providers"`
	}
	if err := json.Unmarshal([]byte(data), &raw); err != nil {
		return nil
	}
	out := make([]*provider, 0, len(raw.Providers))
	for _, rp := range raw.Providers {
		if p := compileProvider(&rp); p != nil {
			out = append(out, p)
		}
	}
	return out
}

func compileProvider(rp *rawProvider) *provider {
	p := &provider{
		// A global catch-all is exactly a provider whose urlPattern matches any
		// host; classify on the payload rather than the map key so the check
		// survives upstream renames.
		isGlobal: rp.URLPattern == ".*",
		complete: rp.CompleteProvider,
	}
	p.urlPattern = compileRe(rp.URLPattern, false)
	if p.urlPattern == nil {
		return nil
	}
	p.hostTokens = hostTokens(rp.URLPattern)
	p.exceptions = compileAll(rp.Exceptions, false)
	p.redirections = compileAll(rp.Redirections, false)
	p.paramNames = compileAll(append(append([]string(nil), rp.Rules...), rp.ReferralMarketing...), false)
	p.rawRules = compileAll(rp.RawRules, true) // whole-URL rewrite stays case-sensitive
	return p
}

// hostTokens extracts the literal dotted tokens from a urlPattern, used as a
// pre-filter. A URL that contains none of them cannot match, so the provider
// is skipped without a regex run. Returning an empty slice disables filtering
// (keeps every URL the provider's regex would be run against).
func hostTokens(pattern string) []string {
	normalized := strings.NewReplacer(
		"\\.", ".",
		"\\-", "-",
	).Replace(pattern)
	var out []string
	for _, tok := range hostTokenRe.FindAllString(normalized, -1) {
		if strings.Contains(tok, ".") {
			out = append(out, tok)
		}
	}
	return out
}

var hostTokenRe = regexp.MustCompile(`[a-zA-Z0-9][a-zA-Z0-9.\-]{2,}`)

// anyMatch reports whether any regex in res matches s.
func anyMatch(res []*regexp.Regexp, s string) bool {
	for _, re := range res {
		if re.MatchString(s) {
			return true
		}
	}
	return false
}

// cleanURL applies the ClearURLs engine to a full URL: host-scoped matching,
// redirection expansion (bounded), query-parameter stripping, and raw URL
// rewrites. Guard content params are never stripped. Returns the (possibly
// different-host) cleaned URL string.
func cleanURL(rawURL string) string {
	cur := rawURL
	for depth := 0; depth <= maxRedirectDepth; depth++ {
		next := cleanOnce(cur)
		if next == cur {
			// A pass that changed nothing is a fixed point; the common
			// no-redirect case exits after one full sweep.
			return cur
		}
		cur = next
	}
	return cur
}

// cleanOnce runs every provider once against cur. A redirection match returns
// the rewritten target immediately (e.g. google /url → real host), and cleanURL
// re-runs until reaching a fixed point or exhausting maxRedirectDepth.
func cleanOnce(cur string) string {
	parsed, err := url.Parse(cur)
	if err != nil {
		return cur
	}
	for _, p := range compiledProviders() {
		// Cheap pre-filter: skip providers whose literal host tokens are all
		// absent from the URL, avoiding the regex run in the common case.
		if !p.mayMatch(cur) {
			continue
		}
		if anyMatch(p.exceptions, cur) {
			continue
		}
		// Redirection: expand to captured target and re-parse.
		if m := redirectionMatch(p, cur); m != "" && m != cur {
			if dec, derr := url.QueryUnescape(m); derr == nil {
				m = dec
			}
			return m // cleanURL's loop will re-run this new URL
		}
		// Parameter stripping.
		q := parsed.Query()
		for k := range q {
			if guarded(k) {
				continue // never strip content params
			}
			if p.complete || anyMatch(p.paramNames, k) {
				q.Del(k)
			}
		}
		parsed.RawQuery = q.Encode()
		cur = parsed.String()
		for _, r := range p.rawRules {
			cur = r.ReplaceAllString(cur, "")
		}
		parsed, _ = url.Parse(cur)
	}
	return cur
}

// mayMatch is the host-token pre-filter. An empty token set disables it.
func (p *provider) mayMatch(cur string) bool {
	if len(p.hostTokens) == 0 {
		return true
	}
	for _, tok := range p.hostTokens {
		if strings.Contains(cur, tok) {
			return true
		}
	}
	return false
}

func redirectionMatch(p *provider, cur string) string {
	for _, re := range p.redirections {
		if m := re.FindStringSubmatch(cur); len(m) > 1 {
			return m[1]
		}
	}
	return ""
}
