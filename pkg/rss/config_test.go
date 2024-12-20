package rss

import (
	"os"
	"testing"
)

func TestNewConfig(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			content: `
resend:
  token: "test-token"
newsletter:
  schedule: "daily"
  isHideAuthorInTitle: false
feeds:
  - type: "blog"
    urls:
      - feed: "http://example.com/feed"
feed:
  maxTries: 3
  feedLimit: 10
`,
			wantErr: false,
		},
		{
			name: "missing token",
			content: `
resend:
  token: ""
newsletter:
  schedule: "daily"
`,
			wantErr: true,
			errMsg:  "resend token is required",
		},
		{
			name: "invalid schedule",
			content: `
resend:
  token: "test-token"
newsletter:
  schedule: "invalid"
`,
			wantErr: true,
			errMsg:  "invalid schedule: invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建临时配置文件
			tmpfile, err := os.CreateTemp("", "config-*.yaml")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(tmpfile.Name())

			if _, err := tmpfile.Write([]byte(tt.content)); err != nil {
				t.Fatal(err)
			}
			if err := tmpfile.Close(); err != nil {
				t.Fatal(err)
			}

			// 测试配置加载
			cfg, err := NewConfig(tmpfile.Name())
			if tt.wantErr {
				if err == nil {
					t.Errorf("NewConfig() error = nil, wantErr %v", tt.wantErr)
				}
				if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("NewConfig() error = %v, want %v", err, tt.errMsg)
				}
				return
			}
			if err != nil {
				t.Errorf("NewConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// 验证配置内容
			if cfg.Resend.Token == "" {
				t.Error("Config.Resend.Token is empty")
			}
		})
	}
}
