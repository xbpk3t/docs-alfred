package rss

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultFeedFailureStatePath            = ".cache/rss2nl/feed-failures.json"
	DefaultFeedFailureExampleLimit         = 5
	DefaultTransientEscalateAfterRuns      = 3
	DefaultSuppressTransientUntilEscalated = true
	defaultFeedFailureReportConfigMode     = "grouped"
	feedFailureStateVersion                = 1
	feedFailureKindTimeout                 = "timeout"
	feedFailureKindDNS                     = "dns"
	feedFailureKindTLS                     = "tls"
	feedFailureKindHTTPStatus              = "http_status"
	feedFailureKindParse                   = "parse"
	feedFailureKindInvalidURL              = "invalid_url"
	feedFailureKindContextCancelled        = "context_canceled"
	feedFailureKindNetwork                 = "network"
	feedFailureKindUnknown                 = "unknown"
)

// FeedFailureKind is a coarse class for grouping and routing feed failures.
type FeedFailureKind string

const (
	FeedFailureKindTimeout          FeedFailureKind = feedFailureKindTimeout
	FeedFailureKindDNS              FeedFailureKind = feedFailureKindDNS
	FeedFailureKindTLS              FeedFailureKind = feedFailureKindTLS
	FeedFailureKindHTTPStatus       FeedFailureKind = feedFailureKindHTTPStatus
	FeedFailureKindParse            FeedFailureKind = feedFailureKindParse
	FeedFailureKindInvalidURL       FeedFailureKind = feedFailureKindInvalidURL
	FeedFailureKindContextCancelled FeedFailureKind = feedFailureKindContextCancelled
	FeedFailureKindNetwork          FeedFailureKind = feedFailureKindNetwork
	FeedFailureKindUnknown          FeedFailureKind = feedFailureKindUnknown
)

// FeedFailureReportConfig controls how failed feeds are grouped in the dashboard.
type FeedFailureReportConfig struct {
	SuppressTransientUntilEscalated *bool  `yaml:"suppressTransientUntilEscalated,omitempty"`
	Mode                            string `yaml:"mode,omitempty"`
	StatePath                       string `yaml:"statePath,omitempty"`
	ExampleLimit                    int    `yaml:"exampleLimit,omitempty"`
	TransientEscalateAfterRuns      int    `yaml:"transientEscalateAfterRuns,omitempty"`
}

// FeedFailureReport is the rendered dashboard model for fetch failures.
type FeedFailureReport struct {
	StatePath          string
	Groups             []FeedFailureGroup
	TotalFailures      int
	TransientFailures  int
	PersistentFailures int
	EscalatedFailures  int
	SuppressedFailures int
	ExampleLimit       int
	EscalateAfterRuns  int
}

// FeedFailureGroup is one grouped row in the failed-feed dashboard.
type FeedFailureGroup struct {
	Host               string
	Kind               FeedFailureKind
	KindLabel          string
	Status             string
	ExampleURLs        []string
	Count              int
	MaxConsecutiveRuns int
	RemainingCount     int
	Transient          bool
	Escalated          bool
	Suppressed         bool
}

type feedFailureState struct {
	Feeds   map[string]feedFailureStateItem `json:"feeds"`
	Version int                             `json:"version"`
}

type feedFailureStateItem struct {
	FirstFailedAt   time.Time       `json:"firstFailedAt"`
	LastFailedAt    time.Time       `json:"lastFailedAt"`
	URL             string          `json:"url"`
	Kind            FeedFailureKind `json:"kind"`
	Message         string          `json:"message,omitempty"`
	ConsecutiveRuns int             `json:"consecutiveRuns"`
	Transient       bool            `json:"transient"`
}

type classifiedFeedFailure struct {
	URL             string
	Host            string
	Message         string
	Kind            FeedFailureKind
	Transient       bool
	ConsecutiveRuns int
}

// BuildFeedFailureReport groups failed feeds and updates consecutive-failure state.
func BuildFeedFailureReport(
	failedFeeds []*FeedError,
	cfg FeedFailureReportConfig,
	now time.Time,
) (*FeedFailureReport, error) {
	effective := effectiveFeedFailureReportConfig(cfg)
	var reportErr error
	state, err := loadFeedFailureState(effective.StatePath)
	if err != nil {
		reportErr = err
		state = feedFailureState{
			Feeds: make(map[string]feedFailureStateItem),
		}
	}

	failedByURL := make(map[string]*FeedError, len(failedFeeds))
	for _, feedErr := range failedFeeds {
		if feedErr == nil || feedErr.URL == "" {
			continue
		}
		failedByURL[feedErr.URL] = feedErr
	}

	nextState := feedFailureState{
		Version: feedFailureStateVersion,
		Feeds:   make(map[string]feedFailureStateItem),
	}

	classified := make([]classifiedFeedFailure, 0, len(failedByURL))
	for feedURL, feedErr := range failedByURL {
		classification := classifyFeedFailure(feedErr)
		previous := state.Feeds[feedURL]
		firstFailedAt := now
		if !previous.FirstFailedAt.IsZero() && previous.Kind == classification.Kind {
			firstFailedAt = previous.FirstFailedAt
		}
		consecutiveRuns := 1
		if previous.Kind == classification.Kind {
			consecutiveRuns = previous.ConsecutiveRuns + 1
		}

		message := feedFailureMessage(feedErr)
		nextState.Feeds[feedURL] = feedFailureStateItem{
			URL:             feedURL,
			Kind:            classification.Kind,
			Message:         message,
			Transient:       classification.Transient,
			ConsecutiveRuns: consecutiveRuns,
			FirstFailedAt:   firstFailedAt,
			LastFailedAt:    now,
		}

		classified = append(classified, classifiedFeedFailure{
			URL:             feedURL,
			Host:            feedFailureHost(feedURL),
			Message:         message,
			Kind:            classification.Kind,
			Transient:       classification.Transient,
			ConsecutiveRuns: consecutiveRuns,
		})
	}

	if err := saveFeedFailureState(effective.StatePath, nextState); err != nil {
		reportErr = err
	}

	report := buildGroupedFailureReport(classified, effective)
	report.StatePath = effective.StatePath

	return report, reportErr
}

// classifyFeedFailure returns stable grouping metadata for a feed error.
func classifyFeedFailure(feedErr *FeedError) feedFailureStateItem {
	message := strings.ToLower(feedFailureMessage(feedErr))
	if feedErr == nil {
		return feedFailureStateItem{Kind: FeedFailureKindUnknown}
	}
	if feedErr.Kind != "" {
		return feedFailureStateItem{
			Kind:      feedErr.Kind,
			Transient: feedErr.Transient,
		}
	}
	if strings.TrimSpace(feedErr.URL) == "" || strings.Contains(message, "empty url") {
		return feedFailureStateItem{Kind: FeedFailureKindInvalidURL}
	}
	if item, ok := classifyTransientInfrastructureFailure(message); ok {
		return item
	}
	if item, ok := classifyHTTPStatusFailure(message); ok {
		return item
	}
	if item, ok := classifyContentFailure(message); ok {
		return item
	}

	return feedFailureStateItem{Kind: FeedFailureKindUnknown}
}

func classifyTransientInfrastructureFailure(message string) (feedFailureStateItem, bool) {
	if containsAny(message, "context canceled", "context canceled") {
		return feedFailureStateItem{Kind: FeedFailureKindContextCancelled, Transient: true}, true
	}
	if containsAny(message,
		"context deadline exceeded",
		"client.timeout exceeded",
		"i/o timeout",
		"timeout",
		"deadline exceeded",
	) {
		return feedFailureStateItem{Kind: FeedFailureKindTimeout, Transient: true}, true
	}
	if containsAny(message,
		"no such host",
		"temporary failure in name resolution",
		"server misbehaving",
		"lookup ",
	) {
		return feedFailureStateItem{Kind: FeedFailureKindDNS, Transient: true}, true
	}
	if containsAny(message,
		"tls",
		"ssl routines",
		"certificate",
		"handshake",
	) {
		return feedFailureStateItem{Kind: FeedFailureKindTLS, Transient: true}, true
	}

	return feedFailureStateItem{}, false
}

func classifyHTTPStatusFailure(message string) (feedFailureStateItem, bool) {
	if containsAny(message,
		"status code",
		"status:",
		"429",
		"500",
		"502",
		"503",
		"504",
		"404",
		"410",
	) {
		return feedFailureStateItem{
			Kind:      FeedFailureKindHTTPStatus,
			Transient: isTransientHTTPStatus(message),
		}, true
	}

	return feedFailureStateItem{}, false
}

func classifyContentFailure(message string) (feedFailureStateItem, bool) {
	if containsAny(message,
		"xml syntax error",
		"not a valid feed",
		"not a feed",
		"failed to detect feed type",
		"invalid feed",
	) {
		return feedFailureStateItem{Kind: FeedFailureKindParse}, true
	}
	if containsAny(message,
		"unexpected eof",
		"connection reset",
		"connection refused",
		"network is unreachable",
		"eof",
	) {
		return feedFailureStateItem{Kind: FeedFailureKindNetwork, Transient: true}, true
	}

	return feedFailureStateItem{}, false
}

func effectiveFeedFailureReportConfig(cfg FeedFailureReportConfig) FeedFailureReportConfig {
	if cfg.Mode == "" {
		cfg.Mode = defaultFeedFailureReportConfigMode
	}
	if cfg.ExampleLimit <= 0 {
		cfg.ExampleLimit = DefaultFeedFailureExampleLimit
	}
	if cfg.TransientEscalateAfterRuns <= 0 {
		cfg.TransientEscalateAfterRuns = DefaultTransientEscalateAfterRuns
	}
	if cfg.SuppressTransientUntilEscalated == nil {
		value := DefaultSuppressTransientUntilEscalated
		cfg.SuppressTransientUntilEscalated = &value
	}
	if cfg.StatePath == "" {
		cfg.StatePath = DefaultFeedFailureStatePath
	}

	return cfg
}

func buildGroupedFailureReport(
	failures []classifiedFeedFailure,
	cfg FeedFailureReportConfig,
) *FeedFailureReport {
	sort.Slice(failures, func(i, j int) bool {
		if failures[i].Host != failures[j].Host {
			return failures[i].Host < failures[j].Host
		}
		if failures[i].Kind != failures[j].Kind {
			return failures[i].Kind < failures[j].Kind
		}

		return failures[i].URL < failures[j].URL
	})

	grouped := make(map[string]*FeedFailureGroup)
	for i := range failures {
		addFeedFailureToGroup(grouped, &failures[i], cfg.ExampleLimit)
	}

	report := &FeedFailureReport{
		TotalFailures:     len(failures),
		ExampleLimit:      cfg.ExampleLimit,
		EscalateAfterRuns: cfg.TransientEscalateAfterRuns,
	}

	groups := make([]FeedFailureGroup, 0, len(grouped))
	for _, group := range grouped {
		applyFeedFailureGroupState(group, cfg)
		updateFeedFailureReportCounts(report, group)
		groups = append(groups, *group)
	}

	sortFeedFailureGroups(groups)
	report.Groups = groups

	return report
}

func addFeedFailureToGroup(grouped map[string]*FeedFailureGroup, failure *classifiedFeedFailure, exampleLimit int) {
	key := failure.Host + "\x00" + string(failure.Kind)
	group := grouped[key]
	if group == nil {
		group = &FeedFailureGroup{
			Host:        failure.Host,
			Kind:        failure.Kind,
			KindLabel:   feedFailureKindLabel(failure.Kind),
			Transient:   failure.Transient,
			ExampleURLs: make([]string, 0, exampleLimit),
		}
		grouped[key] = group
	}

	group.Count++
	if failure.ConsecutiveRuns > group.MaxConsecutiveRuns {
		group.MaxConsecutiveRuns = failure.ConsecutiveRuns
	}
	if len(group.ExampleURLs) < exampleLimit {
		group.ExampleURLs = append(group.ExampleURLs, failure.URL)
	} else {
		group.RemainingCount++
	}
}

func updateFeedFailureReportCounts(report *FeedFailureReport, group *FeedFailureGroup) {
	if group.Transient {
		report.TransientFailures += group.Count
	} else {
		report.PersistentFailures += group.Count
	}
	if group.Escalated {
		report.EscalatedFailures += group.Count
	}
	if group.Suppressed {
		report.SuppressedFailures += group.Count
	}
}

func sortFeedFailureGroups(groups []FeedFailureGroup) {
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].Escalated != groups[j].Escalated {
			return groups[i].Escalated
		}
		if groups[i].Transient != groups[j].Transient {
			return !groups[i].Transient
		}
		if groups[i].Host != groups[j].Host {
			return groups[i].Host < groups[j].Host
		}

		return groups[i].Kind < groups[j].Kind
	})
}

func applyFeedFailureGroupState(group *FeedFailureGroup, cfg FeedFailureReportConfig) {
	group.Escalated = !group.Transient || group.MaxConsecutiveRuns >= cfg.TransientEscalateAfterRuns
	group.Suppressed = group.Transient &&
		!group.Escalated &&
		cfg.SuppressTransientUntilEscalated != nil &&
		*cfg.SuppressTransientUntilEscalated
	group.Status = feedFailureGroupStatus(group, cfg.TransientEscalateAfterRuns)
}

func feedFailureGroupStatus(group *FeedFailureGroup, escalateAfterRuns int) string {
	if !group.Transient {
		return "persistent"
	}
	if group.Escalated {
		return fmt.Sprintf("escalated after %d consecutive runs", group.MaxConsecutiveRuns)
	}

	return fmt.Sprintf(
		"suppressed today, escalates after %d consecutive runs",
		escalateAfterRuns,
	)
}

func feedFailureKindLabel(kind FeedFailureKind) string {
	switch kind {
	case FeedFailureKindTimeout:
		return "Timeout awaiting response"
	case FeedFailureKindDNS:
		return "DNS lookup failure"
	case FeedFailureKindTLS:
		return "TLS/connect failure"
	case FeedFailureKindHTTPStatus:
		return "HTTP status failure"
	case FeedFailureKindParse:
		return "Feed parse failure"
	case FeedFailureKindInvalidURL:
		return "Invalid feed URL"
	case FeedFailureKindContextCancelled:
		return "Context canceled"
	case FeedFailureKindNetwork:
		return "Network failure"
	default:
		return "Other failure"
	}
}

func feedFailureMessage(feedErr *FeedError) string {
	if feedErr == nil {
		return ""
	}
	if feedErr.Err != nil {
		return feedErr.Err.Error()
	}

	return feedErr.Message
}

func feedFailureHost(feedURL string) string {
	parsed, err := url.Parse(feedURL)
	if err != nil || parsed.Host == "" {
		return "unknown"
	}

	return strings.ToLower(parsed.Host)
}

func isTransientHTTPStatus(message string) bool {
	for _, status := range []int{429, 500, 502, 503, 504} {
		if strings.Contains(message, strconv.Itoa(status)) {
			return true
		}
	}

	return false
}

func containsAny(s string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(s, needle) {
			return true
		}
	}

	return false
}

func loadFeedFailureState(path string) (feedFailureState, error) {
	state := feedFailureState{
		Version: feedFailureStateVersion,
		Feeds:   make(map[string]feedFailureStateItem),
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return state, fmt.Errorf("read feed failure state: %w", err)
	}
	if len(data) == 0 {
		return state, nil
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return feedFailureState{}, fmt.Errorf("parse feed failure state: %w", err)
	}
	if state.Feeds == nil {
		state.Feeds = make(map[string]feedFailureStateItem)
	}

	return state, nil
}

func saveFeedFailureState(path string, state feedFailureState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create feed failure state dir: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal feed failure state: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write feed failure state: %w", err)
	}

	return nil
}
