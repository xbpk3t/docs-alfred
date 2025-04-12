package cmd

import (
	"fmt"
	"slices"
	"strings"

	"github.com/xbpk3t/docs-alfred/alfred/gh/internal/alfred"
	"github.com/xbpk3t/docs-alfred/alfred/gh/internal/cons"

	"github.com/xbpk3t/docs-alfred/pkg/parser"

	gh2 "github.com/xbpk3t/docs-alfred/service/gh"

	aw "github.com/deanishe/awgo"
	"github.com/spf13/cobra"
)

var ghCmd = &cobra.Command{
	Use:              "gh",
	Short:            "Searching from starred repositories and my repositories",
	PersistentPreRun: handlePreRun,
	Run:              handleGhCommand,
}

// 主命令处理函数
func handleGhCommand(cmd *cobra.Command, args []string) {
	builder := alfred.NewItemBuilder(wf)
	r, err := parser.NewParser[gh2.ConfigRepos](data).ParseSingle()
	if err != nil {
		builder.BuildBasicItem(
			"Error parsing config",
			err.Error(),
			"",
			cons.IconError,
		)
		wf.SendFeedback()
		return
	}

	repos := r.ToRepos()
	if repos == nil {
		builder.BuildBasicItem(
			"Invalid configuration",
			"No repositories found in config",
			"",
			cons.IconWarning,
		)
		wf.SendFeedback()
		return
	}

	if len(args) > 0 && strings.HasPrefix(args[0], "#") {
		handleTagSearch(repos, args, builder)
		return
	}

	renderRepos(repos, builder)
	handleSearchFilter(args)
	renderSearchGithub(args)
	wf.SendFeedback()
}

// 处理标签搜索
func handleTagSearch(repos gh2.Repos, args []string, builder *alfred.ItemBuilder) {
	if repos == nil {
		builder.BuildBasicItem(
			"Invalid configuration",
			"No repositories found",
			"",
			cons.IconWarning,
		)
		wf.SendFeedback()
		return
	}

	// 参数验证
	if len(args) == 0 || !strings.HasPrefix(args[0], "#") {
		tags := repos.ExtractTags()
		if len(tags) == 0 {
			builder.BuildBasicItem(
				"No tags found",
				"No tags available in repositories",
				"",
				cons.IconWarning,
			)
		} else {
			renderTagItems(tags)
		}
		wf.SendFeedback()
		return
	}

	// 提取标签
	tags := repos.ExtractTags()
	if len(tags) == 0 {
		builder.BuildBasicItem(
			"No tags found",
			"No tags available in repositories",
			"",
			cons.IconWarning,
		)
		wf.SendFeedback()
		return
	}

	ptag := strings.TrimPrefix(args[0], "#")

	// 如果输入的标签存在
	if slices.Contains(tags, ptag) {
		filteredRepos := repos.QueryReposByTag(ptag)
		if len(filteredRepos) > 0 {
			renderRepos(filteredRepos, builder)
		} else {
			builder.BuildBasicItem(
				"No repositories found",
				fmt.Sprintf("No repositories found with tag: %s", ptag),
				"",
				cons.IconWarning,
			)
		}
	} else {
		// 显示所有标签并根据输入进行过滤
		renderTagItems(tags)
		if len(ptag) > 0 {
			wf.Filter(ptag) // 使用去掉#的标签进行过滤
		}
	}

	wf.SendFeedback()
}

// 处理搜索过滤
func handleSearchFilter(args []string) {
	if len(args) > 0 {
		wf.Filter(args[0])
	}
}

// 渲染 Github 搜索项
func renderSearchGithub(args []string) {
	searchQuery := strings.Join(args, "+")
	searchTitle := fmt.Sprintf("Search Github For '%s'", strings.Join(args, " "))

	wf.NewItem("Search Github").
		Arg(fmt.Sprintf(cons.GithubSearchURL, searchQuery)).
		Valid(true).
		Icon(&aw.Icon{Value: cons.IconSearch}).
		Title(searchTitle)
}

// 构建仓库描述
func buildRepoDescription(repo gh2.Repository) string {
	var des strings.Builder

	for flag, prefix := range map[bool]string{
		repo.IsSubRepo:      "SUB",
		repo.IsReplacedRepo: "REP",
		repo.IsRelatedRepo:  "REL",
	} {
		if flag {
			des.WriteString(fmt.Sprintf("[%s#%s]", prefix, repo.MainRepo))
		}
	}

	if repo.Type != "" {
		des.WriteString(fmt.Sprintf("[%s#%s]", repo.Tag, repo.Type))
	}

	if repo.Des != "" {
		des.WriteString(fmt.Sprintf(" %s", repo.Des))
	}

	return des.String()
}

// 构建文档 URL
// 分为三种情况：
// 1、如果有qs就直接跳转到对应repo
// 2、如果是sub, rep, rel repos 就跳转到对应的主repo
// 3、如果没有qs，也没有上面这几种repos的repo（说明是某个type下面的repo），就直接跳转到type
// [2025-03-16] 现在跳转到docs，所以只需要
func buildDocsURL(repo gh2.Repository) string {
	var docsURL strings.Builder
	docsPath := wf.Config.Get("docs")

	if docsPath == "" {
		return ""
	}
	docsURL.WriteString(fmt.Sprintf("%s%s", docsPath, strings.ToLower(repo.FullName())))

	//if repo.IsSubOrDepOrRelRepo() {
	//	docsURL.WriteString(strings.ToLower(pkg.JoinSlashParts(repo.MainRepo)))
	//	return docsURL.String()
	//}
	return docsURL.String()
}

// 确定仓库图标
func determineRepoIcon(repo gh2.Repository) string {
	switch {
	case repo.Topics != nil && repo.Doc != "":
		return cons.IconQsDoc
	case repo.Topics != nil:
		return cons.IconQs
	case repo.Doc != "":
		return cons.IconDoc
	default:
		return cons.IconCheck
	}
}

// 主渲染函数
func renderRepos(repos gh2.Repos, builder *alfred.ItemBuilder) {
	for _, repo := range repos {
		item := builder.BuildBasicItem(
			repo.FullName(),
			buildRepoDescription(repo),
			repo.URL,
			determineRepoIcon(repo),
		)
		docsURL := buildDocsURL(repo)
		builder.AddRepoModifiers(item, repo, docsURL)
	}
}

// 渲染标签项
func renderTagItems(tags []string) {
	for _, tag := range tags {
		tag = fmt.Sprintf("#%s", tag)
		wf.NewItem(tag).
			Title(tag).
			Valid(false).
			Autocomplete(tag)
	}
}
