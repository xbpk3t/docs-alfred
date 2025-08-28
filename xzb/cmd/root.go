/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"log"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
func createRootCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "bm",
		Short: "合并微信和支付宝账单的工具",
		Long: `xzb 是一个用来合并微信和支付宝账单的命令行工具。
它可以自动处理账单格式，清理元数据，并按月份拆分输出为CSV和Excel文件。`,
	}
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	rootCmd := createRootCmd()
	setupRootCmd(rootCmd)
	err := rootCmd.Execute()
	if err != nil {
		log.Fatal(err)
	}
}

// setupRootCmd sets up the root command with subcommands
func setupRootCmd(rootCmd *cobra.Command) {
	rootCmd.AddCommand(getMergeCmd())
	rootCmd.AddCommand(getWebCmd())
}
