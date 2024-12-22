package gh

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepository_Methods(t *testing.T) {
	tests := []struct {
		name     string
		wantName string
		repo     Repository
		isValid  bool
		isSub    bool
	}{
		{
			name: "有效仓库",
			repo: Repository{
				User: "user",
				Name: "repo",
			},
			wantName: "user/repo",
			isValid:  true,
			isSub:    false,
		},
		{
			name: "子仓库",
			repo: Repository{
				User: "user",
				Name: "repo",
				Type: "sub",
			},
			wantName: "user/repo",
			isValid:  true,
			isSub:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantName, tt.repo.FullName())
			assert.Equal(t, tt.isValid, tt.repo.IsValid())
			assert.Equal(t, tt.isSub, tt.repo.IsSubRepo())
		})
	}
}

func TestConfigMerger(t *testing.T) {
	// 创建临时测试目录
	tempDir, err := os.MkdirTemp("", "test-config-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// 创建测试文件
	testFiles := map[string]string{
		"test1.yml": `
- type: "test1"
  repos:
    - url: "https://github.com/user/repo1"`,
		"test2.yml": `
- type: "test2"
  repos:
    - url: "https://github.com/user/repo2"`,
	}

	for name, content := range testFiles {
		err := os.WriteFile(filepath.Join(tempDir, name), []byte(content), 0o644)
		require.NoError(t, err)
	}

	// 测试合并
	merger := NewConfigMerger(MergeOptions{
		FolderPath: tempDir,
		FileNames:  []string{"test1.yml", "test2.yml"},
		OutputPath: filepath.Join(tempDir, "output.yml"),
	})

	err = merger.Merge()
	assert.NoError(t, err)

	// 验证输出文件
	content, err := os.ReadFile(filepath.Join(tempDir, "output.yml"))
	assert.NoError(t, err)
	assert.Contains(t, string(content), "test1")
	assert.Contains(t, string(content), "test2")
}
