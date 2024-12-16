package gh

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigMerger_Merge(t *testing.T) {
	// 创建临时测试目录
	tempDir := t.TempDir()

	// 准备测试文件
	testFiles := map[string]string{
		"test1.yml": `
type: test1
repo:
  - url: https://github.com/test1/repo1
`,
		"test2.yml": `
type: test2
repo:
  - url: https://github.com/test2/repo2
`,
	}

	// 写入测试文件
	for name, content := range testFiles {
		err := os.WriteFile(filepath.Join(tempDir, name), []byte(content), 0o644)
		if err != nil {
			t.Fatalf("创建测试文件失败: %v", err)
		}
	}

	tests := []struct {
		name    string
		opts    MergeOptions
		wantErr bool
	}{
		{
			name: "正常合并",
			opts: MergeOptions{
				FolderPath: tempDir,
				FileNames:  []string{"test1.yml", "test2.yml"},
				OutputPath: filepath.Join(tempDir, "output.yml"),
			},
			wantErr: false,
		},
		{
			name: "空文件列表",
			opts: MergeOptions{
				FolderPath: tempDir,
				FileNames:  []string{},
				OutputPath: filepath.Join(tempDir, "output.yml"),
			},
			wantErr: true,
		},
		// 可以添加更多测试用例
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merger := NewConfigMerger(tt.opts)
			err := merger.Merge()
			if (err != nil) != tt.wantErr {
				t.Errorf("Merge() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
