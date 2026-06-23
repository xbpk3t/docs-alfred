package enrich

import (
	"net/url"

	"github.com/go-resty/resty/v2"
	"golang.org/x/time/rate"
)

func newTestHTTPClient(baseURL string) *resty.Client {
	u, _ := url.Parse(baseURL)
	return resty.New().SetBaseURL(u.Scheme + "://" + u.Host)
}

func newTestRateLimiter() *rate.Limiter {
	return rate.NewLimiter(rate.Inf, 1)
}
