package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
	"github.com/xbpk3t/docs-alfred/pkg/rss"
)

// Package-level HTTP client reused across requests.
var huntHTTPClient = &http.Client{Timeout: 30 * time.Second}

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

const (
	huntQueryKey         = "query"
	huntAuthorizationKey = "Authorization"
)

type huntCandidate struct {
	URL           string        `json:"url"`
	Title         string        `json:"title,omitempty"`
	Category      string        `json:"category"`
	CandidateType candidateType `json:"candidateType"`
	Provider      huntProvider  `json:"provider"`
	Reason        string        `json:"reason,omitempty"`
	NormalizedURL string        `json:"normalizedUrl"`
	Domain        string        `json:"domain"`
	FirstSeenAt   string        `json:"firstSeenAt,omitempty"`
	Confidence    float64       `json:"confidence,omitempty"`
	SeenCount     int           `json:"seenCount"`
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

type huntReport struct {
	GeneratedAt string          `json:"generatedAt"`
	Candidates  []huntCandidate `json:"candidates"`
	Warnings    []string        `json:"warnings"`
	Stats       huntStats       `json:"stats"`
	DryRun      bool            `json:"dryRun"`
}

type huntStats struct {
	CategoriesScanned  int `json:"categoriesScanned"`
	AcceptedCandidates int `json:"acceptedCandidates"`
}

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

	categories := filterCategories(cfg, opts.category)
	report.Stats.CategoriesScanned = len(categories)
	blockedSet := make(map[string]bool)
	for _, b := range opts.blocked {
		blockedSet[b] = true
	}

	for _, category := range categories {
		processCategory(category, providerNames, opts.seedLimit, opts.perCat, opts.newOnly, report, state, blockedSet, now)
	}

	writeHuntReports(report, opts.reportMd, opts.reportHTML, opts.reportJSON)
	if !opts.dryRun {
		saveHuntState(opts.state, state)
	}

	slog.Info("Hunt complete", "candidates", report.Stats.AcceptedCandidates, "categories", report.Stats.CategoriesScanned)

	return nil
}

// filterCategories filters cfg.Feeds by categoryFilter, or returns all if filter is empty.
func filterCategories(cfg *rss.Config, categoryFilter []string) []rss.FeedsDetail {
	categories := cfg.Feeds
	if len(categoryFilter) == 0 {
		return categories
	}

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

// processCategory scans a single category with the given providers and updates report/state.
func processCategory(
	category rss.FeedsDetail,
	providerNames []huntProvider,
	seedLimit, perCat int,
	newOnly bool,
	report *huntReport,
	state *huntState,
	blockedSet map[string]bool,
	now time.Time,
) {
	slog.Info("Scanning category", "type", category.Type)

	if seedLimit <= 0 {
		seedLimit = 10
	}

	var seedURLs []string
	for _, u := range category.URLs {
		if u.URL != "" {
			seedURLs = append(seedURLs, u.URL)
		} else if u.Feed != "" {
			seedURLs = append(seedURLs, u.Feed)
		}
	}
	if len(seedURLs) > seedLimit {
		seedURLs = seedURLs[:seedLimit]
	}
	if len(seedURLs) == 0 {
		return
	}

	if perCat <= 0 {
		perCat = 10
	}

	for _, provider := range providerNames {
		candidates := discoverWithProvider(provider, category.Type, seedURLs, perCat)
		processCandidates(candidates, provider, category.Type, report, state, blockedSet, newOnly, now)
	}
}

func processCandidates(candidates []huntCandidate, provider huntProvider, category string,
	report *huntReport, state *huntState, blockedSet map[string]bool, newOnly bool, now time.Time) {
	for ci := range candidates {
		c := candidates[ci]
		c.NormalizedURL = normalizeURL(c.URL)
		c.Domain = extractDomain(c.URL)
		if blockedSet[c.Domain] || isBlocked(c.Domain, blockedSet) {
			continue
		}
		if rec, ok := state.Seen[c.NormalizedURL]; ok {
			c.IsNew = false
			c.SeenCount = rec.Count
			c.FirstSeenAt = rec.FirstSeenAt
			if newOnly {
				continue
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
		report.Candidates = append(report.Candidates, c)
		report.Stats.AcceptedCandidates++
	}
}

// -- Provider implementations --

func discoverWithProvider(provider huntProvider, category string, seedURLs []string, maxResults int) []huntCandidate {
	switch provider {
	case providerExa:
		return discoverExa(category, seedURLs, maxResults)
	case providerTavily:
		return discoverTavily(category, seedURLs, maxResults)
	}

	return nil
}

func discoverExa(category string, seedURLs []string, maxResults int) []huntCandidate {
	apiKey := os.Getenv("EXA_API_KEY")
	if apiKey == "" {
		return nil
	}

	payload := map[string]any{
		huntQueryKey: "site:github.com " + category,
		"numResults": maxResults,
		"useWeb":     true,
	}
	if len(seedURLs) > 0 {
		payload["includeUrls"] = seedURLs
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	resp, err := doHTTPPost("https://api.exa.ai/search", body, map[string]string{
		huntAuthorizationKey: "Bearer " + apiKey,
	})
	if err != nil {
		return nil
	}

	var result struct {
		Results []struct {
			URL   string  `json:"url"`
			Title string  `json:"title"`
			Score float64 `json:"score"`
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
			CandidateType: determineCandidateType(r.URL),
			Provider:      providerExa,
			Confidence:    r.Score,
		})
	}

	return candidates
}

func discoverTavily(category string, seedURLs []string, maxResults int) []huntCandidate {
	apiKey := os.Getenv("TAVILY_API_KEY")
	if apiKey == "" {
		return nil
	}

	payload := map[string]any{
		"query":           fmt.Sprintf("github %s repository", category),
		"max_results":     maxResults,
		"include_domains": []string{"github.com"},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	resp, err := doHTTPPost("https://api.tavily.com/search", body, map[string]string{
		huntAuthorizationKey: "Bearer " + apiKey,
		"Content-Type":       "application/json",
	})
	if err != nil {
		return nil
	}

	var result struct {
		Results []struct {
			URL   string  `json:"url"`
			Title string  `json:"title"`
			Score float64 `json:"score"`
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
			CandidateType: determineCandidateType(r.URL),
			Provider:      providerTavily,
			Confidence:    r.Score,
		})
	}

	return candidates
}

// -- URL helpers --

func normalizeURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	u.Scheme = "https"
	u.RawQuery = ""
	u.Fragment = ""

	return strings.TrimRight(u.String(), "/")
}

func extractDomain(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	return u.Hostname()
}

func determineCandidateType(rawURL string) candidateType {
	u := strings.ToLower(rawURL)
	if strings.Contains(u, "github.com") {
		return candRepo
	}
	if strings.Contains(u, "medium.com") || strings.Contains(u, "substack.com") {
		return candNewsletter
	}

	return candUnknown
}

func isBlocked(domain string, blockedSet map[string]bool) bool {
	parts := strings.Split(domain, ".")
	for i, v := range slices.Backward(parts) {
		partial := strings.Join(parts[i:], ".")
		if blockedSet[partial] {
			return true
		}
		_ = v
	}

	return false
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

	return &state
}

func saveHuntState(path string, state *huntState) {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return
	}
	var dir string
	if idx := strings.LastIndexByte(path, '/'); idx >= 0 {
		dir = path[:idx]
	} else {
		dir = "."
	}
	_ = os.MkdirAll(dir, fileutil.DirPerm)
	_ = os.WriteFile(path, data, fileutil.FilePermPrivate)
}

// -- Report generation --

func writeHuntReports(report *huntReport, mdPath, htmlPath, jsonPath string) {
	jsonData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		slog.Warn("Failed to marshal JSON report", "error", err)

		return
	}

	// JSON report
	ensureDir(jsonPath)
	_ = os.WriteFile(jsonPath, jsonData, fileutil.FilePermPrivate)

	// Markdown report
	mdContent := renderHuntMarkdown(report)
	ensureDir(mdPath)
	_ = os.WriteFile(mdPath, []byte(mdContent), fileutil.FilePermPrivate)

	// HTML report
	htmlContent := renderHuntHTML(report)
	ensureDir(htmlPath)
	_ = os.WriteFile(htmlPath, []byte(htmlContent), fileutil.FilePermPrivate)

	// Also print brief summary to stdout
	fmt.Fprintln(os.Stdout, mdContent) //nolint:errcheck // report delivery to stdout pipe
}

func renderHuntMarkdown(report *huntReport) string {
	var b strings.Builder
	b.WriteString("# Source Discovery Report\n\n")
	fmt.Fprintf(&b, "**Generated:** %s\n", report.GeneratedAt)
	fmt.Fprintf(&b, "**Categories:** %d\n\n", report.Stats.CategoriesScanned)

	for ci := range report.Candidates {
		c := report.Candidates[ci]
		fmt.Fprintf(&b, "- [%s](%s) (%s, %s, %.0f%%)\n",
			c.Title, c.URL, c.Category, c.Provider, c.Confidence*100)
	}

	return b.String()
}

func renderHuntHTML(report *huntReport) string {
	var b strings.Builder
	b.WriteString("<!DOCTYPE html><html><head><meta charset='utf-8'><title>Source Discovery Report</title></head><body>")
	fmt.Fprintf(&b, "<h1>Source Discovery Report</h1><p>Generated: %s</p><p>Categories: %d</p><ul>",
		report.GeneratedAt, report.Stats.CategoriesScanned)

	for ci := range report.Candidates {
		c := report.Candidates[ci]
		fmt.Fprintf(&b, "<li><a href='%s'>%s</a> (%s, %s)</li>",
			c.URL, c.Title, c.Category, c.Provider)
	}
	b.WriteString("</ul></body></html>")

	return b.String()
}

func ensureDir(path string) {
	if idx := strings.LastIndexByte(path, '/'); idx >= 0 {
		_ = os.MkdirAll(path[:idx], fileutil.DirPerm)
	}
}

// -- HTTP --

func doHTTPPost(rawURL string, body []byte, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPost, rawURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := huntHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)

		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return io.ReadAll(resp.Body)
}

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
