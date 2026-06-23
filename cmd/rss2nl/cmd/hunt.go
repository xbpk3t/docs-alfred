package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/internal/rss/feed"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
	"github.com/xbpk3t/docs-alfred/pkg/httputil"
	"github.com/xbpk3t/docs-alfred/pkg/litter"
	"github.com/xbpk3t/docs-alfred/pkg/md"
	"github.com/xbpk3t/docs-alfred/pkg/urlutil"
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
	publish         rss.HuntPublishConfig
	max             int
	perCat          int
	seedLimit       int
	providerMax     int
	newOnly         bool
	dryRun          bool
}

// ApiKey returns the API key for the given provider.
func (hc *huntRunConfig) ApiKey(provider huntProvider) string {
	return hc.apiKeys[provider]
}

// huntCategoryContext holds per-category ephemeral state passed through the processing pipeline.
type huntCategoryContext struct {
	categoryType string
	seedURLs     []string
	seedDescs    []string
	recentTopics []string
	providerMax  int
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
		providerNames = parseHuntProviders(strings.Join(cfg.HuntConfig.Providers, ","))
	}
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
		publish:         cfg.HuntConfig.Publish,
		max:             maxVal,
		perCat:          perCatVal,
		seedLimit:       seedLimitVal,
		providerMax:     opts.providerMax,
		newOnly:         opts.newOnly || cfg.HuntConfig.NewOnly,
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
		processCategory(category, hc)
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

	// Publish HTML report to a temporary file host if configured.
	publishHuntReport(hc, opts.reportHTML)

	return nil
}

func publishHuntReport(hc *huntRunConfig, reportHTML string) {
	if !hc.publish.Enabled || hc.report.Stats.AcceptedCandidates <= 0 {
		return
	}

	htmlContent, err := os.ReadFile(reportHTML)
	if err != nil {
		slog.Warn("Failed to read HTML report for publish", "error", err)

		return
	}

	uploader := litter.NewFromNames(hc.publish.Drivers, hc.publish.Expiration)
	filename := fmt.Sprintf("feeds-hunt-report-%s.html", hc.report.GeneratedAt[:10])
	result, err := uploader.Upload(context.Background(), filename, string(htmlContent))
	if err != nil {
		slog.Warn("Failed to publish hunt report", "error", err)

		return
	}

	slog.Info("Hunt report published", "driver", result.Driver, "url", result.URL)

	// Write directly to $GITHUB_ENV so the workflow doesn't need bash parsing.
	if ghEnv := os.Getenv("GITHUB_ENV"); ghEnv != "" {
		count := hc.report.Stats.AcceptedCandidates
		entry := fmt.Sprintf("SOURCE_DISCOVERY_URL=%s\nSOURCE_DISCOVERY_COUNT=%d\n", result.URL, count)
		if f, ferr := os.OpenFile(ghEnv, os.O_APPEND|os.O_WRONLY, 0600); ferr == nil { //nolint:gosec // GITHUB_ENV is set by Actions runner
			fmt.Fprint(f, entry) //nolint:errcheck
			_ = f.Close()
		}
	}

	fmt.Fprintln(os.Stdout, result.URL) //nolint:errcheck
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

func processCategory(category rss.FeedsDetail, hc *huntRunConfig) {
	slog.Info("Scanning category", "type", category.Type)

	seedLimit := hc.seedLimit
	if seedLimit <= 0 {
		seedLimit = 10
	}
	seedURLs, seedDescs := buildSeeds(category, seedLimit)
	if len(seedURLs) == 0 {
		return
	}

	perCat := hc.perCat
	if perCat <= 0 {
		perCat = 10
	}
	providerMax := hc.providerMax
	if providerMax <= 0 {
		providerMax = perCat * 2
	}

	catCtx := huntCategoryContext{
		categoryType: category.Type,
		seedURLs:     seedURLs,
		seedDescs:    seedDescs,
		recentTopics: enrichTopics(category.Feeds),
		providerMax:  providerMax,
	}

	for _, provider := range hc.providerNames {
		processCategoryProvider(provider, &catCtx, hc)
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

func processCategoryProvider(provider huntProvider, catCtx *huntCategoryContext, hc *huntRunConfig) {
	candidates := discoverWithProvider(provider, catCtx, hc.ApiKey(provider))
	hc.report.Stats.RawCandidates += len(candidates)
	if len(candidates) > 0 {
		hc.report.Stats.SuccessfulCalls++
	}
	hc.report.Stats.ProviderCalls++

	before := len(hc.report.Candidates)
	processCandidates(candidates, provider, catCtx.categoryType, hc)
	hc.report.Stats.FilteredCandidates += len(candidates) - (len(hc.report.Candidates) - before)
}

func enrichTopics(urls []rss.Feeds) []string {
	var topics []string
	for _, u := range urls {
		if u.Feed == "" {
			continue
		}
		fp := gofeed.NewParser()
		fp.Client = httputil.StdHTTPClient(10 * time.Second)
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

func processCandidates(candidates []huntCandidate, provider huntProvider, category string, hc *huntRunConfig) {
	for ci := range candidates {
		c := &candidates[ci]
		c.NormalizedURL = normalizeURL(c.URL)
		c.Domain = extractDomain(c.URL)

		if hc.blockedSet[c.Domain] || isBlocked(c.Domain, hc.blockedSet) {
			continue
		}

		if _, muted := hc.state.Muted[c.NormalizedURL]; muted {
			continue
		}
		if _, muted := hc.state.Muted[c.Domain]; muted {
			continue
		}

		if !processCandidateSeen(c, hc) {
			continue
		}

		c.Score = computeCandidateScore(c, hc)

		hc.report.Candidates = append(hc.report.Candidates, *c)
		hc.report.Stats.AcceptedCandidates++
	}
}

func processCandidateSeen(c *huntCandidate, hc *huntRunConfig) bool {
	if rec, ok := hc.state.Seen[c.NormalizedURL]; ok {
		c.IsNew = false
		c.SeenCount = rec.Count
		c.FirstSeenAt = rec.FirstSeenAt
		if hc.newOnly {
			return false
		}
	} else {
		c.IsNew = true
		if hc.state.Seen == nil {
			hc.state.Seen = make(map[string]huntSeenRecord)
		}
		hc.state.Seen[c.NormalizedURL] = huntSeenRecord{
			FirstSeenAt: hc.now.Format(time.RFC3339),
			LastSeenAt:  hc.now.Format(time.RFC3339),
			Count:       1,
		}
	}

	return true
}

func computeCandidateScore(c *huntCandidate, hc *huntRunConfig) float64 {
	pw := hc.providerWeights[string(c.Provider)]
	if pw == 0 {
		pw = 0.8
	}
	tw := hc.typeWeights[string(c.CandidateType)]
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

func discoverWithProvider(provider huntProvider, catCtx *huntCategoryContext, apiKey string) []huntCandidate {
	switch provider {
	case providerExa:

		return discoverExa(catCtx, apiKey)
	case providerTavily:

		return discoverTavily(catCtx, apiKey)
	}

	return nil
}

func discoverExa(catCtx *huntCategoryContext, apiKey string) []huntCandidate {
	if apiKey == "" {
		return nil
	}

	query := buildDiscoveryQuery(catCtx.categoryType, catCtx.seedURLs, catCtx.seedDescs, catCtx.recentTopics)

	payload := map[string]any{
		"query":      query,
		"type":       "neural",
		"numResults": catCtx.providerMax,
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
			Category:      catCtx.categoryType,
			CandidateType: classifyCandidate(r.URL, r.Title),
			Provider:      providerExa,
			Reason:        buildReason(catCtx.categoryType, catCtx.seedDescs, "Exa", summary),
			EvidenceURLs:  catCtx.seedURLs[:min(3, len(catCtx.seedURLs))],
			Confidence:    normalizeConfidence(r.Score, 0.72),
		})
	}

	return candidates
}

func discoverTavily(catCtx *huntCategoryContext, apiKey string) []huntCandidate {
	if apiKey == "" {
		return nil
	}

	query := buildTavilyQuery(catCtx.categoryType, catCtx.seedURLs, catCtx.recentTopics)

	payload := map[string]any{
		"api_key":             apiKey, // Tavily: api_key in JSON body
		"query":               query,
		"search_depth":        "advanced",
		"max_results":         catCtx.providerMax,
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
			Category:      catCtx.categoryType,
			CandidateType: classifyCandidate(r.URL, r.Title),
			Provider:      providerTavily,
			Reason:        buildReason(catCtx.categoryType, catCtx.seedDescs, "Tavily", r.Content),
			EvidenceURLs:  catCtx.seedURLs[:min(3, len(catCtx.seedURLs))],
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
	if err := fileutil.AtomicWriteFile(jsonPath, jsonData, fileutil.FilePermPrivate); err != nil {
		slog.Warn("Failed to write JSON report", "path", jsonPath, "error", err)
	}

	mdContent := renderHuntMarkdown(report)
	if err := fileutil.AtomicWriteFile(mdPath, []byte(mdContent), fileutil.FilePermPrivate); err != nil {
		slog.Warn("Failed to write MD report", "path", mdPath, "error", err)
	}

	htmlContent := renderHuntHTML(report)
	if err := fileutil.AtomicWriteFile(htmlPath, []byte(htmlContent), fileutil.FilePermPrivate); err != nil {
		slog.Warn("Failed to write HTML report", "path", htmlPath, "error", err)
	}

	fmt.Fprintln(os.Stdout, mdContent) //nolint:errcheck
}

func renderHuntMarkdown(report *huntReport) string {
	return buildHuntDocument(report).Markdown()
}

// -- Hunt report document rendering with pkg/md --

func groupCandidatesByCategory(candidates []huntCandidate) []huntCategoryGroup {
	groups := lo.GroupBy(candidates, func(candidate huntCandidate) string {
		return candidate.Category
	})

	names := lo.Keys(groups)
	slices.Sort(names)

	views := make([]huntCategoryGroup, 0, len(names))
	for _, name := range names {
		group := groups[name]
		var items []huntCategoryItem
		for i := range group {
			c := &group[i]
			title := c.Title
			if title == "" {
				title = c.NormalizedURL
			}
			reason := c.Reason
			if reason == "" {
				reason = "No reason provided."
			}
			items = append(items, huntCategoryItem{
				Title:         title,
				URL:           c.NormalizedURL,
				Reason:        reason,
				EvidenceURLs:  c.EvidenceURLs,
				Provider:      string(c.Provider),
				CandidateType: string(c.CandidateType),
				Domain:        c.Domain,
				Confidence:    formatConfidence(c.Confidence),
			})
		}
		views = append(views, huntCategoryGroup{
			Name:  name,
			Count: len(group),
			Items: items,
		})
	}

	return views
}

type huntCategoryGroup struct {
	Name  string
	Items []huntCategoryItem
	Count int
}

type huntCategoryItem struct {
	Title         string
	URL           string
	Reason        string
	Provider      string
	CandidateType string
	Domain        string
	Confidence    string
	EvidenceURLs  []string
}

func formatConfidence(f float64) string {
	if f <= 0 {
		return ""
	}

	return strconv.FormatFloat(f, 'f', 2, 64)
}

func renderHuntHTML(report *huntReport) string {
	page, err := buildHuntDocument(report).ToPage()
	if err != nil {
		slog.Warn("Failed to render hunt HTML", "error", err)
		doc2 := md.NewDocument()
		doc2.Add(md.Paragraph(renderHuntMarkdown(report)))
		html2, _ := doc2.ToPage()

		return html2
	}

	return page
}

// buildHuntDocument builds the shared md document for both HTML and Markdown output.
func buildHuntDocument(report *huntReport) *md.Document {
	doc := md.NewDocument()

	statItems := []md.StatItem{
		{Label: "Accepted", Value: report.Stats.AcceptedCandidates},
		{Label: "Categories", Value: report.Stats.CategoriesScanned},
		{Label: "Provider calls", Value: report.Stats.ProviderCalls},
		{Label: "Successful calls", Value: report.Stats.SuccessfulCalls},
		{Label: "Raw candidates", Value: report.Stats.RawCandidates},
		{Label: "Filtered", Value: report.Stats.FilteredCandidates},
	}

	mode := "stateful"
	if report.DryRun {
		mode = "dry-run"
	}

	doc.Add(md.NamedSection("Source Discovery "+report.GeneratedAt[:10],
		md.Paragraph(fmt.Sprintf("Generated: %s. Mode: %s.", report.GeneratedAt, mode)),
		md.StatsGrid(statItems),
	))

	for _, w := range report.Warnings {
		doc.Add(md.Notice("Warning", w.Message))
	}

	for _, f := range report.Failures {
		doc.Add(md.Notice("Failure", f.Message))
	}

	if len(report.Candidates) > 0 {
		categories := groupCandidatesByCategory(report.Candidates)
		for _, cat := range categories {
			items := make([]string, 0, len(cat.Items))
			for i := range cat.Items {
				c := &cat.Items[i]
				items = append(items, md.Link(c.Title, c.URL))
			}
			doc.Add(md.NamedSection(cat.Name, md.BulletList(items, false)))
		}
	} else {
		doc.Add(md.NamedSection("Candidates", md.Paragraph("No candidates accepted.")))
	}

	return doc
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
