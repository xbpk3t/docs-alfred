package rss

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/mmcdole/gofeed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInspectFetchErrorTypedHTTP(t *testing.T) {
	info := inspectFetchError(gofeed.HTTPError{StatusCode: 503, Status: "503 Service Unavailable"})
	assert.Equal(t, FeedFailureKindHTTPStatus, info.Kind)
	assert.Equal(t, 503, info.StatusCode)
	assert.True(t, info.Transient)
	assert.True(t, info.FastRetryable)
	assert.True(t, info.Is5xx)

	info404 := inspectFetchError(gofeed.HTTPError{StatusCode: 404, Status: "404 Not Found"})
	assert.Equal(t, FeedFailureKindHTTPStatus, info404.Kind)
	assert.False(t, info404.Transient)
	assert.False(t, info404.FastRetryable)
	assert.False(t, info404.Is5xx)
}

func TestInspectFetchErrorContextAndDeadline(t *testing.T) {
	info := inspectFetchError(context.Canceled)
	assert.Equal(t, FeedFailureKindContextCancelled, info.Kind)
	assert.True(t, info.Transient)
	assert.False(t, info.FastRetryable)

	infoTO := inspectFetchError(context.DeadlineExceeded)
	assert.Equal(t, FeedFailureKindTimeout, infoTO.Kind)
	assert.True(t, infoTO.FastRetryable)
}

func TestInspectFetchErrorDNS(t *testing.T) {
	info := inspectFetchError(&net.DNSError{Err: "no such host", Name: "nope.invalid", IsNotFound: true})
	assert.Equal(t, FeedFailureKindDNS, info.Kind)
	assert.True(t, info.Transient)
	assert.True(t, info.FastRetryable)
}

func TestInspectFetchErrorRiskControlOverrides(t *testing.T) {
	// Risk markers win even when wrapped as plain error text.
	info := inspectFetchError(errors.New("upstream gRPC status 2: -352 wind control"))
	assert.True(t, info.RiskControl)
	assert.Equal(t, FeedFailureKindHTTPStatus, info.Kind)
	assert.True(t, info.Transient)
	assert.False(t, info.FastRetryable)
}

func TestClassifyFetchErrorUsesTypedInfo(t *testing.T) {
	feedErr := classifyFetchError("https://example.com/feed", gofeed.HTTPError{
		StatusCode: 429,
		Status:     "429 Too Many Requests",
	})
	require.NotNil(t, feedErr)
	assert.Equal(t, FeedFailureKindHTTPStatus, feedErr.Kind)
	assert.True(t, feedErr.Transient)
}

func TestClassifyFeedFailurePrefersErrOverMessage(t *testing.T) {
	result := classifyFeedFailure(&FeedError{
		URL:     "https://example.com",
		Message: "ignored misleading message",
		Err:     gofeed.HTTPError{StatusCode: 404, Status: "404 Not Found"},
	})
	assert.Equal(t, FeedFailureKindHTTPStatus, result.Kind)
	assert.False(t, result.Transient)
}

func TestExtractHTTPStatusCode(t *testing.T) {
	code, ok := extractHTTPStatusCode("http error: 503 service unavailable")
	assert.True(t, ok)
	assert.Equal(t, 503, code)

	code, ok = extractHTTPStatusCode("status code 410")
	assert.True(t, ok)
	assert.Equal(t, 410, code)

	_, ok = extractHTTPStatusCode("no status here")
	assert.False(t, ok)
}
