package cmd

import (
	"bytes"
	"os"
	"sync"

	yaml "github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/docs/pkg"
)

// processConfig 处理单个配置项.
func processConfig(raw pkg.DocsConfig) pkg.DocsConfig {
	config := pkg.DocsConfig{
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

	return config
}

// loadConfigs 加载并解析配置文件.
func loadConfigs(cfgFile string) ([]pkg.DocsConfig, error) {
	configData, err := os.ReadFile(cfgFile)
	if err != nil {
		return nil, err
	}

	var rawConfigs []pkg.DocsConfig
	if err := yaml.NewDecoder(bytes.NewReader(configData)).Decode(&rawConfigs); err != nil {
		return nil, err
	}

	configs := make([]pkg.DocsConfig, 0, len(rawConfigs))
	for _, raw := range rawConfigs {
		configs = append(configs, processConfig(raw))
	}

	return configs, nil
}

// processConfigs 并发处理所有配置.
func processConfigs(configs []pkg.DocsConfig) {
	var wg sync.WaitGroup
	wg.Add(len(configs))

	for _, config := range configs {
		go func(cfg pkg.DocsConfig) {
			defer wg.Done()
			_ = cfg.Process()
		}(config)
	}

	wg.Wait()
}

// createRootCmd creates the root command with the given config file parameter.
func createRootCmd(cfgFile *string) *cobra.Command {
	return &cobra.Command{
		Use:   "docs-alfred",
		Short: "根据 docs.yml 生成文档",
		RunE: func(_ *cobra.Command, _ []string) error {
			configs, err := loadConfigs(*cfgFile)
			if err != nil {
				return err
			}

			processConfigs(configs)

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
