package rss

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
	"syscall"

	"github.com/mmcdole/gofeed"
)

// fetchErrorInfo is the typed classification of a fetch/parse failure.
// String matching is only a fallback for domain-specific risk-control messages
// and legacy plain-text errors (tests / non-gofeed sources).
type fetchErrorInfo struct {
	Kind          FeedFailureKind
	StatusCode    int
	Transient     bool
	RiskControl   bool
	FastRetryable bool
	Is5xx         bool
}

func inspectFetchError(err error) fetchErrorInfo {
	if err == nil {
		return fetchErrorInfo{Kind: FeedFailureKindUnknown}
	}

	// Domain-specific risk control (Bilibili etc.) — no structured type upstream.
	if isRiskControlMessage(err.Error()) {
		return fetchErrorInfo{
			Kind:          FeedFailureKindHTTPStatus,
			Transient:     true,
			RiskControl:   true,
			FastRetryable: false,
			Is5xx:         true,
		}
	}

	if info, ok := classifyTypedFetchError(err); ok {
		return info
	}

	return classifyFromMessage(strings.ToLower(err.Error()))
}

func classifyTypedFetchError(err error) (fetchErrorInfo, bool) {
	if info, ok := classifyContextError(err); ok {
		return info, true
	}
	if info, ok := classifyHTTPOrDNSError(err); ok {
		return info, true
	}
	if info, ok := classifyNetTransportError(err); ok {
		return info, true
	}
	if info, ok := classifyURLError(err); ok {
		return info, true
	}
	if info, ok := classifyConnSyscallError(err); ok {
		return info, true
	}
	if errors.Is(err, gofeed.ErrFeedTypeNotDetected) {
		return fetchErrorInfo{Kind: FeedFailureKindParse}, true
	}

	return fetchErrorInfo{}, false
}

func classifyContextError(err error) (fetchErrorInfo, bool) {
	if errors.Is(err, context.Canceled) {
		return fetchErrorInfo{
			Kind:      FeedFailureKindContextCancelled,
			Transient: true,
		}, true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return transientFast(FeedFailureKindTimeout), true
	}

	return fetchErrorInfo{}, false
}

func classifyHTTPOrDNSError(err error) (fetchErrorInfo, bool) {
	var httpErr gofeed.HTTPError
	if errors.As(err, &httpErr) {
		return fromHTTPStatus(httpErr.StatusCode), true
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return transientFast(FeedFailureKindDNS), true
	}

	return fetchErrorInfo{}, false
}

func classifyNetTransportError(err error) (fetchErrorInfo, bool) {
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return classifyOpError(opErr)
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return transientFast(FeedFailureKindTimeout), true
	}

	return fetchErrorInfo{}, false
}

func classifyOpError(opErr *net.OpError) (fetchErrorInfo, bool) {
	if opErr.Timeout() {
		return transientFast(FeedFailureKindTimeout), true
	}
	if isTLSOpError(opErr) {
		return transientFast(FeedFailureKindTLS), true
	}
	if isNetworkOpError(opErr) {
		return transientFast(FeedFailureKindNetwork), true
	}

	return fetchErrorInfo{}, false
}

func classifyURLError(err error) (fetchErrorInfo, bool) {
	var urlErr *url.Error
	if !errors.As(err, &urlErr) {
		return fetchErrorInfo{}, false
	}
	if urlErr.Timeout() {
		return transientFast(FeedFailureKindTimeout), true
	}
	if urlErr.Err == nil || errors.Is(urlErr.Err, err) {
		return fetchErrorInfo{}, false
	}

	return classifyTypedFetchError(urlErr.Err)
}

func classifyConnSyscallError(err error) (fetchErrorInfo, bool) {
	if errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ECONNABORTED) {
		return transientFast(FeedFailureKindNetwork), true
	}

	return fetchErrorInfo{}, false
}

func transientFast(kind FeedFailureKind) fetchErrorInfo {
	return fetchErrorInfo{
		Kind:          kind,
		Transient:     true,
		FastRetryable: true,
	}
}

func fromHTTPStatus(code int) fetchErrorInfo {
	info := fetchErrorInfo{
		Kind:       FeedFailureKindHTTPStatus,
		StatusCode: code,
		Is5xx:      code >= 500 && code <= 599,
	}
	switch code {
	case 429, 500, 502, 503, 504:
		info.Transient = true
		info.FastRetryable = true
	default:
		if info.Is5xx {
			info.Transient = true
			info.FastRetryable = true
		}
		// 4xx (incl. 401/403/404/410): not fast-retryable
	}

	return info
}

func isTLSOpError(opErr *net.OpError) bool {
	if opErr == nil {
		return false
	}
	msg := strings.ToLower(opErr.Error())

	return strings.Contains(msg, "tls") ||
		strings.Contains(msg, "certificate") ||
		strings.Contains(msg, "handshake")
}

func isNetworkOpError(opErr *net.OpError) bool {
	if opErr == nil {
		return false
	}
	if errors.Is(opErr.Err, syscall.ECONNRESET) ||
		errors.Is(opErr.Err, syscall.ECONNREFUSED) ||
		errors.Is(opErr.Err, syscall.ECONNABORTED) ||
		errors.Is(opErr.Err, syscall.EPIPE) {
		return true
	}
	msg := strings.ToLower(opErr.Error())

	return containsAny(msg,
		"connection reset",
		"connection refused",
		"broken pipe",
		"network is unreachable",
	)
}

// classifyFromMessage is the legacy substring fallback used when no typed
// error is available (plain errors.New in tests, parse messages, etc.).
func classifyFromMessage(message string) fetchErrorInfo {
	if message == "" {
		return fetchErrorInfo{Kind: FeedFailureKindUnknown}
	}
	if containsAny(message, "context canceled") {
		return fetchErrorInfo{Kind: FeedFailureKindContextCancelled, Transient: true}
	}
	if containsAny(message,
		"context deadline exceeded",
		"client.timeout exceeded",
		"i/o timeout",
		"timeout",
		"deadline exceeded",
	) {
		return fetchErrorInfo{Kind: FeedFailureKindTimeout, Transient: true, FastRetryable: true}
	}
	if containsAny(message,
		"no such host",
		"temporary failure in name resolution",
		"server misbehaving",
		"lookup ",
	) {
		return fetchErrorInfo{Kind: FeedFailureKindDNS, Transient: true, FastRetryable: true}
	}
	if containsAny(message,
		"tls",
		"ssl routines",
		"certificate",
		"handshake",
	) {
		return fetchErrorInfo{Kind: FeedFailureKindTLS, Transient: true, FastRetryable: true}
	}
	if code, ok := extractHTTPStatusCode(message); ok {
		return fromHTTPStatus(code)
	}
	// Loose HTTP status markers (message may only say "502 bad gateway").
	if containsAny(message, "status code", "status:", "http error:") ||
		containsAny(message, "429", "500", "502", "503", "504", "404", "410", "401", "403") {
		transient := isTransientHTTPStatus(message)
		return fetchErrorInfo{
			Kind:          FeedFailureKindHTTPStatus,
			Transient:     transient,
			FastRetryable: transient,
			Is5xx:         containsAny(message, "500", "502", "503", "504"),
		}
	}
	if containsAny(message,
		"xml syntax error",
		"not a valid feed",
		"not a feed",
		"failed to detect feed type",
		"invalid feed",
	) {
		return fetchErrorInfo{Kind: FeedFailureKindParse}
	}
	if containsAny(message,
		"unexpected eof",
		"connection reset",
		"connection refused",
		"network is unreachable",
		"eof",
	) {
		return fetchErrorInfo{Kind: FeedFailureKindNetwork, Transient: true, FastRetryable: true}
	}

	return fetchErrorInfo{Kind: FeedFailureKindUnknown}
}

func extractHTTPStatusCode(message string) (int, bool) {
	// Prefer "status code NNN" / "http error: NNN ..." shapes from gofeed and clients.
	for _, prefix := range []string{"status code ", "http error: ", "status: ", "http "} {
		if i := strings.Index(message, prefix); i >= 0 {
			rest := message[i+len(prefix):]
			code, ok := parseLeadingStatusCode(rest)
			if ok {
				return code, true
			}
		}
	}

	return 0, false
}

func parseLeadingStatusCode(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if len(s) < 3 {
		return 0, false
	}
	n, err := strconv.Atoi(s[:3])
	if err != nil || n < 100 || n > 599 {
		return 0, false
	}

	return n, true
}

func isRiskControlMessage(msg string) bool {
	msg = strings.ToLower(msg)

	return containsAny(msg,
		"-352",
		"风控",
		"grpc status 2",
		"v_voucher",
	)
}

// isRiskControlFetchError reports ban/风控 style failures that must not short-retry.
func isRiskControlFetchError(err error) bool {
	if err == nil {
		return false
	}

	return inspectFetchError(err).RiskControl
}

// isFastRetryableFetchError is worth short backoff (timeouts, soft gateway errors).
func isFastRetryableFetchError(err error) bool {
	if err == nil {
		return false
	}

	return inspectFetchError(err).FastRetryable
}

func isHTTP5xxFetchError(err error) bool {
	if err == nil {
		return false
	}

	return inspectFetchError(err).Is5xx
}

func containsAny(s string, needles ...string) bool {
	for _, needle := range needles {
		if needle != "" && strings.Contains(s, needle) {
			return true
		}
	}

	return false
}
