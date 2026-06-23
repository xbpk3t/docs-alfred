package cmd

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	rss "github.com/xbpk3t/docs-alfred/internal/rss/feed"
)

func TestParseHuntProviders(t *testing.T) {
	assert.Nil(t, parseHuntProviders(""))
	assert.Nil(t, parseHuntProviders("unknown"))
	assert.Equal(t, []huntProvider{providerExa}, parseHuntProviders("exa"))
	assert.Equal(t, []huntProvider{providerTavily}, parseHuntProviders("tavily"))
	assert.Equal(t, []huntProvider{providerExa, providerTavily}, parseHuntProviders("exa,tavily"))
	assert.Equal(t, []huntProvider{providerExa}, parseHuntProviders(" exa , unknown "))
}

func TestBuildBlockedSet(t *testing.T) {
	set := buildBlockedSet(
		[]string{"facebook.com", "twitter.com"},
		[]string{"reddit.com"},
		[]string{"custom.com"},
	)
	assert.True(t, set["facebook.com"])
	assert.True(t, set["twitter.com"])
	assert.True(t, set["reddit.com"])
	assert.True(t, set["custom.com"])
	assert.False(t, set["example.com"])
}

func TestBuildBlockedSetDeduplicatesAndTrims(t *testing.T) {
	set := buildBlockedSet(
		[]string{" Facebook.com "},
		[]string{"facebook.com"},
		nil,
	)
	assert.True(t, set["facebook.com"])
}

func TestFilterCategories(t *testing.T) {
	cfg := &rss.Config{
		RSS: []rss.FeedsDetail{
			{Type: "podcast", Feeds: []rss.Feeds{{Feed: "http://a.com"}}},
			{Type: "blog", Feeds: []rss.Feeds{{Feed: "http://b.com"}}},
			{Type: "newsletter", Feeds: []rss.Feeds{{Feed: "http://c.com"}}},
		},
	}

	// No filter
	got := filterCategories(cfg, nil)
	assert.Len(t, got, 3)

	// CLI filter
	got = filterCategories(cfg, []string{"podcast"})
	assert.Len(t, got, 1)
	assert.Equal(t, "podcast", got[0].Type)

	// Multiple CLI filter
	got = filterCategories(cfg, []string{"podcast,blog"})
	assert.Len(t, got, 2)
}

func TestFilterCategoriesWithExcept(t *testing.T) {
	cfg := &rss.Config{
		RSS: []rss.FeedsDetail{
			{Type: "podcast", Feeds: []rss.Feeds{{Feed: "http://a.com"}}},
			{Type: "blog", Feeds: []rss.Feeds{{Feed: "http://b.com"}}},
		},
		HuntConfig: rss.HuntConfig{
			Categories: &rss.HuntCategoriesConfig{Except: []string{"blog"}},
		},
	}
	got := filterCategories(cfg, nil)
	assert.Len(t, got, 1)
	assert.Equal(t, "podcast", got[0].Type)
}

func TestBuildSeeds(t *testing.T) {
	cat := rss.FeedsDetail{
		Feeds: []rss.Feeds{
			{URL: "https://example.com", Des: "Example"},
			{Feed: "https://example.com/feed", Des: "Feed"},
			{Feed: "https://other.com/feed"},
		},
	}
	urls, descs := buildSeeds(cat, 10)
	assert.Len(t, urls, 3)
	assert.Len(t, descs, 2)

	// With seed limit
	urls, _ = buildSeeds(cat, 2)
	assert.Len(t, urls, 2)
}

func TestBuildSeedsEmpty(t *testing.T) {
	cat := rss.FeedsDetail{Feeds: []rss.Feeds{}}
	urls, descs := buildSeeds(cat, 10)
	assert.Nil(t, urls)
	assert.Nil(t, descs)
}

func TestClassifyCandidate(t *testing.T) {
	assert.Equal(t, candRepo, classifyCandidate("https://github.com/user/repo", "A repo"))
	assert.Equal(t, candNewsletter, classifyCandidate("https://example.com", "My Newsletter"))
	assert.Equal(t, candNewsletter, classifyCandidate("https://john.substack.com", "Blog"))
	assert.Equal(t, candAuthor, classifyCandidate("https://blog.example.com", "A blog post"))
	// example.com has 2 domain parts and no "blog" in title, so it's classified as author
	assert.Equal(t, candAuthor, classifyCandidate("https://example.com/resource", "Resource"))
	// Multi-part domain with non-blog title => source
	assert.Equal(t, candSource, classifyCandidate("https://tech.example.com/resource", "Resource"))
}

func TestNormalizeConfidence(t *testing.T) {
	assert.Equal(t, 0.7, normalizeConfidence(0, 0.7))
	assert.Equal(t, 0.7, normalizeConfidence(math.NaN(), 0.7))
	assert.Equal(t, 0.8, normalizeConfidence(0.8, 0.7))
	assert.Equal(t, 1.0, normalizeConfidence(1.0, 0.7))
	assert.InDelta(t, 0.5, normalizeConfidence(5.0, 0.7), 0.01)
	assert.Equal(t, 0.0, normalizeConfidence(-1.0, 0.7))
}

func TestBuildReason(t *testing.T) {
	reason := buildReason("tech", []string{"seed1"}, "Exa", "Summary text")
	assert.Contains(t, reason, "tech")
	assert.Contains(t, reason, "Exa")
	assert.Contains(t, reason, "Summary text")
}

func TestBuildReasonEmptySummary(t *testing.T) {
	reason := buildReason("tech", nil, "Exa", "")
	assert.Contains(t, reason, "Exa")
	assert.NotContains(t, reason, "Summary")
}

func TestBuildReasonLongSummary(t *testing.T) {
	longSummary := ""
	for i := 0; i < 300; i++ {
		longSummary += "word"
	}
	reason := buildReason("tech", nil, "Exa", longSummary)
	assert.LessOrEqual(t, len(reason), 260+50) // reason prefix + truncated summary
}

func TestTrimToMaxLength(t *testing.T) {
	assert.Equal(t, "hello", trimToMaxLength("hello", 100))
	assert.Equal(t, "hello...", trimToMaxLength("hello world foo bar", 8))
	assert.Equal(t, "a b", trimToMaxLength("a  b", 10))
}

func TestSortCandidatesByScore(t *testing.T) {
	candidates := []huntCandidate{
		{Score: 0.5},
		{Score: 0.9},
		{Score: 0.1},
	}
	sortCandidatesByScore(candidates)
	assert.Equal(t, 0.9, candidates[0].Score)
	assert.Equal(t, 0.5, candidates[1].Score)
	assert.Equal(t, 0.1, candidates[2].Score)
}

func TestComputeCandidateScore(t *testing.T) {
	hc := &huntRunConfig{
		providerWeights: map[string]float64{"exa": 1.0, "tavily": 0.9},
		typeWeights: map[string]float64{
			"source": 1.0, "author": 0.9, "newsletter": 0.8, "repo": 0.7, "unknown": 0.5,
		},
	}
	c := &huntCandidate{Provider: providerExa, CandidateType: candSource, Confidence: 0.8}
	score := computeCandidateScore(c, hc)
	assert.InDelta(t, 0.8, score, 0.01) // 1.0 * 1.0 * 0.8
}

func TestComputeCandidateScoreMissingWeights(t *testing.T) {
	hc := &huntRunConfig{
		providerWeights: map[string]float64{},
		typeWeights:     map[string]float64{},
	}
	c := &huntCandidate{Provider: providerExa, CandidateType: candSource, Confidence: 0.0}
	score := computeCandidateScore(c, hc)
	assert.InDelta(t, 0.2, score, 0.01) // 0.8 * 0.5 * 0.5
}

func TestBuildDiscoveryQuery(t *testing.T) {
	query := buildDiscoveryQuery("tech",
		[]string{"https://a.com", "https://b.com"},
		[]string{"desc1"},
		[]string{"topic1"},
	)
	assert.Contains(t, query, "tech")
	assert.Contains(t, query, "https://a.com")
	assert.Contains(t, query, "desc1")
	assert.Contains(t, query, "topic1")
}

func TestBuildTavilyQuery(t *testing.T) {
	query := buildTavilyQuery("tech",
		[]string{"https://a.com", "https://b.com"},
		[]string{"topic1"},
	)
	assert.Contains(t, query, "tech")
	assert.Contains(t, query, "https://a.com")
}

func TestFormatConfidence(t *testing.T) {
	assert.Equal(t, "", formatConfidence(0))
	assert.Equal(t, "", formatConfidence(-1))
	assert.Equal(t, "0.85", formatConfidence(0.85))
}

func TestGroupCandidatesByCategory(t *testing.T) {
	candidates := []huntCandidate{
		{Category: "tech", Title: "Blog A", NormalizedURL: "https://a.com", Domain: "a.com", Provider: providerExa, CandidateType: candSource, Confidence: 0.8},
		{Category: "tech", Title: "Blog B", NormalizedURL: "https://b.com", Domain: "b.com", Provider: providerExa, CandidateType: candSource, Confidence: 0.7},
		{Category: "science", Title: "Journal", NormalizedURL: "https://c.com", Domain: "c.com", Provider: providerTavily, CandidateType: candSource, Confidence: 0.9},
	}
	groups := groupCandidatesByCategory(candidates)
	require.Len(t, groups, 2)
	// Groups are sorted by name
	assert.Equal(t, "science", groups[0].Name)
	assert.Equal(t, "tech", groups[1].Name)
	assert.Len(t, groups[1].Items, 2)
}

func TestGroupCandidatesByCategoryEmpty(t *testing.T) {
	groups := groupCandidatesByCategory(nil)
	assert.Empty(t, groups)
}

func TestBuildHuntDocumentEmpty(t *testing.T) {
	report := &huntReport{
		GeneratedAt: "2026-01-01T00:00:00Z",
		Stats:       huntStats{},
	}
	doc := buildHuntDocument(report)
	assert.NotNil(t, doc)
	md := doc.Markdown()
	assert.Contains(t, md, "Source Discovery")
}

func TestIsNewsletterDomain(t *testing.T) {
	assert.True(t, isNewsletterDomain("john.substack.com", ""))
	assert.True(t, isNewsletterDomain("blog.buttondown.email", ""))
	assert.True(t, isNewsletterDomain("example.com", "my newsletter"))
	assert.False(t, isNewsletterDomain("example.com", "regular blog"))
}

func TestIsAuthorDomain(t *testing.T) {
	assert.True(t, isAuthorDomain("example.com", "my blog"))
	assert.True(t, isAuthorDomain("dev.io", "tech"))
	assert.False(t, isAuthorDomain("blog.example.com", "posts"))
}

func TestBuildExceptSet(t *testing.T) {
	cfg := &rss.Config{
		HuntConfig: rss.HuntConfig{
			Categories: &rss.HuntCategoriesConfig{Except: []string{"podcast", "blog"}},
		},
	}
	except := buildExceptSet(cfg)
	assert.True(t, except["podcast"])
	assert.True(t, except["blog"])
	assert.False(t, except["newsletter"])
}

func TestBuildExceptSetNil(t *testing.T) {
	cfg := &rss.Config{}
	except := buildExceptSet(cfg)
	assert.Empty(t, except)
}

func TestFilterByCLI(t *testing.T) {
	categories := []rss.FeedsDetail{
		{Type: "podcast"},
		{Type: "blog"},
		{Type: "newsletter"},
	}
	got := filterByCLI(categories, []string{"podcast,blog"})
	assert.Len(t, got, 2)
}

func TestFilterByExcept(t *testing.T) {
	categories := []rss.FeedsDetail{
		{Type: "podcast"},
		{Type: "blog"},
	}
	except := map[string]bool{"blog": true}
	got := filterByExcept(categories, except)
	assert.Len(t, got, 1)
	assert.Equal(t, "podcast", got[0].Type)
}

func TestProcessCandidateSeenNew(t *testing.T) {
	hc := &huntRunConfig{
		state: &huntState{Seen: make(map[string]huntSeenRecord)},
		now:   testTime(),
	}
	c := &huntCandidate{NormalizedURL: "https://example.com"}
	ok := processCandidateSeen(c, hc)
	assert.True(t, ok)
	assert.True(t, c.IsNew)
	// New candidates don't have SeenCount set on the candidate struct
	assert.Equal(t, 0, c.SeenCount)
	// But the state is updated
	rec, exists := hc.state.Seen["https://example.com"]
	assert.True(t, exists)
	assert.Equal(t, 1, rec.Count)
}

func TestProcessCandidateSeenExisting(t *testing.T) {
	hc := &huntRunConfig{
		state: &huntState{Seen: map[string]huntSeenRecord{
			"https://example.com": {FirstSeenAt: "2026-01-01T00:00:00Z", Count: 3},
		}},
		now: testTime(),
	}
	c := &huntCandidate{NormalizedURL: "https://example.com"}
	ok := processCandidateSeen(c, hc)
	assert.True(t, ok)
	assert.False(t, c.IsNew)
	assert.Equal(t, 3, c.SeenCount)
}

func TestProcessCandidateSeenNewOnly(t *testing.T) {
	hc := &huntRunConfig{
		state: &huntState{Seen: map[string]huntSeenRecord{
			"https://example.com": {FirstSeenAt: "2026-01-01T00:00:00Z", Count: 3},
		}},
		newOnly: true,
		now:     testTime(),
	}
	c := &huntCandidate{NormalizedURL: "https://example.com"}
	ok := processCandidateSeen(c, hc)
	assert.False(t, ok)
}

func testTime() time.Time {
	return time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
}

// ---------------------------------------------------------------------------
// classifyCandidate more cases
// ---------------------------------------------------------------------------

func TestClassifyCandidateNewsletterSubstack(t *testing.T) {
	assert.Equal(t, candNewsletter, classifyCandidate("https://myblog.substack.com", "Some Blog"))
}

func TestClassifyCandidateNewsletterButtondown(t *testing.T) {
	assert.Equal(t, candNewsletter, classifyCandidate("https://mynewsletter.buttondown.email", "Some Newsletter"))
}

func TestClassifyCandidateSourceMultiPartDomain(t *testing.T) {
	assert.Equal(t, candSource, classifyCandidate("https://blog.tech.example.com/resource", "Resource"))
}

func TestClassifyCandidateUnknownEmptyDomain(t *testing.T) {
	// Invalid URL produces empty domain -> unknown
	assert.Equal(t, candUnknown, classifyCandidate("://invalid", "Title"))
}

// ---------------------------------------------------------------------------
// normalizeConfidence edge cases
// ---------------------------------------------------------------------------

func TestNormalizeConfidenceNegativeScore(t *testing.T) {
	assert.Equal(t, 0.0, normalizeConfidence(-5.0, 0.7))
}

func TestNormalizeConfidenceLargeScore(t *testing.T) {
	result := normalizeConfidence(100.0, 0.7)
	assert.Equal(t, 1.0, result) // 100/10=10, clamped to 1
}

func TestNormalizeConfidenceBoundaryScore(t *testing.T) {
	result := normalizeConfidence(10.0, 0.7)
	assert.Equal(t, 1.0, result) // 10/10=1.0
}

// ---------------------------------------------------------------------------
// buildReason edge cases
// ---------------------------------------------------------------------------

func TestBuildReasonLongSummaryTruncated(t *testing.T) {
	longSummary := strings.Repeat("a", 300)
	reason := buildReason("tech", nil, "Exa", longSummary)
	assert.LessOrEqual(t, len(reason), 300)
	assert.NotContains(t, reason, strings.Repeat("a", 250)) // truncated
}

func TestBuildReasonEmptySeedDescs(t *testing.T) {
	reason := buildReason("science", nil, "Tavily", "summary")
	assert.Contains(t, reason, "science")
	assert.Contains(t, reason, "Tavily")
}

// ---------------------------------------------------------------------------
// buildTavilyQuery edge cases
// ---------------------------------------------------------------------------

func TestBuildTavilyQueryManySeeds(t *testing.T) {
	seeds := make([]string, 20)
	for i := range seeds {
		seeds[i] = "https://example" + strings.Repeat("x", 50) + ".com/feed"
	}
	query := buildTavilyQuery("tech", seeds, nil)
	assert.LessOrEqual(t, len(query), 403) // maxLen + "..."
}

func TestBuildTavilyQueryManyTopics(t *testing.T) {
	topics := make([]string, 20)
	for i := range topics {
		topics[i] = "topic" + strings.Repeat("x", 50)
	}
	query := buildTavilyQuery("tech", nil, topics)
	assert.LessOrEqual(t, len(query), 403)
}

// ---------------------------------------------------------------------------
// buildDiscoveryQuery edge cases
// ---------------------------------------------------------------------------

func TestBuildDiscoveryQueryEmpty(t *testing.T) {
	query := buildDiscoveryQuery("tech", nil, nil, nil)
	assert.Contains(t, query, "tech")
	assert.Contains(t, query, "Avoid")
}

func TestBuildDiscoveryQueryLongSeeds(t *testing.T) {
	seeds := make([]string, 20)
	for i := range seeds {
		seeds[i] = "https://example.com/" + strings.Repeat("x", 100)
	}
	query := buildDiscoveryQuery("tech", seeds, nil, nil)
	assert.Contains(t, query, "tech")
}

// ---------------------------------------------------------------------------
// groupCandidatesByCategory with candidate without title
// ---------------------------------------------------------------------------

func TestGroupCandidatesByCategoryNoTitle(t *testing.T) {
	candidates := []huntCandidate{
		{Category: "tech", NormalizedURL: "https://a.com", Domain: "a.com", Provider: providerExa, CandidateType: candSource, Confidence: 0.8},
	}
	groups := groupCandidatesByCategory(candidates)
	require.Len(t, groups, 1)
	assert.Len(t, groups[0].Items, 1)
	assert.Equal(t, "https://a.com", groups[0].Items[0].Title) // falls back to NormalizedURL
}

func TestGroupCandidatesByCategoryEmptyReason(t *testing.T) {
	candidates := []huntCandidate{
		{Category: "tech", Title: "Blog", NormalizedURL: "https://a.com", Domain: "a.com", Provider: providerExa, CandidateType: candSource, Confidence: 0.8, Reason: ""},
	}
	groups := groupCandidatesByCategory(candidates)
	require.Len(t, groups, 1)
	assert.Equal(t, "No reason provided.", groups[0].Items[0].Reason)
}

// ---------------------------------------------------------------------------
// buildBlockedSet edge cases
// ---------------------------------------------------------------------------

func TestBuildBlockedSetEmptyInputs(t *testing.T) {
	set := buildBlockedSet(nil, nil, nil)
	assert.Empty(t, set)
}

func TestBuildBlockedSetWhitespaceOnly(t *testing.T) {
	set := buildBlockedSet([]string{"  ", ""}, nil, nil)
	assert.Empty(t, set)
}

// ---------------------------------------------------------------------------
// processCandidateSeen with nil state.Seen
// ---------------------------------------------------------------------------

func TestProcessCandidateSeenNilSeen(t *testing.T) {
	hc := &huntRunConfig{
		state: &huntState{Seen: nil},
		now:   testTime(),
	}
	c := &huntCandidate{NormalizedURL: "https://example.com"}
	ok := processCandidateSeen(c, hc)
	assert.True(t, ok)
	assert.True(t, c.IsNew)
	assert.NotNil(t, hc.state.Seen)
}

// ---------------------------------------------------------------------------
// sortCandidatesByScore edge cases
// ---------------------------------------------------------------------------

func TestSortCandidatesByScoreEqualScores(t *testing.T) {
	candidates := []huntCandidate{
		{Score: 0.5, Title: "A"},
		{Score: 0.5, Title: "B"},
		{Score: 0.5, Title: "C"},
	}
	sortCandidatesByScore(candidates)
	assert.Equal(t, 0.5, candidates[0].Score)
	assert.Equal(t, 0.5, candidates[1].Score)
	assert.Equal(t, 0.5, candidates[2].Score)
}

func TestSortCandidatesByScoreEmpty(t *testing.T) {
	candidates := []huntCandidate{}
	sortCandidatesByScore(candidates)
	assert.Empty(t, candidates)
}

func TestSortCandidatesByScoreSingle(t *testing.T) {
	candidates := []huntCandidate{{Score: 0.5}}
	sortCandidatesByScore(candidates)
	assert.Len(t, candidates, 1)
}

// ---------------------------------------------------------------------------
// renderHuntMarkdown edge cases
// ---------------------------------------------------------------------------

func TestRenderHuntMarkdownWithFailures(t *testing.T) {
	report := &huntReport{
		GeneratedAt: "2026-06-23T00:00:00Z",
		DryRun:      false,
		Stats:       huntStats{AcceptedCandidates: 0},
		Warnings:    []huntWarning{{Message: "rate limited"}},
		Failures:    []huntFailure{{Message: "API error"}},
	}
	result := renderHuntMarkdown(report)
	assert.Contains(t, result, "rate limited")
	assert.Contains(t, result, "API error")
	assert.Contains(t, result, "stateful")
}

func TestRenderHuntMarkdownNoCandidates(t *testing.T) {
	report := &huntReport{
		GeneratedAt: "2026-06-23T00:00:00Z",
		DryRun:      true,
		Stats:       huntStats{},
	}
	result := renderHuntMarkdown(report)
	assert.Contains(t, result, "No candidates accepted")
}

// ---------------------------------------------------------------------------
// renderHuntHTML edge cases
// ---------------------------------------------------------------------------

func TestRenderHuntHTMLWithCandidates(t *testing.T) {
	report := &huntReport{
		GeneratedAt: "2026-06-23T00:00:00Z",
		DryRun:      true,
		Stats:       huntStats{AcceptedCandidates: 1},
		Candidates: []huntCandidate{
			{Title: "Blog", NormalizedURL: "https://blog.example.com", Category: "tech", Domain: "blog.example.com"},
		},
	}
	html := renderHuntHTML(report)
	assert.NotEmpty(t, html)
	assert.Contains(t, html, "Blog")
}

// ---------------------------------------------------------------------------
// isRepoURL
// ---------------------------------------------------------------------------

func TestIsRepoURLGitHub(t *testing.T) {
	assert.True(t, isRepoURL("https://github.com/user/repo"))
}

func TestIsRepoURLNotRepo(t *testing.T) {
	assert.False(t, isRepoURL("https://example.com/blog"))
}

// ---------------------------------------------------------------------------
// isBlocked with subdomain
// ---------------------------------------------------------------------------

func TestIsBlockedSubdomain(t *testing.T) {
	blocked := map[string]bool{"facebook.com": true}
	assert.True(t, isBlocked("m.facebook.com", blocked))
	assert.True(t, isBlocked("www.facebook.com", blocked))
}
