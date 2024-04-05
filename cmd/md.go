package cmd

import (
	"errors"
	"html/template"
	"log"
	"os"

	"github.com/91go/docs-alfred/pkg/qs"

	"github.com/spf13/cobra"
)

// mdCmd represents the md command
var mdCmd = &cobra.Command{
	Use:   "md",
	Short: "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {
	},
}

var targetFile string

func init() {
	rootCmd.AddCommand(mdCmd)
	mdCmd.AddCommand(mdQsCmd)
	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// mdCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// mdCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	// mdCmd.PersistentFlags().StringVar(&yamlFile, "config", "src/data/qs.yml", "config file (default is src/data/qs.yml)")
	mdCmd.PersistentFlags().StringVar(&targetFile, "target", "qs.md", "target file (default is qs.md)")
}

// const tpl = `{{ range . }}
//
// ## {{ .Tag }}
//
// {{ range . }}
//
// ### {{ .Name }}
// {{range .Qs}}
// - {{.}}{{end}}
//
// {{ end }}{{ end }}
// `

// const tpl = `{{range .}}
// ## {{.Tag}}
//
// ### {{.Type}}
// {{range .Qs}}
// - {{.}}{{end}}
//
// {{end}}`

const tpl = `{{- range .}}
## {{.Tag}}
{{- range .Types}}
### {{.Name}}
{{- range .Qs}}
- {{.}}
{{- end}}
{{- end}}
{{- end}}`

// qsCmd represents the qs command
var mdQsCmd = &cobra.Command{
	Use:   "qs",
	Short: "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {
		if !wf.Cache.Exists(cfgFile) {
			ErrorHandle(errors.New(cfgFile + "not found"))
		}
		f, err := wf.Cache.Load(cfgFile)
		if err != nil {
			return
		}
		docs := qs.NewConfigQs(f).ConvertToDocsTemps()

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
