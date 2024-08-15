package cmd

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hxhac/docs-alfred/pkg/gh"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type Gh []string

// mergeCmd represents the merge command
var mergeCmd = &cobra.Command{
	Use:   "merge",
	Short: "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {
		var cr gh.ConfigRepos

		err := filepath.WalkDir(folderName, func(path string, d fs.DirEntry, err error) error {
			if !d.IsDir() && slices.Contains(ghFiles, d.Name()) {
				fmt.Println(d.Name())
				fx, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				cr = append(cr, gh.NewConfigRepos(fx).WithTag(strings.TrimSuffix(d.Name(), ".yml"))...)
			}

			return nil
		})
		if err != nil {
			return
		}

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

var (
	// URL     string
	ghFiles    []string
	folderName string
)

func init() {
	rootCmd.AddCommand(mergeCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// mergeCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// mergeCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	mergeCmd.Flags().StringVar(&folderName, "folder", "data/x", "Folder Name")

	// mergeCmd.Flags().StringVar(&URL, "url", "", "CDN Base URL")
	mergeCmd.Flags().StringSliceVar(&ghFiles, "yf", []string{}, "gh.yml files")
}
