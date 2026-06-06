package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/PuerkitoBio/purell"
	"github.com/mmcdole/gofeed"
	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
	"github.com/xbpk3t/docs-alfred/pkg/httputil"
	"github.com/xbpk3t/docs-alfred/pkg/rss"
	"golang.org/x/net/publicsuffix"
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

type huntReport struct {
	GeneratedAt string          `json:"generatedAt"`
	Candidates  []huntCandidate `json:"candidates"`
	Warnings    []huntWarning   `json:"warnings"`
	Stats       huntStats       `json:"stats"`
	DryRun      bool            `json:"dryRun"`
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
			return runHunt(&opts)
		},
	}

	cmd.Flags().StringVarP(&opts.config, "config", "c", "rss2nl.yml", "Config file path")
	cmd.Flags().StringVar(&opts.state, "state", ".cache/rss2nl/hunt/feeds-hunt-state.json", "State file path")
	cmd.Flags().StringArrayVar(&opts.category, "category", nil, "Category to scan")
	cmd.Flags().StringVar(&opts.providers, "providers", "", "Providers: exa,tavily")
	cmd.Flags().IntVar(&opts.max, "max", 0, "Global candidate cap")
	cmd.Flags().IntVar(&opts.perCat, "per-category", 0, "Candidate cap per category")
	cmd.Flags().IntVar(&opts.providerMax, "provider-max", 0, "Raw candidates per provider per category")
	cmd.Flags().IntVar(&opts.seedLimit, "seed-limit", 0, "Seed source cap per category")
	cmd.Flags().StringVar(&opts.reportMd, "report-md", ".cache/rss2nl/hunt/feeds-hunt-report.md", "Markdown report")
	cmd.Flags().StringVar(&opts.reportHTML, "report-html", ".cache/rss2nl/hunt/feeds-hunt-report.html", "HTML report")
	cmd.Flags().StringVar(&opts.reportJSON, "report-json", ".cache/rss2nl/hunt/feeds-hunt-report.json", "JSON report")
	cmd.Flags().StringArrayVar(&opts.blocked, "blocked-domain", nil, "Extra blocked domain")
	cmd.Flags().BoolVar(&opts.newOnly, "new-only", false, "Only accept candidates not in state")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "Write reports only")
	cmd.Flags().BoolVar(&opts.sendMail, "send-mail", false, "Send HTML report through Resend")

	return cmd
}

func runHunt(opts *struct {
	reportHTML, config, providers, reportMd, state, reportJSON string
	category, blocked                                          []string
	max, providerMax, seedLimit, perCat                        int
	newOnly, dryRun, sendMail                                  bool
}) error {
	cfg, err := rss.NewConfig(opts.config)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
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

	for _, category := range categories {
		processCategory(category, providerNames, opts.seedLimit, opts.perCat, opts.providerMax,
			opts.newOnly, report, state, blockedSet, now, providerWeights, typeWeights)
	}

	sortCandidatesByScore(report.Candidates)

	if opts.max > 0 && len(report.Candidates) > opts.max {
		report.Candidates = report.Candidates[:opts.max]
		report.Stats.AcceptedCandidates = len(report.Candidates)
	}

	writeHuntReports(report, opts.reportMd, opts.reportHTML, opts.reportJSON)
	if !opts.dryRun {
		saveHuntState(opts.state, state)
	}

	slog.Info("Hunt complete",
		"candidates", report.Stats.AcceptedCandidates,
		"categories", report.Stats.CategoriesScanned,
	)

	return nil
}

// -- Blocked set --

func buildBlockedSet(defaults, configBlocked, flagBlocked []string) map[string]bool {
	set := make(map[string]bool)
	for _, d := range defaults {
		set[strings.ToLower(d)] = true
	}
	for _, d := range configBlocked {
		set[strings.ToLower(d)] = true
	}
	for _, d := range flagBlocked {
		set[strings.ToLower(d)] = true
	}

	return set
}

// -- Category processing --

func filterCategories(cfg *rss.Config, categoryFilter []string) []rss.FeedsDetail {
	categories := cfg.Feeds
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
	catMap := make(map[string]bool)
	for _, c := range categoryFilter {
		for item := range strings.SplitSeq(c, ",") {
			catMap[strings.TrimSpace(item)] = true
		}
	}

	var filtered []rss.FeedsDetail
	for _, f := range categories {
		if catMap[f.Type] {
			filtered = append(filtered, f)
		}
	}

	return filtered
}

func filterByExcept(categories []rss.FeedsDetail, except map[string]bool) []rss.FeedsDetail {
	var filtered []rss.FeedsDetail
	for _, f := range categories {
		if !except[f.Type] {
			filtered = append(filtered, f)
		}
	}

	return filtered
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

	recentTopics := enrichTopics(category.URLs)

	for _, provider := range providerNames {
		processCategoryProvider(provider, category.Type, seedURLs, seedDescs, providerMax,
			recentTopics, report, state, blockedSet, newOnly, now, providerWeights, typeWeights)
	}
}

//nolint:nonamedreturns
func buildSeeds(category rss.FeedsDetail, seedLimit int) (seedURLs, seedDescs []string) {
	for _, u := range category.URLs {
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
) {
	candidates := discoverWithProvider(provider, categoryType, seedURLs, seedDescs, providerMax, recentTopics)
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
) []huntCandidate {
	switch provider {
	case providerExa:

		return discoverExa(category, seedURLs, seedDescs, maxResults, recentTopics)
	case providerTavily:

		return discoverTavily(category, seedURLs, seedDescs, maxResults, recentTopics)
	}

	return nil
}

func discoverExa(category string, seedURLs, seedDescs []string, maxResults int, recentTopics []string) []huntCandidate {
	apiKey := os.Getenv("EXA_API_KEY")
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

	body, err := json.Marshal(payload)
	if err != nil {
		return nil
	}

	// Exa uses x-api-key header (not Authorization: Bearer)
	resp, err := httputil.PostJSON("https://api.exa.ai/search", body, map[string]string{
		"x-api-key": apiKey,
	})
	if err != nil {
		return nil
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
	if err := json.Unmarshal(resp, &result); err != nil || len(result.Results) == 0 {
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

func discoverTavily(category string, seedURLs, seedDescs []string, maxResults int, recentTopics []string) []huntCandidate {
	apiKey := os.Getenv("TAVILY_API_KEY")
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

	body, err := json.Marshal(payload)
	if err != nil {
		return nil
	}

	resp, err := httputil.PostJSON("https://api.tavily.com/search", body, map[string]string{
		"Content-Type": "application/json",
	})
	if err != nil {
		return nil
	}

	var result struct {
		Results []struct {
			URL     string  `json:"url"`
			Title   string  `json:"title"`
			Content string  `json:"content"`
			Score   float64 `json:"score"`
		} `json:"results"`
	}
	if err := json.Unmarshal(resp, &result); err != nil || len(result.Results) == 0 {
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
	if !strings.Contains(u, "github.com") && !strings.Contains(u, "gitlab.com") {
		return false
	}
	path := strings.TrimPrefix(strings.TrimPrefix(u, "https://"), "http://")
	segments := strings.Split(strings.Trim(path, "/"), "/")

	return len(segments) >= 2
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
	normalized, err := purell.NormalizeURLString(rawURL,
		purell.FlagLowercaseScheme|
			purell.FlagLowercaseHost|
			purell.FlagRemoveDefaultPort|
			purell.FlagRemoveDotSegments|
			purell.FlagRemoveFragment|
			purell.FlagRemoveDuplicateSlashes|
			purell.FlagSortQuery)
	if err != nil {
		return rawURL
	}

	return strings.TrimRight(normalized, "/")
}

func extractDomain(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	return u.Hostname()
}

// isBlocked checks if a domain matches any blocked entry by registrable domain.
func isBlocked(domain string, blockedSet map[string]bool) bool {
	// First try exact match
	if blockedSet[domain] {
		return true
	}
	// Then try registrable domain match
	regDomain, err := publicsuffix.EffectiveTLDPlusOne(domain)
	if err != nil {
		// Fallback to suffix matching for IP addresses or unusual TLDs
		parts := strings.Split(domain, ".")
		for i := range slices.Backward(parts) {
			partial := strings.Join(parts[i:], ".")
			if blockedSet[partial] {
				return true
			}
		}

		return false
	}

	return blockedSet[regDomain]
}

// -- State persistence --

func loadHuntState(path string) *huntState {
	data, err := os.ReadFile(path)
	if err != nil {
		return &huntState{Seen: make(map[string]huntSeenRecord)}
	}
	var state huntState
	if err := json.Unmarshal(data, &state); err != nil {
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
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return
	}
	_ = fileutil.EnsureFileDir(path)
	_ = os.WriteFile(path, data, fileutil.FilePermPrivate)
}

// -- Report generation --

func writeHuntReports(report *huntReport, mdPath, htmlPath, jsonPath string) {
	jsonData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		slog.Warn("Failed to marshal JSON report", "error", err)

		return
	}
	_ = fileutil.EnsureFileDir(jsonPath)
	_ = os.WriteFile(jsonPath, jsonData, fileutil.FilePermPrivate)

	mdContent := renderHuntMarkdown(report)
	_ = fileutil.EnsureFileDir(mdPath)
	_ = os.WriteFile(mdPath, []byte(mdContent), fileutil.FilePermPrivate)

	htmlContent := renderHuntHTML(report)
	_ = fileutil.EnsureFileDir(htmlPath)
	_ = os.WriteFile(htmlPath, []byte(htmlContent), fileutil.FilePermPrivate)

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

func renderHuntHTML(report *huntReport) string {
	funcMap := template.FuncMap{
		"percent": func(f float64) string {
			return fmt.Sprintf("%.0f", f*100)
		},
		"isNewBadge": func(isNew bool) string {
			if isNew {
				return `<span style="background:#22c55e;color:#fff;padding:2px 6px;border-radius:4px;font-size:11px">NEW</span>`
			}

			return ""
		},
	}

	//nolint:lll
	tmplSrc := `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Source Discovery Report</title>
<style>
  :root { --bg: #fff; --text: #1f2937; --border: #e5e7eb; --card-bg: #f9fafb; }
  @media (prefers-color-scheme: dark) {
    :root { --bg: #111827; --text: #f3f4f6; --border: #374151; --card-bg: #1f2937; }
  }
  * { box-sizing: border-box; }
  body { font-family: -apple-system, system-ui, sans-serif; background: var(--bg); color: var(--text); max-width: 960px; margin: 0 auto; padding: 20px; }  //nolint:lll
  h1 { font-size: 1.5em; margin-bottom: 0.25em; }
  .meta { color: #6b7280; font-size: 0.9em; margin-bottom: 1.5em; }
  .stats { display: flex; gap: 1em; margin-bottom: 1.5em; }
  .stat { background: var(--card-bg); border: 1px solid var(--border); border-radius: 8px; padding: 12px 20px; flex: 1; text-align: center; }  //nolint:lll
  .stat-value { font-size: 1.5em; font-weight: 700; }
  .stat-label { font-size: 0.8em; color: #6b7280; }
  .warning { background: #fef3c7; border-left: 4px solid #f59e0b; padding: 10px 15px; margin-bottom: 1em; border-radius: 4px; color: #92400e; }  //nolint:lll
  .candidate { padding: 12px 0; border-bottom: 1px solid var(--border); }
  .candidate-title { font-weight: 600; }
  .candidate-title a { color: #3b82f6; text-decoration: none; }
  .candidate-title a:hover { text-decoration: underline; }
  .candidate-meta { font-size: 0.85em; color: #6b7280; margin-top: 4px; }
  .candidate-reason { font-size: 0.85em; margin-top: 4px; }
  .badge { display: inline-block; padding: 1px 6px; border-radius: 4px; font-size: 11px; margin-right: 4px; }
  .badge-exa { background: #dbeafe; color: #1e40af; }
  .badge-tavily { background: #fce7f3; color: #9d174d; }
  .badge-repo { background: #d1fae5; color: #065f46; }
</style>
</head>
<body>
<h1>Source Discovery Report</h1>
<div class="meta">Generated: {{.GeneratedAt}} · Dry Run: {{.DryRun}}</div>
<div class="stats">
  <div class="stat"><div class="stat-value">{{.Stats.CategoriesScanned}}</div><div class="stat-label">Categories</div></div>
  <div class="stat"><div class="stat-value">{{.Stats.AcceptedCandidates}}</div><div class="stat-label">Candidates</div></div>
</div>
{{range .Warnings}}
<div class="warning">⚠ {{.Message}}</div>
{{end}}
{{range .Candidates}}
<div class="candidate">
  <div class="candidate-title">{{isNewBadge .IsNew}} <a href="{{.URL}}" target="_blank">{{.Title}}</a></div>
  <div class="candidate-meta">
    <span class="badge badge-{{.Provider}}">{{.Provider}}</span>
    <span class="badge badge-{{.CandidateType}}">{{.CandidateType}}</span>
    {{.Category}} · Score: {{percent .Score}}% · Confidence: {{percent .Confidence}}%
  </div>
  {{if .Reason}}<div class="candidate-reason">{{.Reason}}</div>{{end}}
</div>
{{end}}
</body>
</html>`

	tmpl, err := template.New("hunt-report").Funcs(funcMap).Parse(tmplSrc)
	if err != nil {
		slog.Warn("Failed to parse hunt HTML template", "error", err)

		return fmt.Sprintf("<pre>%s</pre>", renderHuntMarkdown(report))
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, report); err != nil {
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
