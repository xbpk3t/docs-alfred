package cmd

import (
	"fmt"
	"github.com/xbpk3t/docs-alfred/pkg/gh"
	"github.com/xbpk3t/docs-alfred/pkg/goods"
	"github.com/xbpk3t/docs-alfred/pkg/work"
	"github.com/xbpk3t/docs-alfred/pkg/ws"
	"github.com/xbpk3t/docs-alfred/utils"
	"os"

	"github.com/spf13/viper"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "yaml2md",
	Short: "A brief description of your application",
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

var cfgFile string

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.AddCommand(ghCmd)
	rootCmd.AddCommand(worksCmd)
	rootCmd.AddCommand(wsCmd)
	rootCmd.AddCommand(goodsCmd)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.yaml2md.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "src/data/qs.yml", "config file (default is src/data/qs.yml)")
}

func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath("src/data/")
		viper.SetConfigName("qs")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

var ghCmd = &cobra.Command{
	Use:   "gh",
	Short: "Convert GitHub repos yaml to markdown",
	RunE: func(cmd *cobra.Command, args []string) error {
		renderer := &gh.GhRenderer{}
		return utils.ProcessFile(cfgFile, renderer)
	},
}

// cmd/works.go
var worksCmd = &cobra.Command{
	Use:   "works",
	Short: "Convert works yaml to markdown",
	RunE: func(cmd *cobra.Command, args []string) error {
		renderer := &work.WorksRenderer{}
		return utils.ProcessFile(cfgFile, renderer)
	},
}

var wsCmd = &cobra.Command{
	Use:   "ws",
	Short: "Convert website links yaml to markdown",
	RunE: func(cmd *cobra.Command, args []string) error {
		renderer := &ws.WsRenderer{}
		return utils.ProcessFile(cfgFile, renderer)
	},
}

// cmd/goods.go
var goodsCmd = &cobra.Command{
	Use:   "goods",
	Short: "Convert goods yaml to markdown",
	RunE: func(cmd *cobra.Command, args []string) error {
		renderer := &goods.GoodsRenderer{}
		return utils.ProcessFile(cfgFile, renderer)
	},
}
