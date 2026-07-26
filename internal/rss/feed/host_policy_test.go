package rss

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveHostRuleExactMatch(t *testing.T) {
	cfg := FeedConfig{
		HostDefaultConcurrency: 4,
		Hosts: []FeedHostRule{
			{Match: "rss-worker.yyzw.workers.dev", Concurrency: 1, MinIntervalMs: 1500},
		},
	}
	rule := cfg.resolveHostRule("RSS-WORKER.yyzw.workers.dev")
	assert.True(t, rule.hasExplicit)
	assert.Equal(t, int64(1), rule.concurrency)
	assert.Equal(t, 1500*time.Millisecond, rule.minInterval)
	assert.True(t, rule.circuitOn5xx)

	other := cfg.resolveHostRule("example.com")
	assert.False(t, other.hasExplicit)
	assert.Equal(t, int64(4), other.concurrency)
	assert.False(t, other.circuitOn5xx)
}

func TestIsRiskControlAndRetryClassification(t *testing.T) {
	assert.True(t, isRiskControlFetchError(errors.New("gRPC status 2: -352")))
	assert.True(t, isRiskControlFetchError(errors.New("Error: 风控校验失败")))
	assert.False(t, isFastRetryableFetchError(errors.New("gRPC status 2: -352")))
	assert.True(t, isFastRetryableFetchError(errors.New("http error: 503")))
	assert.False(t, isFastRetryableFetchError(errors.New("http error: 404")))

	// Typed gofeed.HTTPError path (no string scraping).
	assert.True(t, isFastRetryableFetchError(gofeed.HTTPError{StatusCode: 503, Status: "503 Service Unavailable"}))
	assert.False(t, isFastRetryableFetchError(gofeed.HTTPError{StatusCode: 404, Status: "404 Not Found"}))
	assert.True(t, isHTTP5xxFetchError(gofeed.HTTPError{StatusCode: 500, Status: "500 Internal Server Error"}))
}

func TestShouldTripHostCircuit(t *testing.T) {
	explicit := resolvedHostRule{hasExplicit: true, circuitOn5xx: true}
	assert.True(t, shouldTripHostCircuit(explicit, gofeed.HTTPError{StatusCode: 500, Status: "500 Internal Server Error"}))
	assert.True(t, shouldTripHostCircuit(explicit, errors.New("gRPC status 2: -352")))

	plain := resolvedHostRule{hasExplicit: false, circuitOn5xx: false}
	assert.False(t, shouldTripHostCircuit(plain, gofeed.HTTPError{StatusCode: 500, Status: "500 Internal Server Error"}))
	assert.True(t, shouldTripHostCircuit(plain, errors.New("-352 risk")))
}

func TestHostFetchControllerCircuitSkipsRetry(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		http.Error(w, "gRPC status 2: -352", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	cfg := &Config{FeedConfig: FeedConfig{
		Timeout:                5,
		MaxTries:               5,
		Concurrency:            4,
		HostDefaultConcurrency: 4,
		Hosts: []FeedHostRule{
			{Match: hostnameOf(server.URL), Concurrency: 1},
		},
	}}

	urls := []string{
		server.URL + "/a.xml",
		server.URL + "/b.xml",
		server.URL + "/c.xml",
	}
	_, _, failed := FetchURLsWithMeta(context.Background(), urls, cfg)
	require.Len(t, failed, 3)
	// First request may hit once; remaining should short-circuit without full retry storms.
	// With circuit after first 5xx, total upstream hits should be << maxTries*len(urls)=15.
	assert.LessOrEqual(t, int(hits.Load()), 3)
	for _, f := range failed {
		assert.True(t, f.Transient, f.Message)
	}
}

func TestHostFetchControllerSerializesSameHost(t *testing.T) {
	var inFlight atomic.Int32
	var maxInFlight atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		cur := inFlight.Add(1)
		for {
			old := maxInFlight.Load()
			if cur <= old || maxInFlight.CompareAndSwap(old, cur) {
				break
			}
		}
		time.Sleep(40 * time.Millisecond)
		inFlight.Add(-1)
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0"?><rss version="2.0"><channel><title>t</title></channel></rss>`))
	}))
	t.Cleanup(server.Close)

	host := hostnameOf(server.URL)
	cfg := &Config{FeedConfig: FeedConfig{
		Timeout:                5,
		MaxTries:               1,
		Concurrency:            10,
		HostDefaultConcurrency: 10,
		Hosts: []FeedHostRule{
			{Match: host, Concurrency: 1},
		},
	}}
	urls := []string{server.URL + "/1", server.URL + "/2", server.URL + "/3"}
	feeds, _, failed := FetchURLsWithMeta(context.Background(), urls, cfg)
	require.Empty(t, failed)
	require.Len(t, feeds, 3)
	assert.Equal(t, int32(1), maxInFlight.Load())
}

func TestFeedConfigValidateHosts(t *testing.T) {
	cfg := &Config{FeedConfig: FeedConfig{
		Hosts: []FeedHostRule{{Match: ""}, {Match: "a.com"}, {Match: "a.com"}},
	}}
	err := cfg.FeedConfig.validateHosts()
	require.Error(t, err)
}
