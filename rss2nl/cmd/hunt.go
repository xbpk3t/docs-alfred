package cmd

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/gosimple/slug"
	"github.com/mmcdole/gofeed"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
	"github.com/xbpk3t/docs-alfred/pkg/httputil"
	"github.com/xbpk3t/docs-alfred/pkg/urlutil"
	"github.com/xbpk3t/docs-alfred/service/rss"
)

// -- Types (matching TS schema) --

type huntProvider string

const (
	providerExa    huntProvider = "exa"
	providerTavily huntProvider = "tavily"
)

type candidateType string

const (
	candSource     candidateType = "source"
	candAuthor     candidateType = "author"
	candNewsletter candidateType = "newsletter"
	candRepo       candidateType = "repo"
	candUnknown    candidateType = "unknown"
)

type huntCandidate struct {
	FirstSeenAt   string        `json:"firstSeenAt,omitempty"`
	NormalizedURL string        `json:"normalizedUrl"`
	Category      string        `json:"category"`
	CandidateType candidateType `json:"candidateType"`
	Provider      huntProvider  `json:"provider"`
	Reason        string        `json:"reason,omitempty"`
	Title         string        `json:"title,omitempty"`
	SourceHint    string        `json:"sourceHint,omitempty"`
	Domain        string        `json:"domain"`
	URL           string        `json:"url"`
	EvidenceURLs  []string      `json:"evidenceUrls,omitempty"`
	SeenCount     int           `json:"seenCount"`
	Score         float64       `json:"score,omitempty"`
	Confidence    float64       `json:"confidence,omitempty"`
	IsNew         bool          `json:"isNew"`
}

type huntSeenRecord struct {
	FirstSeenAt string `json:"firstSeenAt"`
	LastSeenAt  string `json:"lastSeenAt"`
	Count       int    `json:"count"`
}

type huntMutedRecord struct {
	MutedAt string `json:"mutedAt"`
	Reason  string `json:"reason,omitempty"`
}

type huntState struct {
	Seen  map[string]huntSeenRecord  `json:"seen"`
	Muted map[string]huntMutedRecord `json:"muted,omitempty"`
}

type huntStats struct {
	CategoriesScanned  int `json:"categoriesScanned"`
	ProviderCalls      int `json:"providerCalls"`
	SuccessfulCalls    int `json:"successfulProviderCalls"`
	RawCandidates      int `json:"rawCandidates"`
	AcceptedCandidates int `json:"acceptedCandidates"`
	FilteredCandidates int `json:"filteredCandidates"`
}

type huntWarning struct {
	Provider huntProvider `json:"provider,omitempty"`
	Category string       `json:"category,omitempty"`
	Message  string       `json:"message"`
}

type huntFailure struct {
	Provider huntProvider `json:"provider"`
	Category string       `json:"category"`
	Message  string       `json:"message"`
}

type huntReport struct {
	GeneratedAt string          `json:"generatedAt"`
	Candidates  []huntCandidate `json:"candidates"`
	Warnings    []huntWarning   `json:"warnings"`
	Failures    []huntFailure   `json:"failures,omitempty"`
	Stats       huntStats       `json:"stats"`
	DryRun      bool            `json:"dryRun"`
}

// huntRunConfig holds resolved configuration after initialization.
type huntRunConfig struct {
	now             time.Time
	apiKeys         map[huntProvider]string
	report          *huntReport
	state           *huntState
	blockedSet      map[string]bool
	providerWeights map[string]float64
	typeWeights     map[string]float64
	providerNames   []huntProvider
	categories      []rss.FeedsDetail
	max             int
	perCat          int
	seedLimit       int
	newOnly         bool
	dryRun          bool
}

// -- Default blocklist --

var defaultBlockedDomains = []string{
	"facebook.com", "twitter.com", "x.com", "instagram.com",
	"linkedin.com", "reddit.com", "youtube.com", "tiktok.com",
	"pinterest.com", "snapchat.com", "tumblr.com", "whatsapp.com",
	"google.com", "bing.com", "duckduckgo.com", "baidu.com",
	"wikipedia.org", "amazon.com", "ebay.com",
}

// -- Command setup --

func newHuntCmd() *cobra.Command {
	var opts struct {
		reportHTML  string
		config      string
		providers   string
		reportMd    string
		state       string
		reportJSON  string
		category    []string
		blocked     []string
		max         int
		providerMax int
		seedLimit   int
		perCat      int
		newOnly     bool
		dryRun      bool
		sendMail    bool
	}

	cmd := &cobra.Command{
		Use:   "hunt",
		Short: "Discover high-quality source URLs",
		Long:  "Discover high-quality source URLs via Exa/Tavily providers and generate review reports.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHunt(&opts, os.Getenv("EXA_API_KEY"), os.Getenv("TAVILY_API_KEY"))
		},
	}

	cmd.Flags().StringVarP(&opts.config, "config", "c", "rss2nl.yml", "Config file path")
	cmd.Flags().StringVar(&opts.state, "state", fileutil.CachePath("rss2nl/hunt/feeds-hunt-state.json"), "State file path")
	cmd.Flags().StringArrayVar(&opts.category, "category", nil, "Category to scan")
	cmd.Flags().StringVar(&opts.providers, "providers", "", "Providers: exa,tavily")
	cmd.Flags().IntVar(&opts.max, "max", 0, "Global candidate cap")
	cmd.Flags().IntVar(&opts.perCat, "per-category", 0, "Candidate cap per category")
	cmd.Flags().IntVar(&opts.providerMax, "provider-max", 0, "Raw candidates per provider per category")
	cmd.Flags().IntVar(&opts.seedLimit, "seed-limit", 0, "Seed source cap per category")
	cmd.Flags().StringVar(&opts.reportMd, "report-md", fileutil.CachePath("rss2nl/hunt/feeds-hunt-report.md"), "Markdown report")
	cmd.Flags().StringVar(&opts.reportHTML, "report-html", fileutil.CachePath("rss2nl/hunt/feeds-hunt-report.html"), "HTML report")
	cmd.Flags().StringVar(&opts.reportJSON, "report-json", fileutil.CachePath("rss2nl/hunt/feeds-hunt-report.json"), "JSON report")
	cmd.Flags().StringArrayVar(&opts.blocked, "blocked-domain", nil, "Extra blocked domain")
	cmd.Flags().BoolVar(&opts.newOnly, "new-only", false, "Only accept candidates not in state")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "Write reports only")
	cmd.Flags().BoolVar(&opts.sendMail, "send-mail", false, "Send HTML report through Resend")

	return cmd
}

// initHuntRun loads config, state, and resolves all defaults.
func initHuntRun(opts *struct {
	reportHTML, config, providers, reportMd, state, reportJSON string
	category, blocked                                          []string
	max, providerMax, seedLimit, perCat                        int
	newOnly, dryRun, sendMail                                  bool
},
	exaAPIKey, tavilyAPIKey string,
) (*huntRunConfig, error) {
	cfg, err := rss.NewConfig(opts.config)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	now := time.Now().UTC()
	report := &huntReport{
		GeneratedAt: now.Format(time.RFC3339),
		DryRun:      opts.dryRun,
	}

	state := loadHuntState(opts.state)

	providerNames := parseHuntProviders(opts.providers)
	if len(providerNames) == 0 {
		providerNames = []huntProvider{providerExa, providerTavily}
	}

	// Apply categories.except filter
	categories := filterCategories(cfg, opts.category)
	report.Stats.CategoriesScanned = len(categories)

	// Build blocked set
	blockedSet := buildBlockedSet(defaultBlockedDomains, cfg.HuntConfig.BlockedDomains, opts.blocked)

	// Provider weights
	providerWeights := cfg.HuntConfig.ProviderWeights
	if providerWeights == nil {
		providerWeights = map[string]float64{"exa": 1.0, "tavily": 0.9}
	}
	typeWeights := cfg.HuntConfig.TypeWeights
	if typeWeights == nil {
		typeWeights = map[string]float64{
			string(candSource): 1.0, string(candAuthor): 0.9,
			string(candNewsletter): 0.8, string(candRepo): 0.7,
			string(candUnknown): 0.5,
		}
	}

	apiKeys := map[huntProvider]string{
		providerExa:    exaAPIKey,
		providerTavily: tavilyAPIKey,
	}

	// Apply config defaults when CLI flags are not set (0 = unset).
	maxVal := opts.max
	if maxVal == 0 {
		maxVal = cfg.HuntConfig.DefaultMax
	}
	perCatVal := opts.perCat
	if perCatVal == 0 {
		perCatVal = cfg.HuntConfig.DefaultPerCat
	}
	seedLimitVal := opts.seedLimit
	if seedLimitVal == 0 {
		seedLimitVal = cfg.HuntConfig.DefaultSeed
	}

	return &huntRunConfig{
		categories:      categories,
		report:          report,
		state:           state,
		providerNames:   providerNames,
		blockedSet:      blockedSet,
		providerWeights: providerWeights,
		typeWeights:     typeWeights,
		apiKeys:         apiKeys,
		max:             maxVal,
		perCat:          perCatVal,
		seedLimit:       seedLimitVal,
		newOnly:         opts.newOnly,
		dryRun:          opts.dryRun,
		now:             now,
	}, nil
}

func runHunt(opts *struct {
	reportHTML, config, providers, reportMd, state, reportJSON string
	category, blocked                                          []string
	max, providerMax, seedLimit, perCat                        int
	newOnly, dryRun, sendMail                                  bool
},
	exaAPIKey, tavilyAPIKey string,
) error {
	hc, err := initHuntRun(opts, exaAPIKey, tavilyAPIKey)
	if err != nil {
		return err
	}

	for _, category := range hc.categories {
		processCategory(category, hc.providerNames, hc.seedLimit, hc.perCat, opts.providerMax,
			hc.newOnly, hc.report, hc.state, hc.blockedSet, hc.now, hc.providerWeights, hc.typeWeights, hc.apiKeys)
	}

	sortCandidatesByScore(hc.report.Candidates)

	if hc.max > 0 && len(hc.report.Candidates) > hc.max {
		hc.report.Candidates = hc.report.Candidates[:hc.max]
		hc.report.Stats.AcceptedCandidates = len(hc.report.Candidates)
	}

	writeHuntReports(hc.report, opts.reportMd, opts.reportHTML, opts.reportJSON)
	if !hc.dryRun {
		saveHuntState(opts.state, hc.state)
	}

	slog.Info("Hunt complete",
		"candidates", hc.report.Stats.AcceptedCandidates,
		"categories", hc.report.Stats.CategoriesScanned,
	)

	return nil
}

// -- Blocked set --

func buildBlockedSet(defaults, configBlocked, flagBlocked []string) map[string]bool {
	domains := slices.Concat(defaults, configBlocked, flagBlocked)
	domains = lo.FilterMap(domains, func(domain string, _ int) (string, bool) {
		domain = strings.ToLower(strings.TrimSpace(domain))

		return domain, domain != ""
	})

	return lo.SliceToMap(domains, func(domain string) (string, bool) {
		return domain, true
	})
}

// -- Category processing --

func filterCategories(cfg *rss.Config, categoryFilter []string) []rss.FeedsDetail {
	categories := cfg.RSS
	except := buildExceptSet(cfg)

	if len(categoryFilter) > 0 {
		return filterByCLI(categories, categoryFilter)
	}

	if len(except) > 0 {
		return filterByExcept(categories, except)
	}

	return categories
}

func buildExceptSet(cfg *rss.Config) map[string]bool {
	except := make(map[string]bool)
	if cfg.HuntConfig.Categories != nil {
		for _, e := range cfg.HuntConfig.Categories.Except {
			except[e] = true
		}
	}

	return except
}

func filterByCLI(categories []rss.FeedsDetail, categoryFilter []string) []rss.FeedsDetail {
	filters := lo.FlatMap(categoryFilter, func(category string, _ int) []string {
		return strings.Split(category, ",")
	})
	catMap := lo.SliceToMap(lo.FilterMap(filters, func(category string, _ int) (string, bool) {
		category = strings.TrimSpace(category)

		return category, category != ""
	}), func(category string) (string, bool) {
		return category, true
	})

	return lo.Filter(categories, func(feed rss.FeedsDetail, _ int) bool {
		return catMap[feed.Type]
	})
}

func filterByExcept(categories []rss.FeedsDetail, except map[string]bool) []rss.FeedsDetail {
	return lo.Filter(categories, func(feed rss.FeedsDetail, _ int) bool {
		return !except[feed.Type]
	})
}

func processCategory(
	category rss.FeedsDetail,
	providerNames []huntProvider,
	seedLimit, perCat, providerMax int,
	newOnly bool,
	report *huntReport,
	state *huntState,
	blockedSet map[string]bool,
	now time.Time,
	providerWeights, typeWeights map[string]float64,
	apiKeys map[huntProvider]string,
) {
	slog.Info("Scanning category", "type", category.Type)

	if seedLimit <= 0 {
		seedLimit = 10
	}
	seedURLs, seedDescs := buildSeeds(category, seedLimit)
	if len(seedURLs) == 0 {
		return
	}

	if perCat <= 0 {
		perCat = 10
	}
	if providerMax <= 0 {
		providerMax = perCat * 2
	}

	recentTopics := enrichTopics(category.Feeds)

	for _, provider := range providerNames {
		processCategoryProvider(provider, category.Type, seedURLs, seedDescs, providerMax,
			recentTopics, report, state, blockedSet, newOnly, now, providerWeights, typeWeights, apiKeys)
	}
}

//nolint:nonamedreturns
func buildSeeds(category rss.FeedsDetail, seedLimit int) (seedURLs, seedDescs []string) {
	for _, u := range category.Feeds {
		if u.URL != "" {
			seedURLs = append(seedURLs, u.URL)
		} else if u.Feed != "" {
			seedURLs = append(seedURLs, u.Feed)
		}
		if u.Des != "" {
			seedDescs = append(seedDescs, u.Des)
		}
	}
	if len(seedURLs) > seedLimit {
		seedURLs = seedURLs[:seedLimit]
	}

	return
}

func processCategoryProvider(
	provider huntProvider,
	categoryType string,
	seedURLs, seedDescs []string,
	providerMax int,
	recentTopics []string,
	report *huntReport,
	state *huntState,
	blockedSet map[string]bool,
	newOnly bool,
	now time.Time,
	providerWeights, typeWeights map[string]float64,
	apiKeys map[huntProvider]string,
) {
	candidates := discoverWithProvider(provider, categoryType, seedURLs, seedDescs, providerMax, recentTopics, apiKeys[provider])
	report.Stats.RawCandidates += len(candidates)
	if len(candidates) > 0 {
		report.Stats.SuccessfulCalls++
	}
	report.Stats.ProviderCalls++

	before := len(report.Candidates)
	processCandidates(candidates, provider, categoryType, report, state, blockedSet, newOnly, now, providerWeights, typeWeights)
	report.Stats.FilteredCandidates += len(candidates) - (len(report.Candidates) - before)
}

func enrichTopics(urls []rss.Feeds) []string {
	var topics []string
	for _, u := range urls {
		if u.Feed == "" {
			continue
		}
		fp := gofeed.NewParser()
		fp.Client = &http.Client{Timeout: 10 * time.Second}
		parsed, err := fp.ParseURL(u.Feed)
		if err != nil {
			continue
		}
		for i, item := range parsed.Items {
			if i >= 3 {
				break
			}
			if item.Title != "" {
				topics = append(topics, item.Title)
			}
		}
	}
	if len(topics) > 5 {
		topics = topics[:5]
	}

	return topics
}

func processCandidates(
	candidates []huntCandidate,
	provider huntProvider,
	category string,
	report *huntReport,
	state *huntState,
	blockedSet map[string]bool,
	newOnly bool,
	now time.Time,
	providerWeights, typeWeights map[string]float64,
) {
	for ci := range candidates {
		c := &candidates[ci]
		c.NormalizedURL = normalizeURL(c.URL)
		c.Domain = extractDomain(c.URL)

		if blockedSet[c.Domain] || isBlocked(c.Domain, blockedSet) {
			continue
		}

		if _, muted := state.Muted[c.NormalizedURL]; muted {
			continue
		}
		if _, muted := state.Muted[c.Domain]; muted {
			continue
		}

		if !processCandidateSeen(c, state, newOnly, now) {
			continue
		}

		c.Score = computeCandidateScore(c, providerWeights, typeWeights)

		report.Candidates = append(report.Candidates, *c)
		report.Stats.AcceptedCandidates++
	}
}

func processCandidateSeen(c *huntCandidate, state *huntState, newOnly bool, now time.Time) bool {
	if rec, ok := state.Seen[c.NormalizedURL]; ok {
		c.IsNew = false
		c.SeenCount = rec.Count
		c.FirstSeenAt = rec.FirstSeenAt
		if newOnly {
			return false
		}
	} else {
		c.IsNew = true
		if state.Seen == nil {
			state.Seen = make(map[string]huntSeenRecord)
		}
		state.Seen[c.NormalizedURL] = huntSeenRecord{
			FirstSeenAt: now.Format(time.RFC3339),
			LastSeenAt:  now.Format(time.RFC3339),
			Count:       1,
		}
	}

	return true
}

func computeCandidateScore(c *huntCandidate, providerWeights, typeWeights map[string]float64) float64 {
	pw := providerWeights[string(c.Provider)]
	if pw == 0 {
		pw = 0.8
	}
	tw := typeWeights[string(c.CandidateType)]
	if tw == 0 {
		tw = 0.5
	}
	confidence := c.Confidence
	if confidence <= 0 {
		confidence = 0.5
	}

	return pw * tw * confidence
}

func sortCandidatesByScore(candidates []huntCandidate) {
	slices.SortStableFunc(candidates, func(a, b huntCandidate) int {
		if a.Score > b.Score {
			return -1
		}
		if a.Score < b.Score {
			return 1
		}

		return 0
	})
}

// -- Provider implementations --

func discoverWithProvider(
	provider huntProvider, category string,
	seedURLs, seedDescs []string, maxResults int, recentTopics []string,
	apiKey string,
) []huntCandidate {
	switch provider {
	case providerExa:

		return discoverExa(category, seedURLs, seedDescs, maxResults, recentTopics, apiKey)
	case providerTavily:

		return discoverTavily(category, seedURLs, seedDescs, maxResults, recentTopics, apiKey)
	}

	return nil
}

func discoverExa(category string, seedURLs, seedDescs []string, maxResults int, recentTopics []string, apiKey string) []huntCandidate {
	if apiKey == "" {
		return nil
	}

	query := buildDiscoveryQuery(category, seedURLs, seedDescs, recentTopics)

	payload := map[string]any{
		"query":      query,
		"type":       "neural",
		"numResults": maxResults,
		"contents": map[string]any{
			"text":       true,
			"highlights": true,
		},
	}

	var result struct {
		Results []struct {
			URL        string   `json:"url"`
			Title      string   `json:"title"`
			Text       string   `json:"text"`
			Highlights []string `json:"highlights"`
			Score      float64  `json:"score"`
		} `json:"results"`
	}
	// Exa uses x-api-key header (not Authorization: Bearer)
	if _, err := httputil.PostJSONWithResult(context.Background(), "https://api.exa.ai/search", payload, &result, httputil.RequestOptions{
		Headers: map[string]string{"x-api-key": apiKey},
	}); err != nil || len(result.Results) == 0 {
		return nil
	}

	var candidates []huntCandidate
	for _, r := range result.Results {
		summary := ""
		if len(r.Highlights) > 0 {
			summary = r.Highlights[0]
		} else if r.Text != "" {
			summary = r.Text
		}
		candidates = append(candidates, huntCandidate{
			URL:           r.URL,
			Title:         r.Title,
			Category:      category,
			CandidateType: classifyCandidate(r.URL, r.Title),
			Provider:      providerExa,
			Reason:        buildReason(category, seedDescs, "Exa", summary),
			EvidenceURLs:  seedURLs[:min(3, len(seedURLs))],
			Confidence:    normalizeConfidence(r.Score, 0.72),
		})
	}

	return candidates
}

func discoverTavily(category string, seedURLs, seedDescs []string, maxResults int, recentTopics []string, apiKey string) []huntCandidate {
	if apiKey == "" {
		return nil
	}

	query := buildTavilyQuery(category, seedURLs, recentTopics)

	payload := map[string]any{
		"api_key":             apiKey, // Tavily: api_key in JSON body
		"query":               query,
		"search_depth":        "advanced",
		"max_results":         maxResults,
		"include_answer":      false,
		"include_raw_content": false,
	}

	var result struct {
		Results []struct {
			URL     string  `json:"url"`
			Title   string  `json:"title"`
			Content string  `json:"content"`
			Score   float64 `json:"score"`
		} `json:"results"`
	}
	if _, err := httputil.PostJSONWithResult(
		context.Background(),
		"https://api.tavily.com/search",
		payload,
		&result,
		httputil.RequestOptions{},
	); err != nil || len(result.Results) == 0 {
		return nil
	}

	var candidates []huntCandidate
	for _, r := range result.Results {
		candidates = append(candidates, huntCandidate{
			URL:           r.URL,
			Title:         r.Title,
			Category:      category,
			CandidateType: classifyCandidate(r.URL, r.Title),
			Provider:      providerTavily,
			Reason:        buildReason(category, seedDescs, "Tavily", r.Content),
			EvidenceURLs:  seedURLs[:min(3, len(seedURLs))],
			Confidence:    normalizeConfidence(r.Score, 0.62),
		})
	}

	return candidates
}

func buildDiscoveryQuery(category string, seedURLs, seedDescs, recentTopics []string) string {
	parts := []string{
		fmt.Sprintf("Find high-quality technical source websites for category %s.", category),
		"Prefer original engineering blogs, author homepages, newsletters, open source project homepages, and deep technical publications.",
		"Return source or homepage URLs, not RSS feed URLs and not individual article URLs.",
	}

	if len(seedURLs) > 0 {
		urls := seedURLs[:min(8, len(seedURLs))]
		parts = append(parts, fmt.Sprintf("Known good seeds: %s.", strings.Join(urls, ", ")))
	}
	if len(seedDescs) > 0 {
		descs := seedDescs[:min(8, len(seedDescs))]
		parts = append(parts, fmt.Sprintf("Seed notes: %s.", strings.Join(descs, "; ")))
	}
	if len(recentTopics) > 0 {
		parts = append(parts, fmt.Sprintf("Recent topics from these sources: %s.", strings.Join(recentTopics, "; ")))
	}
	parts = append(parts, "Avoid SEO content farms, social feeds, generic marketing pages, and already-known domains.")

	return strings.Join(parts, " ")
}

func buildTavilyQuery(category string, seedURLs, recentTopics []string) string {
	maxLen := 400
	query := fmt.Sprintf("Find high-quality technical source/homepage URLs for %s. "+
		"Prefer engineering blogs, authors, newsletters, project homepages. "+
		"Avoid RSS feeds, individual articles, SEO farms, social feeds, marketing pages, known domains.", category)

	for _, seedURL := range seedURLs {
		var suffix string
		if strings.Contains(query, " Seeds: ") {
			suffix = ", " + seedURL
		} else {
			suffix = " Seeds: " + seedURL
		}
		if len(query)+len(suffix) > maxLen {
			break
		}
		query += suffix
	}

	for _, topic := range recentTopics {
		var suffix string
		if strings.Contains(query, " Recent: ") {
			suffix = "; " + topic
		} else {
			suffix = " Recent: " + topic
		}
		if len(query)+len(suffix) > maxLen {
			break
		}
		query += suffix
	}

	return trimToMaxLength(query, maxLen)
}

func trimToMaxLength(s string, maxLen int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= maxLen {
		return s
	}

	return s[:max(maxLen-3, 0)] + "..."
}

// -- Candidate classification (matching TS provider-utils.ts) --

func classifyCandidate(rawURL, title string) candidateType {
	u := strings.ToLower(rawURL)
	domain := extractDomain(rawURL)
	lowerTitle := strings.ToLower(title)

	if isRepoURL(u) {
		return candRepo
	}
	if isNewsletterDomain(domain, lowerTitle) {
		return candNewsletter
	}
	if isAuthorDomain(domain, lowerTitle) {
		return candAuthor
	}
	if domain != "" {
		return candSource
	}

	return candUnknown
}

func isRepoURL(u string) bool {
	return urlutil.IsSourceRepo(u)
}

func isNewsletterDomain(domain, lowerTitle string) bool {
	return strings.Contains(domain, "substack.com") ||
		strings.Contains(domain, "buttondown.email") ||
		strings.Contains(lowerTitle, "newsletter")
}

func isAuthorDomain(domain, lowerTitle string) bool {
	return strings.Contains(lowerTitle, "blog") ||
		(len(strings.Split(domain, ".")) == 2 && !strings.HasSuffix(domain, ".com.cn"))
}

func buildReason(category string, seedDescs []string, providerName, summary string) string {
	sourceReason := fmt.Sprintf("Similar to existing %s seeds and returned by %s.", category, providerName)
	if summary == "" {
		return sourceReason
	}

	shortSummary := strings.Join(strings.Fields(summary), " ")
	if len(shortSummary) > 220 {
		shortSummary = shortSummary[:220]
	}

	return fmt.Sprintf("%s %s", sourceReason, shortSummary)
}

func normalizeConfidence(score, fallback float64) float64 {
	if score == 0 || score != score { // NaN check
		return fallback
	}
	if score >= 0 && score <= 1 {
		return score
	}

	return min(max(score/10, 0), 1)
}

// -- URL helpers --

// normalizeURL canonicalizes a URL using purell (lowercase scheme/host,
// remove default port, normalize path, strip fragment).
func normalizeURL(rawURL string) string {
	return urlutil.Normalize(rawURL)
}

func extractDomain(rawURL string) string {
	return urlutil.Domain(rawURL)
}

// isBlocked checks if a domain matches any blocked entry by registrable domain.
func isBlocked(domain string, blockedSet map[string]bool) bool {
	return urlutil.DomainBlocked(domain, blockedSet)
}

// -- State persistence --

func loadHuntState(path string) *huntState {
	state, err := fileutil.ReadJSONFile[huntState](path)
	if err != nil {
		return &huntState{Seen: make(map[string]huntSeenRecord)}
	}
	if state.Seen == nil {
		state.Seen = make(map[string]huntSeenRecord)
	}
	if state.Muted == nil {
		state.Muted = make(map[string]huntMutedRecord)
	}

	return &state
}

func saveHuntState(path string, state *huntState) {
	_ = fileutil.AtomicWriteJSONFile(path, state, fileutil.FilePermPrivate)
}

// -- Report generation --

func writeHuntReports(report *huntReport, mdPath, htmlPath, jsonPath string) {
	jsonData, err := fileutil.MarshalJSON(report)
	if err != nil {
		slog.Warn("Failed to marshal JSON report", "error", err)

		return
	}
	_ = fileutil.AtomicWriteFile(jsonPath, jsonData, fileutil.FilePermPrivate)

	mdContent := renderHuntMarkdown(report)
	_ = fileutil.AtomicWriteFile(mdPath, []byte(mdContent), fileutil.FilePermPrivate)

	htmlContent := renderHuntHTML(report)
	_ = fileutil.AtomicWriteFile(htmlPath, []byte(htmlContent), fileutil.FilePermPrivate)

	fmt.Fprintln(os.Stdout, mdContent) //nolint:errcheck
}

func renderHuntMarkdown(report *huntReport) string {
	var b strings.Builder
	b.WriteString("# Source Discovery Report\n\n")
	fmt.Fprintf(&b, "**Generated:** %s\n", report.GeneratedAt)
	fmt.Fprintf(&b, "**Categories:** %d\n", report.Stats.CategoriesScanned)
	fmt.Fprintf(&b, "**Candidates:** %d\n\n", report.Stats.AcceptedCandidates)

	for _, w := range report.Warnings {
		fmt.Fprintf(&b, "> ⚠ %s\n\n", w.Message)
	}

	for ci := range report.Candidates {
		c := report.Candidates[ci]
		scoreStr := fmt.Sprintf("%.1f%%", c.Score*100)
		if c.IsNew {
			scoreStr = "**NEW** " + scoreStr
		}
		fmt.Fprintf(&b, "- [%s](%s) (%s, %s, %s)\n",
			c.Title, c.URL, c.Category, c.Provider, scoreStr)
		if c.Reason != "" {
			fmt.Fprintf(&b, "  - %s\n", c.Reason)
		}
	}

	return b.String()
}

// -- Hunt report HTML view types (matching hunt-report.gohtml) --

type huntReportView struct {
	Title                 string
	GeneratedAt           string
	Mode                  string
	Stats                 []huntStatView
	Warnings              []huntWarningView
	Failures              []huntFailureView
	Categories            []huntCategoryView
	HasWarnings           bool
	HasFailures           bool
	HasMultipleCategories bool
	HasNoCandidates       bool
}

type huntStatView struct {
	Value any
	Label string
}

type huntWarningView struct {
	Message string
}

type huntFailureView struct {
	Message string
}

type huntCategoryView struct {
	Name       string
	ID         string
	Candidates []huntCandidateView
	Count      int
}

type huntCandidateView struct {
	Title         string
	URL           string
	Status        string
	Reason        string
	Provider      string
	CandidateType string
	Domain        string
	Confidence    string
	EvidenceURLs  []string
	HasEvidence   bool
}

func buildHuntReportView(report *huntReport) huntReportView {
	categories := groupCandidatesByCategory(report.Candidates)

	return huntReportView{
		Title:       "Source Discovery " + report.GeneratedAt[:10],
		GeneratedAt: report.GeneratedAt,
		Mode:        huntReportMode(report.DryRun),
		Stats: []huntStatView{
			{Label: "Accepted", Value: report.Stats.AcceptedCandidates},
			{Label: "Categories", Value: report.Stats.CategoriesScanned},
			{Label: "Provider calls", Value: report.Stats.ProviderCalls},
			{Label: "Successful calls", Value: report.Stats.SuccessfulCalls},
			{Label: "Raw candidates", Value: report.Stats.RawCandidates},
			{Label: "Filtered", Value: report.Stats.FilteredCandidates},
		},
		HasWarnings:           len(report.Warnings) > 0,
		Warnings:              buildWarningViews(report.Warnings),
		HasFailures:           len(report.Failures) > 0,
		Failures:              buildFailureViews(report.Failures),
		HasMultipleCategories: len(categories) > 1,
		HasNoCandidates:       len(categories) == 0,
		Categories:            categories,
	}
}

func buildWarningViews(warnings []huntWarning) []huntWarningView {
	views := make([]huntWarningView, len(warnings))
	for i, w := range warnings {
		parts := []string{}
		if w.Provider != "" {
			parts = append(parts, string(w.Provider))
		}
		if w.Category != "" {
			parts = append(parts, w.Category)
		}
		msg := w.Message
		if len(parts) > 0 {
			msg = strings.Join(parts, "/") + ": " + msg
		}
		views[i] = huntWarningView{Message: msg}
	}

	return views
}

func buildFailureViews(failures []huntFailure) []huntFailureView {
	views := make([]huntFailureView, len(failures))
	for i, f := range failures {
		views[i] = huntFailureView{
			Message: fmt.Sprintf("%s/%s: %s", f.Provider, f.Category, f.Message),
		}
	}

	return views
}

func groupCandidatesByCategory(candidates []huntCandidate) []huntCategoryView {
	groups := lo.GroupBy(candidates, func(candidate huntCandidate) string {
		return candidate.Category
	})

	// Sort category names for stable output
	names := lo.Keys(groups)
	slices.Sort(names)

	views := make([]huntCategoryView, 0, len(names))
	for _, name := range names {
		group := groups[name]
		candidateViews := make([]huntCandidateView, len(group))
		for i := range group {
			candidateViews[i] = buildCandidateView(&group[i])
		}
		views = append(views, huntCategoryView{
			Name:       name,
			ID:         categoryID(name),
			Count:      len(group),
			Candidates: candidateViews,
		})
	}

	return views
}

func buildCandidateView(c *huntCandidate) huntCandidateView {
	title := c.Title
	if title == "" {
		title = c.NormalizedURL
	}

	reason := c.Reason
	if reason == "" {
		reason = "No reason provided."
	}

	return huntCandidateView{
		Title:         title,
		URL:           c.NormalizedURL,
		Status:        candidateStatus(c),
		Reason:        reason,
		HasEvidence:   len(c.EvidenceURLs) > 0,
		EvidenceURLs:  c.EvidenceURLs,
		Provider:      string(c.Provider),
		CandidateType: string(c.CandidateType),
		Domain:        c.Domain,
		Confidence:    formatConfidence(c.Confidence),
	}
}

func candidateStatus(c *huntCandidate) string {
	if c.IsNew {
		return "new"
	}

	return fmt.Sprintf("seen %dx", c.SeenCount)
}

func formatConfidence(f float64) string {
	if f <= 0 {
		return ""
	}

	return fmt.Sprintf("%.2f", f)
}

func categoryID(name string) string {
	s := slug.Make(name)
	if s == "" {
		s = "unknown"
	}

	return "category-" + s
}

func huntReportMode(dryRun bool) string {
	if dryRun {
		return "dry-run"
	}

	return "stateful"
}

func renderHuntHTML(report *huntReport) string {
	view := buildHuntReportView(report)

	funcMap := template.FuncMap{}

	tmpl, err := template.New("hunt-report.gohtml").Funcs(funcMap).ParseFS(templates, "templates/hunt-report.gohtml")
	if err != nil {
		slog.Warn("Failed to parse hunt HTML template", "error", err)

		return fmt.Sprintf("<pre>%s</pre>", renderHuntMarkdown(report))
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, view); err != nil {
		slog.Warn("Failed to render hunt HTML", "error", err)

		return fmt.Sprintf("<pre>%s</pre>", renderHuntMarkdown(report))
	}

	return buf.String()
}

// -- Parsing --

func parseHuntProviders(value string) []huntProvider {
	if value == "" {
		return nil
	}
	var providers []huntProvider
	for p := range strings.SplitSeq(value, ",") {
		p = strings.TrimSpace(p)
		switch huntProvider(p) {
		case providerExa, providerTavily:
			providers = append(providers, huntProvider(p))
		}
	}

	return providers
}
