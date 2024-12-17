package cmd

import (
	"os"
	"testing"

	gh2 "github.com/xbpk3t/docs-alfred/service/gh"

	aw "github.com/deanishe/awgo"
)

// RenderRepos function should correctly render repository items with valid data using a table-driven approach.

// func TestRenderRepos(t *testing.T) {
// 	tests := []struct {
// 		name     string
// 		repos    gh.Repos
// 		wantIcon string
// 		wantDes  string
// 	}{
// 		{
// 			name: "基础仓库",
// 			repos: gh.Repos{
// 				gh.Repository{
// 					Name:   "test-repo",
// 					User:   "test-user",
// 					Type:   "tool",
// 					Des:    "测试描述",
// 					IsStar: true,
// 				},
// 			},
// 			wantIcon: FaStar,
// 			wantDes:  "【#tool】 测试描述",
// 		},
// 		{
// 			name: "带文档的仓库",
// 			repos: gh.Repos{
// 				gh.Repository{
// 					Name:   "test-repo",
// 					User:   "test-user",
// 					Type:   "tool",
// 					Des:    "测试描述",
// 					Doc:    "http://example.com/doc",
// 					IsStar: true,
// 				},
// 			},
// 			wantIcon: FaDoc,
// 			wantDes:  "【#tool】 测试描述",
// 		},
// 		{
// 			name: "带问答的仓库",
// 			repos: gh.Repos{
// 				gh.Repository{
// 					Name:   "test-repo",
// 					User:   "test-user",
// 					Type:   "tool",
// 					Des:    "测试描述",
// 					Qs:     gh.Qs{{Q: "问题1", X: "答案1"}},
// 					IsStar: true,
// 				},
// 			},
// 			wantIcon: FaQs,
// 			wantDes:  "【#tool】 测试描述",
// 		},
// 		{
// 			name: "同时带文档和问答的仓库",
// 			repos: gh.Repos{
// 				gh.Repository{
// 					Name:   "test-repo",
// 					User:   "test-user",
// 					Type:   "tool",
// 					Des:    "测试描述",
// 					Doc:    "http://example.com/doc",
// 					Qs:     gh.Qs{{Q: "问题1", X: "答案1"}},
// 					IsStar: true,
// 				},
// 			},
// 			wantIcon: FaQsAndDoc,
// 			wantDes:  "【#tool】 测试描述",
// 		},
// 		{
// 			name: "无类型的仓库",
// 			repos: gh.Repos{
// 				gh.Repository{
// 					Name:   "test-repo",
// 					User:   "test-user",
// 					Des:    "测试描述",
// 					IsStar: true,
// 				},
// 			},
// 			wantIcon: FaStar,
// 			wantDes:  "测试描述",
// 		},
// 	}
//
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			// 初始化 workflow
// 			wf = aw.New()
//
// 			gotItem := RenderRepos(tt.repos)
// 			if gotItem == nil {
// 				t.Fatal("RenderRepos 返回了空值")
// 			}
//
// 			// 检查图标
// 			if gotItem.Icon != tt.wantIcon {
// 				t.Errorf("图标不匹配:\n- 期望: %v\n- 实际: %v", tt.wantIcon, gotItem.Icon.Value)
// 			}
//
// 			// 检查描述
// 			if gotItem.Subtitle != tt.wantDes {
// 				t.Errorf("描述不匹配:\n- 期望: %v\n- 实际: %v", tt.wantDes, gotItem.Subtitle)
// 			}
//
// 			// 检查标题
// 			expectedTitle := tt.repos[0].FullName()
// 			if gotItem.Title != expectedTitle {
// 				t.Errorf("标题不匹配:\n- 期望: %v\n- 实际: %v", expectedTitle, gotItem.Title)
// 			}
// 		})
// 	}
// }

// func setupTestEnv(t *testing.T) func() {
// 	// 确保测试目录存在
// 	if err := os.MkdirAll("./testenv/cache", 0755); err != nil {
// 		t.Fatalf("Failed to create cache directory: %v", err)
// 	}
// 	if err := os.MkdirAll("./testenv/data", 0755); err != nil {
// 		t.Fatalf("Failed to create data directory: %v", err)
// 	}
//
// 	envVars := map[string]string{
// 		"alfred_workflow_bundleid": "com.hapihacking.pwgen",
// 		"alfred_workflow_cache":    "./testenv/cache",
// 		"alfred_workflow_data":     "./testenv/data",
// 		// 可以添加其他需要的环境变量
// 	}
//
// 	t.Logf("Setting up test environment variables: %v", envVars)
//
// 	originalEnv := make(map[string]string)
// 	for k, v := range envVars {
// 		if original, exists := os.LookupEnv(k); exists {
// 			originalEnv[k] = original
// 		}
// 		if err := os.Setenv(k, v); err != nil {
// 			t.Fatalf("Failed to set environment variable %s: %v", k, err)
// 		}
//
// 		// 立即验证环境变量是否正确设置
// 		if actual := os.Getenv(k); actual != v {
// 			t.Fatalf("Environment variable %s not set correctly. Expected %s, got %s", k, v, actual)
// 		}
// 	}
//
// 	return func() {
// 		// 清理环境变量
// 		for k := range envVars {
// 			if original, exists := originalEnv[k]; exists {
// 				os.Setenv(k, original)
// 			} else {
// 				os.Unsetenv(k)
// 			}
// 		}
// 		// 清理测试目录
// 		if err := os.RemoveAll("./testenv"); err != nil {
// 			t.Logf("Warning: Failed to cleanup test directory: %v", err)
// 		}
// 	}
// }

func setupTestEnv(t *testing.T) func() {
	// 确保测试目录存在
	if err := os.MkdirAll("./testenv/cache", 0o755); err != nil {
		t.Fatalf("Failed to create cache directory: %v", err)
	}
	if err := os.MkdirAll("./testenv/data", 0o755); err != nil {
		t.Fatalf("Failed to create data directory: %v", err)
	}

	envVars := map[string]string{
		"alfred_workflow_bundleid": "com.hapihacking.pwgen",
		"alfred_workflow_cache":    "./testenv/cache",
		"alfred_workflow_data":     "./testenv/data",
		// 可以添加其他需要的环境变量
	}

	t.Logf("Setting up test environment variables: %v", envVars)

	originalEnv := make(map[string]string)
	for k, v := range envVars {
		if original, exists := os.LookupEnv(k); exists {
			originalEnv[k] = original
		}
		if err := os.Setenv(k, v); err != nil {
			t.Fatalf("Failed to set environment variable %s: %v", k, err)
		}

		// 立即验证环境变量是否正确设置
		if actual := os.Getenv(k); actual != v {
			t.Fatalf("Environment variable %s not set correctly. Expected %s, got %s", k, v, actual)
		}
	}

	return func() {
		// 清理环境变量
		for k := range envVars {
			if original, exists := originalEnv[k]; exists {
				os.Setenv(k, original)
			} else {
				os.Unsetenv(k)
			}
		}
		// 清理测试目录
		if err := os.RemoveAll("./testenv"); err != nil {
			t.Logf("Warning: Failed to cleanup test directory: %v", err)
		}
	}
}

func Test_buildDocsURL(t *testing.T) {
	ResetWorkflow()
	// 1. 首先设置环境变量
	cleanup := setupTestEnv(t)
	defer cleanup()

	// 3. 初始化 workflow (如果需要的话)
	wf = aw.New()

	// 4. 验证环境变量（调试用）
	t.Logf("Environment variables:")
	t.Logf("alfred_workflow_bundleid: %s", os.Getenv("alfred_workflow_bundleid"))
	t.Logf("alfred_workflow_cache: %s", os.Getenv("alfred_workflow_cache"))
	t.Logf("alfred_workflow_data: %s", os.Getenv("alfred_workflow_data"))

	tests := []struct {
		name string
		repo gh2.Repository
		want string
	}{
		{
			name: "基础仓库测试",
			repo: gh2.Repository{
				User: "owner",
				Name: "repo",
				Tag:  "tag",
				Type: "type",
			},
			want: "tag/#type",
		},
		{
			name: "带有Qs的仓库测试",
			repo: gh2.Repository{
				User: "owner",
				Name: "repo",
				Tag:  "tag",
				Qs:   []gh2.Question{{Q: "question", X: "answer"}},
			},
			want: "tag/#owner/repo",
		},
		{
			name: "大写标签测试",
			repo: gh2.Repository{
				User: "owner",
				Name: "repo",
				Tag:  "TAG",
				Type: "TYPE",
			},
			want: "tag/#type",
		},
		{
			name: "空标签测试",
			repo: gh2.Repository{
				User: "owner",
				Name: "repo",
				Tag:  "",
				Type: "type",
			},
			want: "/#type",
		},
		{
			name: "kong",
			repo: gh2.Repository{
				User: "spf13",
				Name: "cobra",
				Tag:  "works",
				Type: "cli",
				Sub: gh2.Repos{
					gh2.Repository{
						User: "alecthomas",
						Name: "kong",
						Tag:  "works",
						Type: "cli",
						Qs:   nil,
					},
				},
			},
			want: "works/#spf13cobra",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildDocsURL(tt.repo); got != tt.want {
				t.Errorf("buildDocsURL() = %v, want %v", got, tt.want)
			}
		})
	}
}
