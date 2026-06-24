package rss

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
)

func TestClassifyFeedFailureNilError(t *testing.T) {
	result := classifyFeedFailure(nil)
	assert.Equal(t, FeedFailureKindUnknown, result.Kind)
}

func TestClassifyFeedFailureKindSet(t *testing.T) {
	result := classifyFeedFailure(&FeedError{
		URL:       "https://example.com",
		Kind:      FeedFailureKindDNS,
		Transient: true,
	})
	assert.Equal(t, FeedFailureKindDNS, result.Kind)
	assert.True(t, result.Transient)
}

func TestClassifyFeedFailureEmptyURL(t *testing.T) {
	result := classifyFeedFailure(&FeedError{URL: "", Err: errors.New("empty url error")})
	assert.Equal(t, FeedFailureKindInvalidURL, result.Kind)
}

func TestClassifyFeedFailureContextCancelled(t *testing.T) {
	result := classifyFeedFailure(&FeedError{
		URL: "https://example.com",
		Err: errors.New("context canceled"),
	})
	assert.Equal(t, FeedFailureKindContextCancelled, result.Kind)
	assert.True(t, result.Transient)
}

func TestClassifyFeedFailureDNS(t *testing.T) {
	result := classifyFeedFailure(&FeedError{
		URL: "https://example.com",
		Err: errors.New("no such host"),
	})
	assert.Equal(t, FeedFailureKindDNS, result.Kind)
	assert.True(t, result.Transient)
}

func TestClassifyFeedFailureTLS(t *testing.T) {
	result := classifyFeedFailure(&FeedError{
		URL: "https://example.com",
		Err: errors.New("tls handshake error"),
	})
	assert.Equal(t, FeedFailureKindTLS, result.Kind)
	assert.True(t, result.Transient)
}

func TestClassifyFeedFailureHTTPStatus(t *testing.T) {
	tests := []struct {
		name      string
		message   string
		transient bool
	}{
		{"429 rate limit", "status code 429", true},
		{"500 server error", "HTTP 500 internal server error", true},
		{"502 bad gateway", "502 bad gateway", true},
		{"404 not found", "status: 404 not found", false},
		{"410 gone", "status code 410", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyFeedFailure(&FeedError{
				URL: "https://example.com",
				Err: errors.New(tt.message),
			})
			assert.Equal(t, FeedFailureKindHTTPStatus, result.Kind)
			assert.Equal(t, tt.transient, result.Transient)
		})
	}
}

func TestClassifyFeedFailureParse(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{"xml syntax error", "xml syntax error on line 5"},
		{"not a valid feed", "not a valid feed"},
		{"invalid feed", "invalid feed format"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyFeedFailure(&FeedError{
				URL: "https://example.com",
				Err: errors.New(tt.message),
			})
			assert.Equal(t, FeedFailureKindParse, result.Kind)
		})
	}
}

func TestClassifyFeedFailureNetwork(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{"connection reset", "connection reset by peer"},
		{"connection refused", "connection refused"},
		{"unexpected eof", "unexpected eof"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyFeedFailure(&FeedError{
				URL: "https://example.com",
				Err: errors.New(tt.message),
			})
			assert.Equal(t, FeedFailureKindNetwork, result.Kind)
			assert.True(t, result.Transient)
		})
	}
}

func TestClassifyFeedFailureUnknown(t *testing.T) {
	result := classifyFeedFailure(&FeedError{
		URL: "https://example.com",
		Err: errors.New("some random error"),
	})
	assert.Equal(t, FeedFailureKindUnknown, result.Kind)
}

func TestClassifyFeedFailureWithMessageField(t *testing.T) {
	result := classifyFeedFailure(&FeedError{
		URL:     "https://example.com",
		Message: "context deadline exceeded",
	})
	assert.Equal(t, FeedFailureKindTimeout, result.Kind)
	assert.True(t, result.Transient)
}

func TestIsTransientHTTPStatus(t *testing.T) {
	assert.True(t, isTransientHTTPStatus("status code 429"))
	assert.True(t, isTransientHTTPStatus("HTTP 500"))
	assert.True(t, isTransientHTTPStatus("502 bad gateway"))
	assert.True(t, isTransientHTTPStatus("503 service unavailable"))
	assert.True(t, isTransientHTTPStatus("504 gateway timeout"))
	assert.False(t, isTransientHTTPStatus("status code 404"))
	assert.False(t, isTransientHTTPStatus("410 gone"))
}

func TestContainsAny(t *testing.T) {
	assert.True(t, containsAny("hello world", "hello", "foo"))
	assert.True(t, containsAny("hello world", "foo", "world"))
	assert.False(t, containsAny("hello world", "foo", "bar"))
}

func TestFeedFailureMessage(t *testing.T) {
	assert.Empty(t, feedFailureMessage(nil))
	assert.Equal(t, "err msg", feedFailureMessage(&FeedError{Err: errors.New("err msg")}))
	assert.Equal(t, "fallback", feedFailureMessage(&FeedError{Message: "fallback"}))
	assert.Empty(t, feedFailureMessage(&FeedError{}))
}

func TestFeedFailureHost(t *testing.T) {
	assert.Equal(t, "example.com", feedFailureHost("https://example.com/feed.xml"))
	assert.Equal(t, "unknown", feedFailureHost("://invalid"))
	assert.Equal(t, "unknown", feedFailureHost(""))
}

func TestFeedFailureKindLabel(t *testing.T) {
	tests := []struct {
		kind FeedFailureKind
		want string
	}{
		{FeedFailureKindTimeout, "Timeout awaiting response"},
		{FeedFailureKindDNS, "DNS lookup failure"},
		{FeedFailureKindTLS, "TLS/connect failure"},
		{FeedFailureKindHTTPStatus, "HTTP status failure"},
		{FeedFailureKindParse, "Feed parse failure"},
		{FeedFailureKindInvalidURL, "Invalid feed URL"},
		{FeedFailureKindContextCancelled, "Context canceled"},
		{FeedFailureKindNetwork, "Network failure"},
		{FeedFailureKindUnknown, "Other failure"},
	}
	for _, tt := range tests {
		t.Run(string(tt.kind), func(t *testing.T) {
			assert.Equal(t, tt.want, feedFailureKindLabel(tt.kind))
		})
	}
}

func TestEffectiveFeedFailureReportConfigDefaults(t *testing.T) {
	cfg := effectiveFeedFailureReportConfig(FeedFailureReportConfig{})
	assert.Equal(t, "grouped", cfg.Mode)
	assert.Equal(t, DefaultFeedFailureExampleLimit, cfg.ExampleLimit)
	assert.Equal(t, DefaultTransientEscalateAfterRuns, cfg.TransientEscalateAfterRuns)
	assert.NotNil(t, cfg.SuppressTransientUntilEscalated)
	assert.True(t, *cfg.SuppressTransientUntilEscalated)
	assert.Equal(t, DefaultFeedFailureStatePath, cfg.StatePath)
}

func TestEffectiveFeedFailureReportConfigPreservesExplicitValues(t *testing.T) {
	suppress := false
	cfg := effectiveFeedFailureReportConfig(FeedFailureReportConfig{
		Mode:                            "flat",
		ExampleLimit:                    10,
		TransientEscalateAfterRuns:      5,
		SuppressTransientUntilEscalated: &suppress,
		StatePath:                       "/tmp/state.json",
	})
	assert.Equal(t, "flat", cfg.Mode)
	assert.Equal(t, 10, cfg.ExampleLimit)
	assert.Equal(t, 5, cfg.TransientEscalateAfterRuns)
	assert.False(t, *cfg.SuppressTransientUntilEscalated)
	assert.Equal(t, "/tmp/state.json", cfg.StatePath)
}

func TestBuildFeedFailureReportPersistenceAcrossRuns(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	cfg := FeedFailureReportConfig{StatePath: statePath, ExampleLimit: 2}
	now := time.Date(2026, 6, 20, 8, 0, 0, 0, time.UTC)

	// Run 1: two failures
	report1, err := BuildFeedFailureReport([]*FeedError{
		{URL: "https://a.com/feed", Err: errors.New("connection refused")},
		{URL: "https://b.com/feed", Err: errors.New("500 internal server error")},
	}, cfg, now)
	require.NoError(t, err)
	assert.Equal(t, 2, report1.TotalFailures)

	// Run 2: same failures persist
	report2, err := BuildFeedFailureReport([]*FeedError{
		{URL: "https://a.com/feed", Err: errors.New("connection refused")},
	}, cfg, now.Add(24*time.Hour))
	require.NoError(t, err)
	assert.Equal(t, 1, report2.TotalFailures)
}

func TestBuildFeedFailureReportNilAndEmptyURLFiltered(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	report, err := BuildFeedFailureReport([]*FeedError{
		nil,
		{URL: "", Err: errors.New("empty")},
		{URL: "https://a.com/feed", Err: errors.New("500")},
	}, FeedFailureReportConfig{StatePath: statePath}, time.Now())
	require.NoError(t, err)
	assert.Equal(t, 1, report.TotalFailures)
}

func TestBuildGroupedFailureReportMultipleGroups(t *testing.T) {
	failures := []classifiedFeedFailure{
		{URL: "https://a.com/1", Host: "a.com", Kind: FeedFailureKindTimeout, Transient: true},
		{URL: "https://a.com/2", Host: "a.com", Kind: FeedFailureKindTimeout, Transient: true},
		{URL: "https://b.com/1", Host: "b.com", Kind: FeedFailureKindParse, Transient: false},
	}
	cfg := FeedFailureReportConfig{
		ExampleLimit:               5,
		TransientEscalateAfterRuns: 1,
	}
	suppress := true
	cfg.SuppressTransientUntilEscalated = &suppress

	report := buildGroupedFailureReport(failures, cfg)
	assert.Equal(t, 3, report.TotalFailures)
	assert.Len(t, report.Groups, 2)
}

func TestSortFeedFailureGroups(t *testing.T) {
	groups := []FeedFailureGroup{
		{Host: "b.com", Kind: FeedFailureKindTimeout, Transient: true},
		{Host: "a.com", Kind: FeedFailureKindParse, Transient: false, Escalated: true},
		{Host: "a.com", Kind: FeedFailureKindTimeout, Transient: true},
	}
	sortFeedFailureGroups(groups)
	// Escalated first, then non-transient, then by host
	assert.True(t, groups[0].Escalated)
}

func TestSortFeedFailureGroupsNonTransientFirst(t *testing.T) {
	groups := []FeedFailureGroup{
		{Host: "a.com", Kind: FeedFailureKindTimeout, Transient: true},
		{Host: "a.com", Kind: FeedFailureKindParse, Transient: false},
	}
	sortFeedFailureGroups(groups)
	assert.False(t, groups[0].Transient)
}

func TestSortFeedFailureGroupsSameHostDifferentKind(t *testing.T) {
	groups := []FeedFailureGroup{
		{Host: "a.com", Kind: FeedFailureKindNetwork, Transient: true},
		{Host: "a.com", Kind: FeedFailureKindTimeout, Transient: true},
	}
	sortFeedFailureGroups(groups)
	assert.Equal(t, FeedFailureKindNetwork, groups[0].Kind) // network < timeout alphabetically
}

func TestFeedFailureGroupStatusPersistent(t *testing.T) {
	group := &FeedFailureGroup{Transient: false}
	assert.Equal(t, "persistent", feedFailureGroupStatus(group, 3))
}

func TestFeedFailureGroupStatusEscalated(t *testing.T) {
	group := &FeedFailureGroup{Transient: true, Escalated: true, MaxConsecutiveRuns: 5}
	assert.Contains(t, feedFailureGroupStatus(group, 3), "escalated after 5")
}

func TestFeedFailureGroupStatusSuppressed(t *testing.T) {
	group := &FeedFailureGroup{Transient: true, Escalated: false}
	assert.Contains(t, feedFailureGroupStatus(group, 3), "suppressed today")
}

func TestLoadAndSaveFeedFailureState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")

	// Load from non-existent file
	state, err := loadFeedFailureState(path)
	require.NoError(t, err)
	assert.NotNil(t, state.Feeds)
	assert.Empty(t, state.Feeds)

	// Save state
	state.Feeds["https://a.com"] = feedFailureStateItem{
		URL: "https://a.com", Kind: FeedFailureKindTimeout, ConsecutiveRuns: 3,
	}
	err = saveFeedFailureState(path, state)
	require.NoError(t, err)

	// Reload state
	loaded, err := loadFeedFailureState(path)
	require.NoError(t, err)
	assert.Len(t, loaded.Feeds, 1)
	assert.Equal(t, 3, loaded.Feeds["https://a.com"].ConsecutiveRuns)
}

func TestLoadFeedFailureStateInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	require.NoError(t, os.WriteFile(path, []byte("not json"), 0o600))
	_, err := loadFeedFailureState(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse feed failure state")
}

func TestReadFeedFailureStateDataPermissionError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "unreadable.json")
	require.NoError(t, os.WriteFile(path, []byte("{}"), 0o000))
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
	_, err := readFeedFailureStateData(path)
	require.Error(t, err)
}

func TestReadLegacyFeedFailureStateDataNonDefaultPath(t *testing.T) {
	data, err := readLegacyFeedFailureStateData("/tmp/custom-state.json")
	require.NoError(t, err)
	assert.Nil(t, data)
}

func TestReadLegacyFeedFailureStateDataSamePath(t *testing.T) {
	// When legacy path equals the requested path, return nil
	legacyPath := fileutil.LegacyCachePath("rss2nl/feed-failures.json")
	data, err := readLegacyFeedFailureStateData(legacyPath)
	require.NoError(t, err)
	assert.Nil(t, data)
}

func TestLoadFeedFailureStateNilFeeds(t *testing.T) {
	// Write valid JSON without "feeds" key to test nil Feeds initialization
	path := filepath.Join(t.TempDir(), "state.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"version": 1}`), 0o600))

	state, err := loadFeedFailureState(path)
	require.NoError(t, err)
	assert.NotNil(t, state.Feeds, "Feeds should be initialized even when nil in JSON")
}

func TestBuildFeedFailureReportSaveError(t *testing.T) {
	// Use an invalid path for saving to trigger saveFeedFailureState error
	report, err := BuildFeedFailureReport(
		[]*FeedError{
			{URL: "https://a.com/feed", Err: errors.New("500")},
		},
		FeedFailureReportConfig{StatePath: "/nonexistent/dir/state.json"},
		time.Now(),
	)
	// Report should still be returned even if save fails
	require.NotNil(t, report)
	require.Error(t, err)
}

func TestSortFeedFailureGroupsTransientVsNonTransientSameHost(t *testing.T) {
	groups := []FeedFailureGroup{
		{Host: "a.com", Kind: FeedFailureKindTimeout, Transient: true},
		{Host: "a.com", Kind: FeedFailureKindParse, Transient: false},
		{Host: "a.com", Kind: FeedFailureKindDNS, Transient: true},
	}
	sortFeedFailureGroups(groups)
	// Non-transient should come first
	assert.False(t, groups[0].Transient)
}

func TestBuildGroupedFailureReportSameKindSameHost(t *testing.T) {
	failures := []classifiedFeedFailure{
		{URL: "https://a.com/1", Host: "a.com", Kind: FeedFailureKindTimeout, Transient: true, ConsecutiveRuns: 2},
		{URL: "https://a.com/2", Host: "a.com", Kind: FeedFailureKindTimeout, Transient: true, ConsecutiveRuns: 5},
	}
	cfg := FeedFailureReportConfig{
		ExampleLimit:               5,
		TransientEscalateAfterRuns: 3,
	}
	suppress := true
	cfg.SuppressTransientUntilEscalated = &suppress

	report := buildGroupedFailureReport(failures, cfg)
	require.Len(t, report.Groups, 1)
	assert.Equal(t, 5, report.Groups[0].MaxConsecutiveRuns)
	assert.True(t, report.Groups[0].Escalated)
}

func TestBuildGroupedFailureReportSameHostDifferentKinds(t *testing.T) {
	failures := []classifiedFeedFailure{
		{URL: "https://a.com/1", Host: "a.com", Kind: FeedFailureKindTimeout, Transient: true},
		{URL: "https://a.com/2", Host: "a.com", Kind: FeedFailureKindDNS, Transient: true},
	}
	cfg := FeedFailureReportConfig{
		ExampleLimit:               5,
		TransientEscalateAfterRuns: 3,
	}
	suppress := true
	cfg.SuppressTransientUntilEscalated = &suppress

	report := buildGroupedFailureReport(failures, cfg)
	assert.Len(t, report.Groups, 2)
}

func TestLoadFeedFailureStateReadError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unreadable.json")
	require.NoError(t, os.WriteFile(path, []byte("{}"), 0o000))
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })

	_, err := loadFeedFailureState(path)
	require.Error(t, err)
}
