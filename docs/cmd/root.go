package cmd

import (
	"os"
	"sync"

	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/docs/pkg"
	"github.com/xbpk3t/docs-alfred/pkg/parser"
)

var cfgFile string

var wg sync.WaitGroup

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "docs-alfred",
	Short: "根据 docs.yml 生成文档",
	RunE: func(cmd *cobra.Command, args []string) error {
		// 读取配置文件
		configData, err := os.ReadFile(cfgFile)
		if err != nil {
			return err
		}

		// 解析配置
		configs, err := parser.NewParser[[]pkg.DocsConfig](configData).ParseSingle()
		if err != nil {
			return err
		}

		wg.Add(len(configs))

		// 处理每个配置
		for _, config := range configs {
			go func() {
				defer wg.Done()
				_ = config.Process()
			}()
		}

		wg.Wait()

		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVarP(&cfgFile, "config", "c", "docs.yml", "配置文件路径")
}
