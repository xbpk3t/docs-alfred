package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/cmd/xzb/internal/aggregate"
	"github.com/xbpk3t/docs-alfred/cmd/xzb/internal/config"
	"github.com/xbpk3t/docs-alfred/cmd/xzb/internal/d1sync"
	"github.com/xbpk3t/docs-alfred/cmd/xzb/internal/importer"
	"github.com/xbpk3t/docs-alfred/cmd/xzb/internal/parser"
	"github.com/xbpk3t/docs-alfred/cmd/xzb/internal/rules"
	"github.com/xbpk3t/docs-alfred/pkg/carboninit"
	"github.com/xbpk3t/docs-alfred/pkg/validator"
)

type syncD1Flags struct {
	rulesPath        string
	accountID        string
	apiToken         string
	databaseID       string
	wranglerConfig   string
	binding          string
	wechatFiles      []string
	alipayFiles      []string
	limit            int
	dryRun           bool
	jsonOutput       bool
	confirmRealWrite bool
}

type exportSQLFlags struct {
	rulesPath   string
	outputPath  string
	wechatFiles []string
	alipayFiles []string
	limit       int
}

type runSummary struct {
	Sync      *d1sync.SyncSummary `json:"sync,omitempty"`
	Database  string              `json:"database,omitempty"`
	Files     []fileSummary       `json:"files"`
	Aggregate aggregate.Summary   `json:"aggregate"`
	DryRun    bool                `json:"dryRun"`
}

type fileSummary struct {
	Path    string `json:"path"`
	Source  string `json:"source"`
	Records int    `json:"records"`
}

func Execute() error {
	carboninit.Setup()
	validator.Setup()

	return newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "xzb",
		Short: "Private finance importer for WeChat and Alipay bills",
	}
	rootCmd.AddCommand(newSyncCmd())
	rootCmd.AddCommand(newExportCmd())
	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})

	return rootCmd
}

func newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync normalized finance transactions",
	}
	cmd.AddCommand(newSyncD1Cmd())

	return cmd
}

func newExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export normalized finance transactions",
	}
	cmd.AddCommand(newExportSQLCmd())

	return cmd
}

func newExportSQLCmd() *cobra.Command {
	var flags exportSQLFlags

	cmd := &cobra.Command{
		Use:   "sql",
		Short: "Export D1-compatible SQL for local verification",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExportSQL(cmd.Context(), &flags)
		},
	}

	cmd.Flags().StringArrayVar(&flags.wechatFiles, "wechat", nil, "WeChat bill CSV/XLSX path; repeatable")
	cmd.Flags().StringArrayVar(&flags.alipayFiles, "alipay", nil, "Alipay bill CSV path; repeatable")
	cmd.Flags().StringVar(&flags.rulesPath, "rules", "", "Rules YAML path")
	cmd.Flags().StringVar(&flags.outputPath, "out", "", "Output SQL path; stdout when omitted")
	cmd.Flags().IntVar(&flags.limit, "limit", 0, "Debug guard: only process first N parsed records")

	return cmd
}

func newSyncD1Cmd() *cobra.Command {
	var flags syncD1Flags

	cmd := &cobra.Command{
		Use:   "d1",
		Short: "Parse bills and sync transactions to Cloudflare D1",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSyncD1(cmd.Context(), &flags)
		},
	}

	cmd.Flags().StringArrayVar(&flags.wechatFiles, "wechat", nil, "WeChat bill CSV/XLSX path; repeatable")
	cmd.Flags().StringArrayVar(&flags.alipayFiles, "alipay", nil, "Alipay bill CSV path; repeatable")
	cmd.Flags().StringVar(&flags.rulesPath, "rules", "", "Rules YAML path")
	cmd.Flags().StringVar(&flags.accountID, "account-id", "", "Cloudflare account ID; fallback CLOUDFLARE_ACCOUNT_ID")
	cmd.Flags().StringVar(&flags.apiToken, "api-token", "", "Cloudflare API token; fallback CLOUDFLARE_API_TOKEN")
	cmd.Flags().StringVar(&flags.databaseID, "database-id", "", "D1 database ID; fallback D1_DATABASE_ID")
	cmd.Flags().StringVar(&flags.wranglerConfig, "wrangler-config", "", "Wrangler TOML path used to resolve the D1 database ID")
	cmd.Flags().StringVar(&flags.binding, "binding", "FINANCE_DB", "D1 binding name in wrangler.toml")
	cmd.Flags().BoolVar(&flags.dryRun, "dry-run", false, "Parse and summarize without writing D1")
	cmd.Flags().BoolVar(&flags.jsonOutput, "json", false, "Emit machine-readable summary")
	cmd.Flags().IntVar(&flags.limit, "limit", 0, "Debug guard: only process first N parsed records")
	cmd.Flags().BoolVar(&flags.confirmRealWrite, "confirm-real-write", false, "Required for non-dry-run writes")

	return cmd
}

func runSyncD1(ctx context.Context, flags *syncD1Flags) error {
	now := time.Now()
	result, err := importTransactions(flags.rulesPath, flags.wechatFiles, flags.alipayFiles, flags.limit, now)
	if err != nil {
		return err
	}

	databaseID, err := resolveDatabaseID(flags)
	if err != nil && !flags.dryRun {
		return err
	}
	if err != nil {
		databaseID = ""
	}

	summary := runSummary{
		Aggregate: aggregate.Build(result.Transactions),
		Files:     summarizeFiles(result.Files),
		DryRun:    flags.dryRun,
		Database:  databaseID,
	}

	if !flags.dryRun {
		syncSummary, err := syncD1(ctx, flags, databaseID, &result, now)
		if err != nil {
			return err
		}
		summary.Sync = &syncSummary
	}

	return writeSummary(&summary, flags.jsonOutput)
}

func runExportSQL(_ context.Context, flags *exportSQLFlags) error {
	now := time.Now()
	result, err := importTransactions(flags.rulesPath, flags.wechatFiles, flags.alipayFiles, flags.limit, now)
	if err != nil {
		return err
	}

	script, syncSummary, err := d1sync.SQLScript(result.Transactions, result.SourceFiles, now)
	if err != nil {
		return err
	}

	if flags.outputPath == "" {
		_, err = os.Stdout.WriteString(script)

		return err
	}

	if err := os.WriteFile(flags.outputPath, []byte(script), 0o600); err != nil {
		return err
	}

	summary := runSummary{
		Aggregate: aggregate.Build(result.Transactions),
		Sync:      &syncSummary,
		Files:     summarizeFiles(result.Files),
		DryRun:    true,
	}

	return writeSummary(&summary, false)
}

func importTransactions(
	rulesPath string,
	wechatFiles []string,
	alipayFiles []string,
	limit int,
	now time.Time,
) (importer.Result, error) {
	if len(wechatFiles) == 0 && len(alipayFiles) == 0 {
		return importer.Result{}, errors.New("at least one --wechat or --alipay file is required")
	}
	if rulesPath == "" {
		return importer.Result{}, errors.New("--rules is required")
	}

	rulesConfig, err := rules.Load(rulesPath)
	if err != nil {
		return importer.Result{}, fmt.Errorf("load rules: %w", err)
	}

	return importer.Run(&importer.Input{
		WechatFiles: wechatFiles,
		AlipayFiles: alipayFiles,
		Rules:       rulesConfig,
		Now:         now,
		Limit:       limit,
	})
}

func syncD1(
	ctx context.Context,
	flags *syncD1Flags,
	databaseID string,
	result *importer.Result,
	now time.Time,
) (d1sync.SyncSummary, error) {
	if !flags.confirmRealWrite {
		return d1sync.SyncSummary{}, errors.New("non-dry-run D1 sync requires --confirm-real-write")
	}
	accountID := firstNonEmpty(flags.accountID, os.Getenv("CLOUDFLARE_ACCOUNT_ID"))
	apiToken := firstNonEmpty(flags.apiToken, os.Getenv("CLOUDFLARE_API_TOKEN"))
	if accountID == "" {
		return d1sync.SyncSummary{}, errors.New("--account-id or CLOUDFLARE_ACCOUNT_ID is required for D1 writes")
	}
	if apiToken == "" {
		return d1sync.SyncSummary{}, errors.New("--api-token or CLOUDFLARE_API_TOKEN is required for D1 writes")
	}

	queryer := d1sync.NewCloudflareQueryer(accountID, apiToken, databaseID)

	return d1sync.Sync(ctx, queryer, result.Transactions, result.SourceFiles, now)
}

func resolveDatabaseID(flags *syncD1Flags) (string, error) {
	databaseID := firstNonEmpty(flags.databaseID, os.Getenv("D1_DATABASE_ID"))
	if databaseID != "" {
		return databaseID, nil
	}
	if flags.wranglerConfig != "" {
		return config.D1DatabaseID(flags.wranglerConfig, flags.binding)
	}

	return "", errors.New("--database-id, D1_DATABASE_ID, or --wrangler-config is required")
}

func summarizeFiles(files []parser.FileResult) []fileSummary {
	summaries := make([]fileSummary, 0, len(files))
	for i := range files {
		file := &files[i]
		summaries = append(summaries, fileSummary{
			Path:    file.Path,
			Source:  string(file.Source),
			Records: file.Records,
		})
	}

	return summaries
}

func writeSummary(summary *runSummary, asJSON bool) error {
	if asJSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")

		return encoder.Encode(summary)
	}

	mode := "dry-run"
	if !summary.DryRun {
		mode = "write"
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "xzb %s summary\n", mode)
	fmt.Fprintf(&builder, "records: %d\n", summary.Aggregate.Records)
	fmt.Fprintf(&builder, "income: %s\n", formatCents(summary.Aggregate.TotalIncomeCents))
	fmt.Fprintf(&builder, "expense: %s\n", formatCents(summary.Aggregate.TotalExpenseCents))
	fmt.Fprintf(&builder, "budget expense: %s\n", formatCents(summary.Aggregate.BudgetCents))
	if summary.Database != "" {
		fmt.Fprintf(&builder, "database: %s\n", summary.Database)
	}
	for i := range summary.Aggregate.Months {
		month := &summary.Aggregate.Months[i]
		fmt.Fprintf(&builder, "month %s: income=%s expense=%s budget=%s records=%d\n",
			month.Month,
			formatCents(month.IncomeCents),
			formatCents(month.ExpenseCents),
			formatCents(month.BudgetCents),
			month.TransactionCount)
	}
	if summary.Sync != nil {
		fmt.Fprintf(&builder, "sync batch: %s processed=%d rowsWritten=%d\n",
			summary.Sync.BatchID,
			summary.Sync.Processed,
			summary.Sync.RowsWritten)
	}

	_, err := os.Stdout.WriteString(builder.String())

	return err
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}

	return ""
}

func formatCents(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}

	return fmt.Sprintf("%s%d.%02d", sign, cents/100, cents%100)
}
