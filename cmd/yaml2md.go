package cmd

import (
	"fmt"
	"html/template"
	"log"
	"os"

	"github.com/91go/docs-alfred/pkg/qs"

	"github.com/spf13/cobra"
)

// yaml2mdCmd represents the yaml2md command
var yaml2mdCmd = &cobra.Command{
	Use:   "yaml2md",
	Short: "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("yaml2md called")
	},
}

var (
	yamlFile   string
	targetFile string
)

func init() {
	rootCmd.AddCommand(yaml2mdCmd)
	yaml2mdCmd.AddCommand(mdQsCmd)
	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// yaml2mdCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// yaml2mdCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	yaml2mdCmd.PersistentFlags().StringVar(&yamlFile, "config", "src/data/qs.yml", "config file (default is src/data/qs.yml)")
	yaml2mdCmd.PersistentFlags().StringVar(&targetFile, "target", "qs.md", "target file (default is qs.md)")
}

const tpl = `{{ range . }}

## {{ .Cate }}

{{ range .Xxx }}

### {{ .Name }}
{{range .Qs}}
- {{.}}{{end}}

{{ end }}{{ end }}
`

// qsCmd represents the qs command
var mdQsCmd = &cobra.Command{
	Use:   "qs",
	Short: "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {
		docs := qs.NewDocs(cfgFile)

		tmpl := template.Must(template.New("").Parse(tpl))

		file, err := os.Create(targetFile)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		err = tmpl.Execute(file, docs)
		if err != nil {
			log.Fatal(err)
		}

		log.Println("Markdown output has been written to qs.md")
	},
}
