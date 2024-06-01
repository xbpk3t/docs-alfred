package cmd

import (
	"fmt"
	"html/template"
	"log"
	"log/slog"
	"os"

	"github.com/hxhac/docs-alfred/pkg/qs"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const tpl = `{{- range .}}
## {{.Tag}}
{{- range .Types}}
### {{.Name}}
{{- range .Qs}}
- {{.}}
{{- end}}
{{- end}}
{{- end}}`

// mdCmd represents the md command
var mdCmd = &cobra.Command{
	Use:   "md",
	Short: "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {
		f, err := os.ReadFile(cfgFile)
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

		slog.Info("Markdown output has been written to", slog.String("File", targetFile))
	},
}

var (
	cfgFile    string
	targetFile string
)

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.AddCommand(mdCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// mdCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// mdCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "src/data/qs.yml", "config file (default is src/data/qs.yml)")
	rootCmd.PersistentFlags().StringVar(&targetFile, "target", "qs.md", "target file (default is qs.md)")
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
