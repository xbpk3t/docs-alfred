package cmd

import (
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/billMerge/pkg/merger"
	"github.com/xbpk3t/docs-alfred/billMerge/pkg/utils"
)

// mergeCmd represents the merge command
func createMergeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "merge",
		Short: "合并微信和支付宝账单",
		Long:  `合并微信和支付宝账单，自动处理格式并按月份拆分输出`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// 获取当前目录下的所有CSV文件
			csvFiles := utils.GetCSVFiles(".")
			xlsxFiles := utils.GetXLSXFiles(".")
			allFiles := append(csvFiles, xlsxFiles...)

			if len(allFiles) == 0 {
				return nil
			}

			var wechatFile, alipayFile string

			// 自动识别可能的微信和支付宝账单文件
			possibleWechatFiles := getPossibleFiles(allFiles, []string{"wechat", "微信", "wx"})
			possibleAlipayFiles := getPossibleFiles(allFiles, []string{"alipay", "支付宝", "zhifubao"})

			// 使用huh创建交互式表单选择文件
			form := huh.NewForm(
				huh.NewGroup(
					huh.NewSelect[string]().
						Title("选择微信账单文件").
						Options(huh.NewOptions(possibleWechatFiles...)...).
						Value(&wechatFile),
					huh.NewSelect[string]().
						Title("选择支付宝账单文件").
						Options(huh.NewOptions(possibleAlipayFiles...)...).
						Value(&alipayFile),
				),
			)

			err := form.Run()
			if err != nil {
				return err
			}

			// 执行合并操作
			return merger.MergeBills(wechatFile, alipayFile)
		},
	}
}

// getPossibleFiles 根据关键字筛选可能的文件
func getPossibleFiles(files []string, keywords []string) []string {
	var possibleFiles []string

	// 先添加匹配关键字的文件
	for _, file := range files {
		lowerFile := strings.ToLower(file)
		for _, keyword := range keywords {
			if strings.Contains(lowerFile, keyword) {
				possibleFiles = append(possibleFiles, file)
				break
			}
		}
	}

	// 如果没有匹配的文件，则添加所有文件
	if len(possibleFiles) == 0 {
		possibleFiles = files
	}

	return possibleFiles
}

// getMergeCmd returns the merge command for external registration
func getMergeCmd() *cobra.Command {
	return createMergeCmd()
}
