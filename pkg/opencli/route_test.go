package opencli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommandForURL_twitterThread(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		wantAdap string
		wantSub  string // first arg after adapter
	}{
		{
			name:     "x.com status URL",
			url:      "https://x.com/user/status/123456789",
			wantAdap: "twitter",
			wantSub:  "thread",
		},
		{
			name:     "twitter.com status URL",
			url:      "https://twitter.com/user/status/123456789",
			wantAdap: "twitter",
			wantSub:  "thread",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter, args := CommandForURL(tt.url)
			assert.Equal(t, tt.wantAdap, adapter)
			require.GreaterOrEqual(t, len(args), 1)
			assert.Equal(t, tt.wantSub, args[0])
		})
	}
}

func TestCommandForURL_weixinDownload(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{
			name: "weixin article URL",
			url:  "https://mp.weixin.qq.com/s/abc123",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter, args := CommandForURL(tt.url)
			assert.Equal(t, "weixin", adapter)
			require.GreaterOrEqual(t, len(args), 1)
			assert.Equal(t, "download", args[0])
			require.GreaterOrEqual(t, len(args), 2)
			assert.Equal(t, urlFlag, args[1])
			require.GreaterOrEqual(t, len(args), 3)
			assert.Equal(t, tt.url, args[2])
		})
	}
}

func TestCommandForURL_tcoNoLongerRouted(t *testing.T) {
	url := "https://t.co/abc123"
	adapter, args := CommandForURL(url)
	assert.Equal(t, "web", adapter, "t.co should fall through to web")
	require.GreaterOrEqual(t, len(args), 1)
	assert.Equal(t, subcmdRead, args[0])
}

func TestCommandForURL_bilibili(t *testing.T) {
	adapter, args := CommandForURL("https://www.bilibili.com/video/BV1xx411c7mD")
	assert.Equal(t, "bilibili", adapter)
	require.GreaterOrEqual(t, len(args), 1)
	assert.Equal(t, subcmdVideo, args[0])

	// Query params should be stripped
	adapter2, args2 := CommandForURL("https://www.bilibili.com/video/BV1xx411c7mD/?spm_id_from=333.1387")
	assert.Equal(t, "bilibili", adapter2)
	for _, a := range args2 {
		assert.NotContains(t, a, "spm_id_from", "args should have query params stripped")
	}
}

func TestCommandForURL_youtube(t *testing.T) {
	adapter, args := CommandForURL("https://www.youtube.com/watch?v=abc123")
	assert.Equal(t, "youtube", adapter)
	require.GreaterOrEqual(t, len(args), 1)
	assert.Equal(t, subcmdVideo, args[0])
}

func TestCommandForURL_reddit(t *testing.T) {
	adapter, _ := CommandForURL("https://www.reddit.com/r/programming/comments/abc/")
	assert.Equal(t, "reddit", adapter)
}

func TestCommandForURL_hackernews(t *testing.T) {
	adapter, _ := CommandForURL("https://news.ycombinator.com/item?id=123")
	assert.Equal(t, "hackernews", adapter)
}

func TestCommandForURL_zhihuQuestion(t *testing.T) {
	adapter, args := CommandForURL("https://www.zhihu.com/question/35129528")
	assert.Equal(t, "zhihu", adapter)
	require.GreaterOrEqual(t, len(args), 2)
	assert.Equal(t, "35129528", args[1])
}

func TestCommandForURL_zhihuAnswer(t *testing.T) {
	adapter, args := CommandForURL("https://www.zhihu.com/question/35129528/answer/123456789")
	assert.Equal(t, "zhihu", adapter)
	require.GreaterOrEqual(t, len(args), 2)
	assert.Equal(t, "35129528", args[1])
}

func TestCommandForURL_zhuanlan(t *testing.T) {
	adapter, args := CommandForURL("https://zhuanlan.zhihu.com/p/123456789")
	assert.Equal(t, "web", adapter, "zhuanlan goes to web read")
	require.GreaterOrEqual(t, len(args), 1)
	assert.Equal(t, subcmdRead, args[0])
}

func TestCommandForURL_genericWeb(t *testing.T) {
	adapter, args := CommandForURL("https://example.com/article")
	assert.Equal(t, "web", adapter)
	require.GreaterOrEqual(t, len(args), 2)
	assert.Equal(t, urlFlag, args[1])
}

func TestIsTcoURL(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"https://t.co/abc123", true},
		{"https://t.co/zpIcP8e6rs", true},
		{"http://t.co/xyz", true},
		{"https://x.com/user/status/123", false},
		{"https://example.com", false},
		{"not-a-url", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			assert.Equal(t, tt.want, IsTcoURL(tt.url))
		})
	}
}

func TestCleanXMediaSuffix(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{
			"https://x.com/user/status/123456789/photo/1",
			"https://x.com/user/status/123456789/",
		},
		{
			"https://x.com/user/status/123456789/video/1",
			"https://x.com/user/status/123456789/",
		},
		{
			"https://twitter.com/user/status/123456789/photo/1",
			"https://twitter.com/user/status/123456789/",
		},
		{
			"https://x.com/user/status/123456789",
			"https://x.com/user/status/123456789",
		},
		{
			"https://x.com/user/status/123456789/",
			"https://x.com/user/status/123456789/",
		},
		{
			"https://example.com/photo/1",
			"https://example.com/photo/1",
		},
		{
			"https://bilibili.com/video/BV1xx/photo/1",
			"https://bilibili.com/video/BV1xx/photo/1",
		},
		{
			"not-a-url",
			"not-a-url",
		},
		{
			"",
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			assert.Equal(t, tt.want, CleanXMediaSuffix(tt.url))
		})
	}
}

func TestHasAdapter(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"https://x.com/user/status/123", true},
		{"https://twitter.com/user", true},
		{"https://www.youtube.com/watch?v=abc", true},
		{"https://www.bilibili.com/video/BV1xx", true},
		{"https://mp.weixin.qq.com/s/abc", true},
		{"https://www.zhihu.com/question/123", true},
		{"https://www.reddit.com/r/test", true},
		{"https://news.ycombinator.com/item?id=1", true},
		{"https://zhuanlan.zhihu.com/p/123", true},
		{"https://t.co/abc123", false},
		{"https://example.com", false},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			assert.Equal(t, tt.want, HasAdapter(tt.url))
		})
	}
}

func TestURLMatchesDomain_InvalidURL(t *testing.T) {
	// Invalid URLs should not match any domain
	assert.False(t, urlMatchesDomain("%zz", []string{"example.com"}))
}

func TestURLMatchesDomain_Subdomain(t *testing.T) {
	assert.True(t, urlMatchesDomain("https://www.example.com/path", []string{"example.com"}))
	assert.True(t, urlMatchesDomain("https://example.com/path", []string{"example.com"}))
	assert.False(t, urlMatchesDomain("https://notexample.com/path", []string{"example.com"}))
}

func TestExtractZhihuQuestionID_NoQuestionPrefix(t *testing.T) {
	assert.Empty(t, extractZhihuQuestionID("/answer/123"))
}

func TestExtractZhihuQuestionID_NonNumericID(t *testing.T) {
	assert.Empty(t, extractZhihuQuestionID("/question/abc"))
}

func TestExtractZhihuQuestionID_EmptyPath(t *testing.T) {
	assert.Empty(t, extractZhihuQuestionID("/"))
}

func TestExtractZhihuQuestionID_RootOnly(t *testing.T) {
	assert.Empty(t, extractZhihuQuestionID(""))
}

func TestIsTcoURL_InvalidURL(t *testing.T) {
	assert.False(t, IsTcoURL("%zz"))
}

func TestCleanXMediaSuffix_InvalidURL(t *testing.T) {
	assert.Equal(t, "%zz", CleanXMediaSuffix("%zz"))
}

func TestIsNumeric(t *testing.T) {
	assert.True(t, isNumeric("123"))
	assert.True(t, isNumeric("0"))
	assert.False(t, isNumeric("abc"))
	assert.False(t, isNumeric(""))
	assert.False(t, isNumeric("12a"))
}

func TestCommandForURL_zhihuNoQuestionID(t *testing.T) {
	adapter, args := CommandForURL("https://www.zhihu.com/hot")
	assert.Equal(t, "zhihu", adapter)
	// Without a valid question ID, args should fall through to generic format with full URL
	require.GreaterOrEqual(t, len(args), 2)
	assert.Equal(t, "https://www.zhihu.com/hot", args[1])
}
