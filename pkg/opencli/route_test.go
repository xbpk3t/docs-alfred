package opencli

import (
	"testing"
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
			if adapter != tt.wantAdap {
				t.Errorf("CommandForURL() adapter = %q, want %q", adapter, tt.wantAdap)
			}
			if len(args) < 1 || args[0] != tt.wantSub {
				t.Errorf("CommandForURL() args[0] = %q, want %q", args[0], tt.wantSub)
			}
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
			if adapter != "weixin" {
				t.Errorf("CommandForURL() adapter = %q, want %q", adapter, "weixin")
			}
			// First arg should be "download" not "article"
			if len(args) < 1 || args[0] != "download" {
				t.Errorf("CommandForURL() args[0] = %q, want %q", args[0], "download")
			}
			// Should use --url flag format
			if len(args) < 2 || args[1] != urlFlag {
				t.Errorf("CommandForURL() args[1] = %q, want %q", args[1], urlFlag)
			}
			if len(args) < 3 || args[2] != tt.url {
				t.Errorf("CommandForURL() args[2] = %q, want %q", args[2], tt.url)
			}
		})
	}
}

func TestCommandForURL_tcoNoLongerRouted(t *testing.T) {
	// t.co is no longer in the route table — it should fall through to web read.
	// Actual resolution happens in the driver layer.
	url := "https://t.co/abc123"
	adapter, args := CommandForURL(url)
	if adapter != "web" {
		t.Errorf("CommandForURL() adapter = %q, want %q (t.co should fall through to web)", adapter, "web")
	}
	if len(args) < 1 || args[0] != subcmdRead {
		t.Errorf("CommandForURL() args[0] = %q, want %q", args[0], subcmdRead)
	}
}

func TestCommandForURL_bilibili(t *testing.T) {
	adapter, args := CommandForURL("https://www.bilibili.com/video/BV1xx411c7mD")
	if adapter != "bilibili" {
		t.Errorf("CommandForURL() adapter = %q, want %q", adapter, "bilibili")
	}
	if len(args) < 1 || args[0] != subcmdVideo {
		t.Errorf("CommandForURL() args[0] = %q, want %q", args[0], subcmdVideo)
	}
	// Query params should be stripped
	adapter2, args2 := CommandForURL("https://www.bilibili.com/video/BV1xx411c7mD/?spm_id_from=333.1387")
	if adapter2 != "bilibili" {
		t.Errorf("CommandForURL() adapter = %q, want %q", adapter2, "bilibili")
	}
	if len(args2) >= 2 && stringsContains(args2[1], "spm_id_from") {
		t.Errorf("CommandForURL() args should have query params stripped: %v", args2)
	}
}

func TestCommandForURL_youtube(t *testing.T) {
	adapter, args := CommandForURL("https://www.youtube.com/watch?v=abc123")
	if adapter != "youtube" {
		t.Errorf("CommandForURL() adapter = %q, want %q", adapter, "youtube")
	}
	if len(args) < 1 || args[0] != subcmdVideo {
		t.Errorf("CommandForURL() args[0] = %q, want %q", args[0], subcmdVideo)
	}
}

func TestCommandForURL_reddit(t *testing.T) {
	adapter, _ := CommandForURL("https://www.reddit.com/r/programming/comments/abc/")
	if adapter != "reddit" {
		t.Errorf("CommandForURL() adapter = %q, want %q", adapter, "reddit")
	}
}

func TestCommandForURL_hackernews(t *testing.T) {
	adapter, _ := CommandForURL("https://news.ycombinator.com/item?id=123")
	if adapter != "hackernews" {
		t.Errorf("CommandForURL() adapter = %q, want %q", adapter, "hackernews")
	}
}

func TestCommandForURL_zhihuQuestion(t *testing.T) {
	adapter, args := CommandForURL("https://www.zhihu.com/question/35129528")
	if adapter != "zhihu" {
		t.Errorf("CommandForURL() adapter = %q, want %q", adapter, "zhihu")
	}
	if len(args) < 2 || args[1] != "35129528" {
		t.Errorf("CommandForURL() args = %v, want numeric ID as second arg", args)
	}
}

func TestCommandForURL_zhihuAnswer(t *testing.T) {
	adapter, args := CommandForURL("https://www.zhihu.com/question/35129528/answer/123456789")
	if adapter != "zhihu" {
		t.Errorf("CommandForURL() adapter = %q, want %q", adapter, "zhihu")
	}
	// Should extract the question ID
	if len(args) < 2 || args[1] != "35129528" {
		t.Errorf("CommandForURL() args = %v, want extracted question ID", args)
	}
}

func TestCommandForURL_zhuanlan(t *testing.T) {
	// zhuanlan.zhihu.com should go to web read
	adapter, args := CommandForURL("https://zhuanlan.zhihu.com/p/123456789")
	if adapter != "web" {
		t.Errorf("CommandForURL() adapter = %q, want %q (zhuanlan goes to web read)", adapter, "web")
	}
	if len(args) < 1 || args[0] != subcmdRead {
		t.Errorf("CommandForURL() args[0] = %q, want %q", args[0], subcmdRead)
	}
}

func TestCommandForURL_genericWeb(t *testing.T) {
	adapter, args := CommandForURL("https://example.com/article")
	if adapter != "web" {
		t.Errorf("CommandForURL() adapter = %q, want %q", adapter, "web")
	}
	if len(args) < 2 || args[1] != urlFlag {
		t.Errorf("CommandForURL() args format wrong: %v", args)
	}
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
			if got := IsTcoURL(tt.url); got != tt.want {
				t.Errorf("IsTcoURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestCleanXMediaSuffix(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		// X.com URLs with photo/video suffix
		{
			"https://x.com/user/status/123456789/photo/1",
			"https://x.com/user/status/123456789/",
		},
		{
			"https://x.com/user/status/123456789/video/1",
			"https://x.com/user/status/123456789/",
		},
		// Twitter.com URLs
		{
			"https://twitter.com/user/status/123456789/photo/1",
			"https://twitter.com/user/status/123456789/",
		},
		// Already clean URLs should be unchanged
		{
			"https://x.com/user/status/123456789",
			"https://x.com/user/status/123456789",
		},
		{
			"https://x.com/user/status/123456789/",
			"https://x.com/user/status/123456789/",
		},
		// Non-X URLs should be unchanged
		{
			"https://example.com/photo/1",
			"https://example.com/photo/1",
		},
		{
			"https://bilibili.com/video/BV1xx/photo/1",
			"https://bilibili.com/video/BV1xx/photo/1",
		},
		// Invalid URLs
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
			if got := CleanXMediaSuffix(tt.url); got != tt.want {
				t.Errorf("CleanXMediaSuffix(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

// stringsContains is a helper for substring matching in slices.
func stringsContains(s, substr string) bool {
	return len(s) >= len(substr) && containsSubstr(s, substr)
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
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
		// t.co no longer has an adapter — resolved by driver layer
		{"https://t.co/abc123", false},
		{"https://example.com", false},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			if got := HasAdapter(tt.url); got != tt.want {
				t.Errorf("HasAdapter(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}
