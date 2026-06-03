package cmd

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/pkg/checkutil"
	"github.com/xbpk3t/docs-alfred/pkg/data"
	ghcheck "github.com/xbpk3t/docs-alfred/pkg/gh"
	"github.com/xbpk3t/docs-alfred/pkg/images"
)

type dataRenderFlags struct {
	config  string
	extract string
	out     string
}

func newDataCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "data",
		Short: "Data rendering and validation commands",
	}

	cmd.AddCommand(newDataRenderCmd())

	// Domain-first subcommands: data <domain> check/duplicate
	for _, d := range data.AllDataDomains {
		if d == data.DomainGH {
			continue // gh has its own command with find/append-record
		}
		cmd.AddCommand(newDomainCmd(d))
	}

	cmd.AddCommand(newDataGhCmd())

	return cmd
}

// ---- render ----

func newDataRenderCmd() *cobra.Command {
	var flags dataRenderFlags

	cmd := &cobra.Command{
		Use:   "render",
		Short: "Render YAML data into outputs",
		RunE: func(cmd *cobra.Command, args []string) error {
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

		return data.ExtractTopics(flags.out)
	}

	configs, err := data.LoadRenderConfigs(flags.config)
	if err != nil {
		return err
	}
	data.ProcessRenderConfigs(configs)

	return nil
}

// ---- domain: data <domain> {check, duplicate} ----

// newDomainCmd creates "data <domain>" with check/duplicate subcommands.
func newDomainCmd(domain data.DataDomain) *cobra.Command {
	cmd := &cobra.Command{
		Use:   string(domain),
		Short: fmt.Sprintf("%s data operations", domain),
	}

	cmd.AddCommand(newDomainCheckCmd(domain))

	if data.IsDuplicateDomain(domain) {
		cmd.AddCommand(newDomainDuplicateCmd(domain))
	}

	return cmd
}

func newDomainCheckCmd(domain data.DataDomain) *cobra.Command {
	var dataPath, scope string

	cmd := &cobra.Command{
		Use:   cmdCheck,
		Short: fmt.Sprintf("Check %s data validity", domain),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDomainCheck(domain, dataPath, scope)
		},
	}

	cmd.Flags().StringVar(&dataPath, "path", "", "Override data directory")
	cmd.Flags().StringVar(&scope, "scope", "", "Structured data check scope")

	return cmd
}

func runDomainCheck(domain data.DataDomain, dataPath, scope string) error {
	path := dataPath
	if path == "" {
		path = data.DefaultPathForDomain(domain)
	}
	s := scope
	if s == "" {
		s = data.DefaultScopeForDomain(domain)
	}

	slog.Info("Checking domain", "domain", domain, "path", path, "scope", s)

	if data.IsStructuredCheckDomain(domain) {
		result, err := data.RunStructuredDataCheck(path, s)
		if err != nil {
			return err
		}
		checkutil.ReportIssues(result.Issues, "data check "+string(domain))
		if checkutil.HasErrors(result.Issues) {
			return fmt.Errorf("data check %s failed", domain)
		}

		return nil
	}

	// For goods/task, at least validate YAML parsing
	if domain == data.DomainGoods || domain == data.DomainTask {
		count, errs := data.ParseYAMLDir(path)
		for _, e := range errs {
			slog.Error("YAML parse error", "error", e)
		}
		if len(errs) > 0 {
			return fmt.Errorf("data check %s: %d file(s) failed YAML parsing", domain, len(errs))
		}
		slog.Info("Data check passed", "domain", domain, "files", count)

		return nil
	}

	slog.Info("Data check passed", "domain", domain)

	return nil
}

func newDomainDuplicateCmd(domain data.DataDomain) *cobra.Command {
	var dataPath string

	cmd := &cobra.Command{
		Use:   "duplicate",
		Short: fmt.Sprintf("Find duplicate %s records", domain),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDomainDuplicate(domain, dataPath)
		},
	}

	cmd.Flags().StringVar(&dataPath, "path", "", "Override data directory")

	return cmd
}

func runDomainDuplicate(domain data.DataDomain, dataPath string) error {
	path := dataPath
	if path == "" {
		path = data.DefaultPathForDomain(domain)
	}

	slog.Info("Checking duplicates", "domain", domain, "path", path)

	report, err := data.RunDuplicateCheck(path)
	if err != nil {
		return err
	}
	if len(report.URLDuplicates) == 0 && len(report.NameAuthorDuplicates) == 0 {
		slog.Info("Data duplicate passed", "domain", domain)

		return nil
	}
	fmt.Fprint(os.Stderr, data.FormatDuplicateReport(report))

	return fmt.Errorf("data duplicate %s found duplicates", domain)
}

// ---- gh ----

func newDataGhCmd() *cobra.Command {
	var ghPath, imagesDir string

	cmd := &cobra.Command{
		Use:   "gh",
		Short: "GitHub data operations",
	}

	checkCmd := &cobra.Command{
		Use:   cmdCheck,
		Short: "Check data/gh YAML entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDataGhCheck(ghPath, imagesDir)
		},
	}
	checkCmd.Flags().StringVar(&ghPath, "path", "data/gh", "Path to data/gh directory")
	checkCmd.Flags().StringVar(&imagesDir, "images-dir", "docs-images", "Path to docs-images directory")

	dupCmd := &cobra.Command{
		Use:   "duplicate",
		Short: "Find duplicate records by URL in data/gh",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDataGhDuplicate(ghPath)
		},
	}
	dupCmd.Flags().StringVar(&ghPath, "path", "data/gh", "Path to data/gh directory")

	findCmd := newDataGhFindCmd()
	appendCmd := newDataGhAppendCmd()

	cmd.AddCommand(checkCmd, dupCmd, findCmd, appendCmd)

	return cmd
}

func runDataGhCheck(path, imagesDir string) error {
	slog.Info("Checking data/gh", "path", path)

	// 1. YAML walker + entry/record validation
	result, err := ghcheck.RunGhCheck(path)
	if err != nil {
		return err
	}

	// 2. Image-dir expectation validation
	slog.Info("Checking image-dir expectations", "images-dir", imagesDir)

	imgResult, imgErr := images.RunImagesCheck(images.CheckConfig{
		DataDir:     path,
		ImagesDir:   imagesDir,
		SkipMissing: true,
		SkipExtra:   true,
	})
	if imgErr != nil {
		return imgErr
	}
	for _, w := range imgResult.Warnings {
		result.Issues = append(result.Issues, ghcheck.CheckIssue{
			File: cmdImages, Severity: "warn", Message: w,
		})
	}
	for _, e := range imgResult.Errors {
		result.Issues = append(result.Issues, ghcheck.CheckIssue{
			File: "images", Severity: "error", Message: e,
		})
	}

	result.Report("data gh check")

	if ghcheck.HasErrors(result) {
		return errors.New("data gh check failed")
	}

	return nil
}

func runDataGhDuplicate(path string) error {
	slog.Info("Finding duplicate URLs in data/gh", "path", path)
	report, err := data.RunGHDuplicateCheck(path)
	if err != nil {
		return err
	}
	if len(report.URLDuplicates) == 0 {
		slog.Info("data gh duplicate passed")

		return nil
	}
	fmt.Fprint(os.Stderr, data.FormatGHDuplicateReport(report))

	return fmt.Errorf("data gh duplicate found %d duplicate URLs", len(report.URLDuplicates))
}

// ---- gh find ----

func newDataGhFindCmd() *cobra.Command {
	var query, findURL string
	var limit int

	cmd := &cobra.Command{
		Use:   "find",
		Short: "Search local data/gh entries",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			q := query
			if q == "" && len(args) > 0 {
				q = args[0]
			}
			if q == "" && findURL == "" {
				return errors.New("provide a query, --query, or --url")
			}

			return runDataGhFind(q, findURL, limit)
		},
	}

	cmd.Flags().StringVarP(&query, "query", "q", "", "Search query")
	cmd.Flags().StringVar(&findURL, "url", "", "Repository URL to find")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max results")

	return cmd
}

func runDataGhFind(query, findURL string, limit int) error {
	ghRoot := "data/gh"
	slog.Info("Searching data/gh", "query", query, "url", findURL, "limit", limit)

	entries, err := ghcheck.FindEntries(ghRoot, query, findURL)
	if err != nil {
		return err
	}

	ghcheck.SortEntries(entries)

	if limit > 0 && limit < len(entries) {
		entries = entries[:limit]
	}

	ghcheck.FormatEntries(entries)

	return nil
}

// ---- gh append-record ----

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
		Short: "Append a record to a data/gh entry",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDataGhAppend(opts.file, opts.url, opts.date, opts.des, opts.topic)
		},
	}

	cmd.Flags().StringVar(&opts.file, "file", "", "Target YAML file path")
	cmd.Flags().StringVar(&opts.url, "url", "", "Repository URL (required unless --file is given)")
	cmd.Flags().StringVar(&opts.date, "date", "", "Record date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&opts.des, "des", "", "Record description")
	cmd.Flags().StringVar(&opts.topic, "topic", "", "Topic name (default: inferred from URL)")

	return cmd
}

func runDataGhAppend(file, url, date, des, topic string) error {
	if url == "" && file == "" {
		return errors.New("either --url or --file is required")
	}
	if date == "" || des == "" {
		return errors.New("--date and --des are required")
	}

	slog.Info("Appending record", "url", url, "date", date, "des", des)

	result, err := ghcheck.AppendRecord(&ghcheck.AppendRecordOptions{
		File:  file,
		URL:   url,
		Date:  date,
		Des:   des,
		Topic: topic,
	})
	if err != nil {
		return fmt.Errorf("append-record failed: %w", err)
	}

	slog.Info("Record appended", "file", result.File)
	if result.Diff != "" {
		slog.Info("Git diff", "diff", result.Diff)
	}

	return nil
}
