package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 测试用的配置结构
type TestConfig struct {
	Type string `yaml:"type"`
	Tag  string `yaml:"tag"`
	Name string `yaml:"name"`
}

func (c *TestConfig) SetTag(tag string) {
	c.Tag = tag
}

func TestMerger_Merge(t *testing.T) {
	// 创建临时测试目录
	tempDir, err := os.MkdirTemp("", "test-merger-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// 创建测试文件
	testFiles := map[string]string{
		"test1.yml": `
- type: "type1"
  name: "test1"
`,
		"test2.yml": `
- type: "type2"
  name: "test2"
`,
	}

	for name, content := range testFiles {
		err := os.WriteFile(filepath.Join(tempDir, name), []byte(content), 0o644)
		require.NoError(t, err)
	}

	outputPath := filepath.Join(tempDir, "output.yml")

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
				OutputPath: outputPath,
			},
			wantErr: false,
		},
		{
			name: "文件夹不存在",
			opts: MergeOptions{
				FolderPath: "not_exists",
				FileNames:  []string{"test1.yml"},
				OutputPath: outputPath,
			},
			wantErr: true,
		},
		{
			name: "空文件列表",
			opts: MergeOptions{
				FolderPath: tempDir,
				FileNames:  []string{},
				OutputPath: outputPath,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merger := NewMerger[TestConfig](tt.opts)
			err := merger.Merge()

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)

			// 验证输出文件存在
			_, err = os.Stat(tt.opts.OutputPath)
			assert.NoError(t, err)

			// 验证输出内容
			if !tt.wantErr {
				content, err := os.ReadFile(tt.opts.OutputPath)
				assert.NoError(t, err)
				assert.Contains(t, string(content), "type1")
				assert.Contains(t, string(content), "type2")
			}
		})
	}
}

func TestMerger_validateInput(t *testing.T) {
	tests := []struct {
		name    string
		opts    MergeOptions
		wantErr bool
	}{
		{
			name: "有效输入",
			opts: MergeOptions{
				FolderPath: "folder",
				FileNames:  []string{"file1.yml"},
				OutputPath: "output.yml",
			},
			wantErr: false,
		},
		{
			name: "空文件夹路径",
			opts: MergeOptions{
				FolderPath: "",
				FileNames:  []string{"file1.yml"},
				OutputPath: "output.yml",
			},
			wantErr: true,
		},
		{
			name: "空文件列表",
			opts: MergeOptions{
				FolderPath: "folder",
				FileNames:  []string{},
				OutputPath: "output.yml",
			},
			wantErr: true,
		},
		{
			name: "空输出路径",
			opts: MergeOptions{
				FolderPath: "folder",
				FileNames:  []string{"file1.yml"},
				OutputPath: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merger := NewMerger[TestConfig](tt.opts)
			err := merger.validateInput()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMerger_processFile(t *testing.T) {
	// 创建临时测试目录
	tempDir, err := os.MkdirTemp("", "test-process-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// 创建测试文件
	testContent := `
- type: "test"
  name: "test1"
`
	testFile := "test.yml"
	err = os.WriteFile(filepath.Join(tempDir, testFile), []byte(testContent), 0o644)
	require.NoError(t, err)

	tests := []struct {
		name     string
		fileName string
		wantLen  int
		wantErr  bool
	}{
		{
			name:     "有效文件",
			fileName: testFile,
			wantLen:  1,
			wantErr:  false,
		},
		{
			name:     "文件不存在",
			fileName: "not_exists.yml",
			wantLen:  0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merger := NewMerger[TestConfig](MergeOptions{
				FolderPath: tempDir,
				FileNames:  []string{tt.fileName},
				OutputPath: "output.yml",
			})

			configs, err := merger.processFile(tt.fileName)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Len(t, configs, tt.wantLen)
			if tt.wantLen > 0 {
				assert.Equal(t, "test", configs[0].Type)
				assert.Equal(t, "test1", configs[0].Name)
				// 验证标签是否被正确设置
				assert.Equal(t, "test", configs[0].Tag)
			}
		})
	}
}

func TestMergeFiles(t *testing.T) {
	// 创建临时测试目录
	tempDir, err := os.MkdirTemp("", "test-merge-files-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// 创建测试文件
	testFiles := map[string]string{
		"config1.yml": `
- type: "type1"
  name: "test1"
`,
		"config2.yml": `
- type: "type2"
  name: "test2"
`,
	}

	for name, content := range testFiles {
		err := os.WriteFile(filepath.Join(tempDir, name), []byte(content), 0o644)
		require.NoError(t, err)
	}

	outputPath := filepath.Join(tempDir, "merged.yml")

	err = MergeFiles[TestConfig](
		tempDir,
		[]string{"config1.yml", "config2.yml"},
		outputPath,
	)
	assert.NoError(t, err)

	// 验证输出文件
	content, err := os.ReadFile(outputPath)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "type1")
	assert.Contains(t, string(content), "type2")
}
