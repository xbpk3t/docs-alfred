package cmd

import (
	"fmt"
	"github.com/hxhac/docs-alfred/pkg/gh"
	"github.com/hxhac/docs-alfred/utils"
	"gopkg.in/yaml.v3"
	"log"
	"os"

	"github.com/spf13/cobra"
)

type Gh []string

// mergeCmd represents the merge command
var mergeCmd = &cobra.Command{
	Use:   "merge",
	Short: "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {
		var cr gh.ConfigRepos

		gf, err2 := utils.Fetch("https://cdn.hxha.xyz/f/gh/gh.yml")
		if err2 != nil {
			return
		}

		// cr = append(cr, gh.NewConfigRepoFile(gf)...)

		cr = gh.NewConfigRepos(gf)

		// 定义输出文件路径
		outputPath := "gh.yml"

		// 将合并后的数据写入 YAML 文件
		if err := WriteYAMLToFile(cr, outputPath); err != nil {
			log.Fatalf("error writing to YAML file: %v", err)
		}

		fmt.Printf("Merged YAML file created: %s\n", outputPath)
	},
}

// WriteYAMLToFile 将 YAML 数据写入文件
func WriteYAMLToFile(data gh.ConfigRepos, outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := yaml.NewEncoder(file)
	defer encoder.Close()

	if err := encoder.Encode(data); err != nil {
		return err
	}
	return nil
}

func init() {
	rootCmd.AddCommand(mergeCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// mergeCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// mergeCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}