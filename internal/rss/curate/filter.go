package curate

import (
	"context"
	"fmt"
	"time"

	"github.com/mmcdole/gofeed"

	rss "github.com/xbpk3t/docs-alfred/internal/rss/feed"
	"github.com/xbpk3t/docs-alfred/pkg/urlutil"
)

// publishTime picks the best publication instant for an item, falling back
// from an explicit publish date to the update date.
func publishTime(it *gofeed.Item) *time.Time {
	if it.PublishedParsed != nil {
		return it.PublishedParsed
	}
	if it.UpdatedParsed != nil {
		return it.UpdatedParsed
	}

	return nil
}

// feedStatus strings for freqResult.Status.
const (
	statusPass    = "pass"
	statusNoURL   = "invalid_url"
	statusUnreach = "unreachable"
	statusFreqLow = "freq_low"
	statusDup     = "dedupe"
	statusFatal   = "fatal"
)

// existingFeedURLs builds the set of feed/source URLs already subscribed in
// rss2nl.yml; used as the Step 2 dedupe reference.
func existingFeedURLs(cfg *rss.Config) map[string]bool {
	set := make(map[string]bool)
	for _, d := range cfg.RSS {
		for _, f := range d.Feeds {
			for _, u := range []string{f.Feed, f.URL} {
				if u != "" {
					set[normalizer(u)] = true
				}
			}
		}
	}

	return set
}

// normalizer canonicalizes a feed URL for dedupe and classification lookups.
func normalizer(u string) string { return urlutil.Normalize(u) }

// step2Fetch is Step 2: inject host limits, dedupe, fetch every candidate that
// still needs it, then run the F_raw frequency gate. Returns the ordered
// freq results plus the fresh feed payload for the caller to cache.
func step2Fetch(
	ctx context.Context,
	feedCfg *rss.Config,
	cfg *Config,
	cands []Candidate,
	now time.Time,
) ([]freqResult, error) {
	feedCfg.FeedConfig.Hosts = append(feedCfg.FeedConfig.Hosts, cfg.HostRules...)
	existing := existingFeedURLs(feedCfg)

	// Partition: candidates with a real, non-deduped feed need a fetch slot.
	indexOf := make(map[string]int)
	needURLs := make([]string, 0, len(cands))
	for i := range cands {
		c := &cands[i]
		if c.Fatal != "" || c.Feed == "" {
			continue // handled in the result loop below
		}
		norm := normalizer(c.Feed)
		if existing[norm] {
			continue
		}
		if _, ok := indexOf[norm]; !ok {
			indexOf[norm] = len(needURLs)
			needURLs = append(needURLs, c.Feed)
		}
	}

	feeds, meta, _ := rss.FetchURLsWithMeta(ctx, needURLs, feedCfg)

	results := make([]freqResult, 0, len(cands))
	for i := range cands {
		c := &cands[i]
		if c.Fatal != "" {
			results = append(results, freqResult{Candidate: *c, Status: statusFatal, Reason: c.Note})
			continue
		}
		if c.Feed == "" {
			results = append(results, freqResult{Candidate: *c, Status: statusNoURL, Reason: "no feed URL resolved"})
			continue
		}
		if existing[normalizer(c.Feed)] {
			results = append(results, freqResult{Candidate: *c, Status: statusDup, Reason: "already subscribed in rss2nl.yml"})
			continue
		}
		slot, ok := indexOf[normalizer(c.Feed)]
		if !ok {
			results = append(results, freqResult{Candidate: *c, Status: statusNoURL, Reason: "unknown fetch slot"})
			continue
		}
		if meta[slot].Err != nil {
			results = append(results, freqResult{
				Candidate: *c,
				Status:    statusUnreach,
				Reason:    "fetch failed: " + meta[slot].Err.Message,
			})
			continue
		}
		f := feeds[slot]
		if f == nil {
			results = append(results, freqResult{Candidate: *c, Status: statusUnreach, Reason: "empty feed"})
			continue
		}
		results = append(results, freqFromFeed(c, f, cfg, now))
	}

	return results, nil
}

// freqFromFeed runs the F_raw gate (posts in the 90d window >= N, latest post
// within M days) and samples titles for Steps 3/4.
func freqFromFeed(c *Candidate, f *gofeed.Feed, cfg *Config, now time.Time) freqResult {
	r := freqResult{Candidate: *c}
	windowLow := now.AddDate(0, 0, -90)
	var latest *time.Time
	for _, it := range f.Items {
		ts := publishTime(it)
		if ts == nil {
			continue
		}
		if ts.After(windowLow) && !ts.After(now) {
			r.Posts90D++
			if latest == nil || ts.After(*latest) {
				latest = ts
			}
		}
		if it.Title != "" && len(r.Titles) < 3 {
			r.Titles = append(r.Titles, it.Title)
		}
	}
	if latest != nil {
		r.Last = latest.Format(time.RFC3339)
	}
	withinLatest := latest != nil && now.Sub(*latest) <= time.Duration(cfg.LastMaxDays)*24*time.Hour
	if r.Posts90D >= cfg.Posts90DMin && withinLatest {
		r.Status = statusPass
		r.Reason = fmt.Sprintf("passes F_raw gate (posts90d>=%d, last<=%dd)", cfg.Posts90DMin, cfg.LastMaxDays)
	} else {
		r.Status = statusFreqLow
		r.Reason = fmt.Sprintf("misses F_raw gate: posts90=%d (need>=%d), last=%s", r.Posts90D, cfg.Posts90DMin, r.Last)
	}

	return r
}
