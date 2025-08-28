package cmd

import (
	"bytes"
	"os"
	"sync"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/docs/pkg"
)

var wg sync.WaitGroup

// createRootCmd creates the root command with the given config file parameter
func createRootCmd(cfgFile *string) *cobra.Command {
	return &cobra.Command{
		Use:   "docs-alfred",
		Short: "根据 docs.yml 生成文档",
		RunE: func(_ *cobra.Command, _ []string) error {
			// 读取配置文件
			configData, err := os.ReadFile(*cfgFile)
			if err != nil {
				return err
			}

			// 解析配置
			var rawConfigs []pkg.DocsConfig
			if err := yaml.NewDecoder(bytes.NewReader(configData)).Decode(&rawConfigs); err != nil {
				return err
			}

			configs := make([]pkg.DocsConfig, 0, len(rawConfigs))
			for _, raw := range rawConfigs {
				config := &pkg.DocsConfig{
					Src: raw.Src,
					Cmd: raw.Cmd,
				}
				if raw.JSON != nil {
					config.JSON = pkg.NewDocProcessor(pkg.FileTypeJSON)
					config.JSON.Dst = raw.JSON.Dst
					config.JSON.MergeOutputFile = raw.JSON.MergeOutputFile
				}
				if raw.YAML != nil {
					config.YAML = pkg.NewDocProcessor(pkg.FileTypeYAML)
					config.YAML.Dst = raw.YAML.Dst
					config.YAML.MergeOutputFile = raw.YAML.MergeOutputFile
				}
				configs = append(configs, *config)
			}

			wg.Add(len(configs))

			// 处理每个配置
			for _, config := range configs {
				go func(cfg pkg.DocsConfig) {
					defer wg.Done()
					_ = cfg.Process()
				}(config)
			}

			wg.Wait()

			return nil
		},
	}
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	var cfgFile string
	rootCmd := createRootCmd(&cfgFile)
	rootCmd.Flags().StringVarP(&cfgFile, "config", "c", "docs.yml", "配置文件路径")

	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
