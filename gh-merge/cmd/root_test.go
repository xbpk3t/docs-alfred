package cmd

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecute(t *testing.T) {
	// 设置测试环境
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	tests := []struct {
		setup    func() error
		teardown func()
		name     string
		args     []string
		wantErr  bool
	}{
		{
			name: "valid config file",
			args: []string{"gh-merge", "--yf", "testdata/test_1.yml", "testdata/test_2.yml"},
			setup: func() error {
				return os.MkdirAll("testdata", 0o755)
			},
			teardown: func() {
				os.RemoveAll("testdata")
			},
		},
		{
			name:    "missing config file",
			args:    []string{"gh-merge", "--yf", "testdata/nonexistent.yml"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				err := tt.setup()
				require.NoError(t, err)
			}

			if tt.teardown != nil {
				defer tt.teardown()
			}

			os.Args = tt.args

			err := rootCmd.Execute()

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}
