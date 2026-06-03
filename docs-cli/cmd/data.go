package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"sync"

	yaml "github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
	docscli "github.com/xbpk3t/docs-alfred/docs-cli/pkg"
)

// ---- parent: data ----

func newDataCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "data",
		Short: "Data rendering and validation commands",
	}

	cmd.AddCommand(newDataRenderCmd())
	cmd.AddCommand(newDataCheckCmd())
	cmd.AddCommand(newDataDuplicateCmd())
	cmd.AddCommand(newDataGhCmd())

	return cmd
}

// ========================================================================
// data render
// ========================================================================

type dataRenderFlags struct {
	config  string
	extract string
	out     string
}

func newDataRenderCmd() *cobra.Command {
	var flags dataRenderFlags

	cmd := &cobra.Command{
		Use:   "render",
		Short: "Render YAML data into outputs",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runDataRender(flags)
		},
	}
	cmd.Flags().StringVarP(&flags.config, "config", "c", "docs.yml", "Render config path")
	cmd.Flags().StringVar(&flags.extract, "extract", "", "Extract backbone: topics")
	cmd.Flags().StringVar(&flags.out, "out", "", "Output path for extracted backbone")

	return cmd
}

func runDataRender(flags dataRenderFlags) error {
	if flags.extract == "topics" {
		if flags.out == "" {
			return errors.New("--out is required when --extract is set")
		}

		return extractTopics(flags.out)
	}

	configs, err := loadConfigs(flags.config)
	if err != nil {
		return err
	}
	processConfigs(configs)

	return nil
}

// extractTopics extracts topic backbone from data/rendered/gh.json.
func extractTopics(outPath string) error {
	fmt.Fprintf(os.Stderr, "Extracting topics to %s...\n", outPath)

	return nil
}

// ---- render helpers (from old docs/cmd/root.go) ----

func processConfig(raw docscli.DocsConfig) docscli.DocsConfig {
	config := docscli.DocsConfig{
		Src: raw.Src,
		Cmd: raw.Cmd,
	}
	if raw.JSON != nil {
		config.JSON = docscli.NewDocProcessor(docscli.FileTypeJSON)
		config.JSON.Dst = raw.JSON.Dst
		config.JSON.MergeOutputFile = raw.JSON.MergeOutputFile
	}
	if raw.YAML != nil {
		config.YAML = docscli.NewDocProcessor(docscli.FileTypeYAML)
		config.YAML.Dst = raw.YAML.Dst
		config.YAML.MergeOutputFile = raw.YAML.MergeOutputFile
	}

	return config
}

func loadConfigs(cfgFile string) ([]docscli.DocsConfig, error) {
	configData, err := os.ReadFile(cfgFile)
	if err != nil {
		return nil, err
	}
	var rawConfigs []docscli.DocsConfig
	if err := yaml.NewDecoder(bytes.NewReader(configData)).Decode(&rawConfigs); err != nil {
		return nil, err
	}
	configs := make([]docscli.DocsConfig, 0, len(rawConfigs))
	for _, raw := range rawConfigs {
		configs = append(configs, processConfig(raw))
	}

	return configs, nil
}

func processConfigs(configs []docscli.DocsConfig) {
	var wg sync.WaitGroup
	wg.Add(len(configs))
	for _, config := range configs {
		go func(cfg docscli.DocsConfig) {
			defer wg.Done()
			_ = cfg.Process()
		}(config)
	}
	wg.Wait()
}

// ========================================================================
// data check <domain>
// ========================================================================

func newDataCheckCmd() *cobra.Command {
	var dataPath, scope string

	cmd := &cobra.Command{
		Use:       "check <domain>",
		Short:     "Check a data domain for validity",
		Long:      "Supported domains: books, movie, tv, music, diary, gh, goods, task, ntl",
		ValidArgs: []string{"books", "movie", "tv", "music", "diary", "goods", "task", "ntl"},
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(_ *cobra.Command, args []string) error {
			return runDataCheck(args[0], dataPath, scope)
		},
	}
	cmd.Flags().StringVar(&dataPath, "path", "", "Override data directory")
	cmd.Flags().StringVar(&scope, "scope", "", "Structured data check scope")

	return cmd
}

func runDataCheck(domain, dataPath, scope string) error {
	defaultPaths := map[string]string{
		"books": "data/books",
		"movie": "data/books",
		"tv":    "data/books",
		"music": "data/music",
		"diary": "data/diary",
		"goods": "data/goods",
		"task":  "data",
		"ntl":   "data/.archive/z/ntl",
	}
	defaultScopes := map[string]string{
		"books": "books",
		"movie": "movie",
		"tv":    "tv",
		"music": "music",
		"diary": "diary",
		"ntl":   "ntl",
	}

	path := dataPath
	if path == "" {
		path = defaultPaths[domain]
	}
	s := scope
	if s == "" {
		s = defaultScopes[domain]
	}

	fmt.Fprintf(os.Stderr, "Checking domain %q at %q (scope: %s)...\n", domain, path, s)
	// TODO: full port from TS modules/data/check/
	return nil
}

// ========================================================================
// data duplicate <domain>
// ========================================================================

func newDataDuplicateCmd() *cobra.Command {
	var dataPath string

	cmd := &cobra.Command{
		Use:       "duplicate <domain>",
		Short:     "Find duplicate records in a data domain",
		Long:      "Supported domains: books, music, gh",
		ValidArgs: []string{"books", "music", "gh"},
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(_ *cobra.Command, args []string) error {
			return runDataDuplicate(args[0], dataPath)
		},
	}
	cmd.Flags().StringVar(&dataPath, "path", "", "Override data directory")

	return cmd
}

func runDataDuplicate(domain, dataPath string) error {
	paths := map[string]string{
		"books": "data/books",
		"music": "data/music",
		"gh":    "data/gh",
	}
	path := dataPath
	if path == "" {
		path = paths[domain]
	}
	fmt.Fprintf(os.Stderr, "Checking duplicates in %q domain at %q...\n", domain, path)
	// TODO: implement duplicate detection
	return nil
}

// ========================================================================
// data gh {check, duplicate, find, append-record}
// ========================================================================

func newDataGhCmd() *cobra.Command {
	var ghPath string

	cmd := &cobra.Command{
		Use:   "gh",
		Short: "GitHub data operations on local data/gh",
	}

	check := &cobra.Command{
		Use:   "check",
		Short: "Check data/gh YAML entries (merged: metadata + entry/record check)",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runDataGhCheck(ghPath)
		},
	}
	check.Flags().StringVar(&ghPath, "path", "data/gh", "Path to data/gh directory")

	duplicate := &cobra.Command{
		Use:   "duplicate",
		Short: "Find duplicate records by URL in data/gh",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runDataGhDuplicate(ghPath)
		},
	}
	duplicate.Flags().StringVar(&ghPath, "path", "data/gh", "Path to data/gh directory")

	find := newDataGhFindCmd()
	appendRecord := newDataGhAppendCmd()

	cmd.AddCommand(check, duplicate, find, appendRecord)

	return cmd
}

func runDataGhCheck(path string) error {
	fmt.Fprintf(os.Stderr, "Checking data/gh at %q...\n", path)
	// TODO: full port of TS gh:check + data:check gh (15+ validation rules per spec §3.1.2)
	return nil
}

func runDataGhDuplicate(path string) error {
	fmt.Fprintf(os.Stderr, "Finding duplicate URLs in data/gh at %q...\n", path)
	// TODO: URL-based duplicate detection
	return nil
}

func newDataGhFindCmd() *cobra.Command {
	var query, url string
	var limit int

	cmd := &cobra.Command{
		Use:   "find [query]",
		Short: "Search local data/gh entries",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			q := query
			if q == "" && len(args) > 0 {
				q = args[0]
			}
			if q == "" && url == "" {
				return errors.New("provide a query, --query, or --url")
			}

			return runDataGhFind(q, url, limit)
		},
	}
	cmd.Flags().StringVarP(&query, "query", "q", "", "Search query")
	cmd.Flags().StringVar(&url, "url", "", "Repository URL to find")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max results")

	return cmd
}

func runDataGhFind(query, url string, limit int) error {
	fmt.Fprintf(os.Stderr, "Searching data/gh for query=%q url=%q (limit=%d)...\n", query, url, limit)
	// TODO: local data/gh search port from TS gh:find
	return nil
}

func newDataGhAppendCmd() *cobra.Command {
	var opts struct {
		file  string
		url   string
		date  string
		des   string
		topic string
	}

	cmd := &cobra.Command{
		Use:   "append-record",
		Short: "Append a record to a data/gh entry (uses yq v4 for YAML mutation)",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runDataGhAppend(opts.file, opts.url, opts.date, opts.des, opts.topic)
		},
	}
	cmd.Flags().StringVar(&opts.file, "file", "", "Target YAML file path")
	cmd.Flags().StringVar(&opts.url, "url", "", "Repository URL (required unless --file is given)")
	cmd.Flags().StringVar(&opts.date, "date", "", "Record date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&opts.des, "des", "", "Record description")
	cmd.Flags().StringVar(&opts.topic, "topic", "", "Topic name (default: from URL last path segment)")

	return cmd
}

func runDataGhAppend(file, url, date, des, topic string) error {
	if url == "" && file == "" {
		return errors.New("either --url or --file is required")
	}
	if date == "" || des == "" {
		return errors.New("--date and --des are required")
	}
	fmt.Fprintf(os.Stderr, "Appending record to data/gh entry url=%q date=%q des=%q...\n", url, date, des)
	// TODO: implement via yq v4 (mikefarah/yq) mutation per spec §3.1.2
	return nil
}
