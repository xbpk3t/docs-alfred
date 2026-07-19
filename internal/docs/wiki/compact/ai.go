package compact

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/xbpk3t/docs-alfred/internal/docs/wiki/blog"
	"github.com/xbpk3t/docs-alfred/internal/docs/wiki/prompt"

	"github.com/avast/retry-go/v4"
	carbon "github.com/dromara/carbon/v2"
	"github.com/samber/lo"
	"github.com/xbpk3t/docs-alfred/pkg/ai"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
	"github.com/xbpk3t/docs-alfred/pkg/validator"
)

// CompactRecommend is the AI JSON schema for one topic.
type CompactRecommend struct {
	Recommend      string   `json:"recommend" validate:"required|in:yes,no"`
	SuggestedAngle string   `json:"suggested_angle"`
	DuplicateOf    string   `json:"duplicate_of_existing_blog,omitempty"`
	Error          string   `json:"-"`
	Why            []string `json:"why"`
	BlogTitles     []string `json:"-"`
	Topic          HotTopic `json:"-"`
	SkippedCooling bool     `json:"-"`
}

const (
	defaultLogBudgetRunes  = 12000
	defaultDiffBudgetRunes = 8000
	compactAITimeout       = 90 * time.Second
	compactAIAttempts      = 3
)

type compactPromptData struct {
	TopicPath   string
	LogPath     string
	LastEdit    string
	Diff        string
	LogBody     string
	BlogTitles  []string
	EditDays    int
	EditCommits int
	DeltaChars  int
	DeltaLines  int
	Score       int
}

// JudgeHotTopics runs AI recommend for each hot topic (≤ topHot already applied).
// On per-topic failure, Recommend is forced to "no" with Error set.
// If AI is globally unavailable (no key / every call fails with transport-level
// errors), returns ok=false so caller can send hot-list mail.
func JudgeHotTopics(ctx context.Context, cfg *ai.ClientConfig, topics []HotTopic) (results []CompactRecommend, aiOK bool) {
	if cfg == nil || cfg.APIKey == "" {
		return nil, false
	}
	results = make([]CompactRecommend, len(topics))
	var wg sync.WaitGroup
	var mu sync.Mutex
	successes := 0

	for i := range topics {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r := judgeOne(ctx, cfg, &topics[i])
			mu.Lock()
			results[i] = r
			if r.Error == "" {
				successes++
			}
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	if len(topics) > 0 && successes == 0 {
		return results, false
	}
	return results, true
}

func judgeOne(ctx context.Context, cfg *ai.ClientConfig, ht *HotTopic) CompactRecommend {
	out := CompactRecommend{Topic: *ht, Recommend: "no"}

	topicAbs := ht.TopicDir
	blogs, _ := blog.ListTopicBlogs(topicAbs)
	out.BlogTitles = blog.BlogTitles(blogs)

	logBody := ""
	if data, err := os.ReadFile(filepath.Join(topicAbs, "log.md")); err == nil {
		logBody = truncateTail(string(data), defaultLogBudgetRunes)
	}
	diff := MergedDiff(ht, defaultDiffBudgetRunes)

	promptText, err := prompt.Render("compact.txt", compactPromptData{
		TopicPath:   ht.TopicPath,
		LogPath:     ht.LogPath,
		EditDays:    ht.EditDays,
		EditCommits: ht.EditCommits,
		DeltaChars:  ht.DeltaChars,
		DeltaLines:  ht.DeltaLines,
		LastEdit:    carbon.CreateFromStdTime(ht.LastEdit).ToDateTimeString(),
		Score:       ht.Score,
		BlogTitles:  out.BlogTitles,
		Diff:        diff,
		LogBody:     logBody,
	})
	if err != nil {
		out.Error = err.Error()
		return out
	}

	callCfg := *cfg
	if callCfg.Timeout <= 0 {
		callCfg.Timeout = compactAITimeout
	}

	var parsed CompactRecommend
	err = retry.Do(
		func() error {
			raw, e := ai.ChatContext(ctx, &callCfg, []ai.Message{
				{Role: ai.RoleUser, Content: promptText},
			})
			if e != nil {
				return fmt.Errorf("AI compact call: %w", e)
			}
			r, e := parseCompactJSON(raw)
			if e != nil {
				return fmt.Errorf("parse compact JSON: %w", e)
			}
			parsed = r
			return nil
		},
		retry.Attempts(compactAIAttempts),
		retry.Delay(1*time.Second),
		retry.DelayType(retry.BackOffDelay),
		retry.Context(ctx),
	)
	if err != nil {
		out.Error = err.Error()
		out.Recommend = "no"
		return out
	}

	out.Recommend = parsed.Recommend
	if out.Recommend != "yes" {
		out.Recommend = "no"
	}
	out.Why = parsed.Why
	out.SuggestedAngle = parsed.SuggestedAngle
	out.DuplicateOf = parsed.DuplicateOf
	return out
}

func parseCompactJSON(raw string) (CompactRecommend, error) {
	r, err := fileutil.UnmarshalJSON[CompactRecommend]([]byte(strings.TrimSpace(raw)))
	if err != nil {
		return CompactRecommend{}, err
	}
	// Normalize before Struct so Yes/NO match in:yes,no (same as prior hand validation).
	r.Recommend = strings.ToLower(strings.TrimSpace(r.Recommend))
	if err := validator.Struct(&r); err != nil {
		return CompactRecommend{}, err
	}
	return r, nil
}

// SelectNotices picks up to topNotice yes recommendations in input order (already score-sorted).
func SelectNotices(results []CompactRecommend, topNotice int) []CompactRecommend {
	if topNotice <= 0 {
		topNotice = 5
	}
	yes := lo.Filter(results, func(r CompactRecommend, _ int) bool {
		return !r.SkippedCooling && strings.EqualFold(r.Recommend, "yes")
	})
	if len(yes) <= topNotice {
		return yes
	}
	return lo.Subset(yes, 0, uint(topNotice))
}

// truncateTail keeps the tail of s (recent notes are usually appended).
func truncateTail(s string, maxRunes int) string {
	if maxRunes <= 0 || utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	return "... (log head truncated)\n" + string(runes[len(runes)-maxRunes:])
}
