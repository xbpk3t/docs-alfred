package importer

import (
	"path/filepath"
	"sort"
	"time"

	"github.com/xbpk3t/docs-alfred/cmd/xzb/internal/model"
	"github.com/xbpk3t/docs-alfred/cmd/xzb/internal/parser"
	"github.com/xbpk3t/docs-alfred/cmd/xzb/internal/rules"
)

type Input struct {
	Now         time.Time
	Rules       *rules.Config
	WechatFiles []string
	AlipayFiles []string
	Limit       int
}

type Result struct {
	Transactions []model.Transaction
	SourceFiles  []string
	Files        []parser.FileResult
}

func Run(input *Input) (Result, error) {
	var parsed []model.ParsedTransaction
	var files []parser.FileResult

	wechatResult, err := parser.ParseWechatFiles(input.WechatFiles)
	if err != nil {
		return Result{}, err
	}
	parsed = append(parsed, wechatResult.Records...)
	files = append(files, wechatResult.Files...)

	alipayResult, err := parser.ParseAlipayFiles(input.AlipayFiles)
	if err != nil {
		return Result{}, err
	}
	parsed = append(parsed, alipayResult.Records...)
	files = append(files, alipayResult.Files...)

	if input.Limit > 0 && len(parsed) > input.Limit {
		parsed = parsed[:input.Limit]
	}

	transactions := parser.NormalizeTransactions(parsed, input.Now)
	transactions = rules.Apply(input.Rules, transactions)
	sort.Slice(transactions, func(i, j int) bool {
		if transactions[i].OccurredAt.Equal(transactions[j].OccurredAt) {
			return transactions[i].ID < transactions[j].ID
		}

		return transactions[i].OccurredAt.Before(transactions[j].OccurredAt)
	})

	sourceFiles := make([]string, 0, len(files))
	for i := range files {
		sourceFiles = append(sourceFiles, filepath.Base(files[i].Path))
	}

	return Result{Transactions: transactions, SourceFiles: sourceFiles, Files: files}, nil
}
