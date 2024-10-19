package cmd

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"github.com/golang-module/carbon/v2"
	"html/template"
	"log"
	"log/slog"
	"os"
	"sync"

	"github.com/resend/resend-go/v2"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/feeds-merge/pkg"
	"github.com/xbpk3t/docs-alfred/utils"
)

var wg sync.WaitGroup

//go:embed templates/newsletter.tpl
var newsletterTpl embed.FS

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "rss2newsletter",
	Short: "merge rss feeds and send newsletter",
	Run: func(cmd *cobra.Command, args []string) {
		f := pkg.NewConfig(cfgFile, feedFile)

		var res pkg.Categories

		for _, cate := range f.Categories {
			wg.Add(1)
			go func(cate pkg.Category) {
				defer wg.Done()
				TypeName := cate.Type
				feeds := cate.URLs

				// 拼接urls
				urls := lo.Map(feeds, func(item pkg.Feed, index int) string {
					return item.Feed
				})

				// 移除一些feed为空字符串的item
				urls = lo.Compact(urls)

				allFeeds := f.FetchURLs(urls)
				if len(allFeeds) == 0 {
					slog.Info("No feed Found", slog.String("Feed Type:", TypeName))
					return
				}

				// 使用MergeAllFeeds合并feeds
				combinedFeed, err := f.MergeAllFeeds(TypeName, allFeeds)
				if err != nil {
					slog.Info("Merge Feeds Error:", slog.Any("Error", err))
					return
				}

				// 将合并后的Feed转换为所需的Feed格式，并填充Des和URL字段
				newFeeds := make([]pkg.Feed, len(combinedFeed.Items))
				for i, item := range combinedFeed.Items {

					var title string
					if !f.Newsletter.IsHideAuthorInTitle && item.Author.Name != "" {
						title = fmt.Sprintf("[%s] %s", item.Author.Name, item.Title)
					} else {
						title = item.Title
					}

					newFeeds[i] = pkg.Feed{
						Feed: title,
						Des:  title, // 这里假设描述就是标题，你可以根据实际情况调整
						URL:  item.Link.Href,
						Name: TypeName, // 使用分类的Type作为Name
						Date: utils.FormatDate(item.Created),
					}
				}

				// 将新的Feeds添加到结果中
				res = append(res, pkg.Category{
					Type: TypeName,
					URLs: newFeeds,
				})
			}(cate)
		}
		wg.Wait()

		// 从嵌入的文件系统加载模板
		tmpl, err := template.ParseFS(newsletterTpl, "templates/newsletter.tpl")
		if err != nil {
			log.Fatalf("[newsletter] Parse template error: %v", err)
		}

		// 创建一个用于存储模板渲染结果的缓冲区
		var tplBytes bytes.Buffer
		// 执行模板渲染，将结果写入缓冲区
		if err := tmpl.Execute(&tplBytes, res); err != nil {
			log.Fatalf("[newsletter] Render template error: %v", err)
		}
		// 渲染后的字符串现在存储在 tplBytes 中
		renderedString := tplBytes.String()

		// 打印出渲染后的字符串，或者根据需要进行其他操作
		fmt.Println(renderedString)

		sendMailByResend(f.Resend.Token, renderedString)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

var (
	cfgFile  string
	feedFile string
)

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.rss2newsletter.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "rss2newsletter.yml", "config file path")
	rootCmd.PersistentFlags().StringVar(&feedFile, "feed", "ws.yml", "feed file path")
}

func sendMailByResend(token, renderedString string) {
	ctx := context.TODO()
	client := resend.NewClient(token)

	params := &resend.SendEmailRequest{
		From:    "Acme <onboarding@resend.dev>",
		To:      []string{"jeffcottlu@gmail.com"},
		Subject: fmt.Sprintf("new items on %s (w%d)", carbon.Now().ToDateString(), utils.WeekNumOfYear()),
		Html:    renderedString,
	}

	sent, err := client.Emails.SendWithContext(ctx, params)
	if err != nil {
		panic(err)
	}
	fmt.Println(sent.Id)
}
