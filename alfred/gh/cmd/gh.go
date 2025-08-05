package cmd

import (
	"fmt"
	"strings"

	"github.com/xbpk3t/docs-alfred/alfred/gh/internal/alfred"
	"github.com/xbpk3t/docs-alfred/alfred/gh/internal/cons"

	"github.com/xbpk3t/docs-alfred/pkg/parser"

	gh2 "github.com/xbpk3t/docs-alfred/service/gh"

	aw "github.com/deanishe/awgo"
	"github.com/spf13/cobra"
)

type GithubParam string

const (
	ParamRepo GithubParam = "repo"
	ParamTag  GithubParam = "tag"
	ParamType GithubParam = "type"
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
		renderTags(repos, builder)
		return
	}

	// 提取标签和类型
	parts := strings.Split(args[0], "#")
	if len(parts) < 2 {
		renderTags(repos, builder)
		return
	}

	tag := parts[1]
	if tag == "" {
		renderTags(repos, builder)
		return
	}

	// 如果只有一个#，显示该tag下的所有type
	if len(parts) == 2 {
		renderTypes(repos, tag, builder)
		return
	}

	// 如果有两个#，显示该tag和type下的所有repo
	typeName := parts[2]
	if typeName == "" {
		renderTypes(repos, tag, builder)
		return
	}

	// 显示该tag和type下的所有repo
	renderReposByTagAndType(repos, tag, typeName, builder)
}

// 渲染标签
func renderTags(repos gh2.Repos, builder *alfred.ItemBuilder) {
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
}

// 渲染类型
func renderTypes(repos gh2.Repos, tag string, builder *alfred.ItemBuilder) {
	types := repos.ExtractTypesByTag(tag)
	if len(types) == 0 {
		builder.BuildBasicItem(
			"No types found",
			fmt.Sprintf("No types found for tag: %s", tag),
			"",
			cons.IconWarning,
		)
	} else {
		for _, t := range types {
			docsURL := buildDocsURL(ParamType, t)
			item := builder.BuildBasicItem(
				fmt.Sprintf("#%s#%s", tag, t),
				fmt.Sprintf("Type: %s", t),
				docsURL,
				cons.IconTypes,
			)

			if docsURL != "" {
				item.Cmd().Subtitle(fmt.Sprintf("Open type: %s", docsURL)).Arg(docsURL)
			}
		}
	}
	wf.SendFeedback()
}

// 渲染指定标签和类型的仓库
func renderReposByTagAndType(repos gh2.Repos, tag, typeName string, builder *alfred.ItemBuilder) {
	filteredRepos := repos.QueryReposByTagAndType(tag, typeName)
	if len(filteredRepos) > 0 {
		renderRepos(filteredRepos, builder)
	} else {
		builder.BuildBasicItem(
			"No repositories found",
			fmt.Sprintf("No repositories found with tag: %s and type: %s", tag, typeName),
			"",
			cons.IconWarning,
		)
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
func buildDocsURL(paramType GithubParam, value string) string {
	var docsURL strings.Builder
	docsPath := wf.Config.Get("docs")

	// 如果 docsPath 为空，使用默认值
	if docsPath == "" {
		docsPath = "https://docs.example.com" // 这里应该设置你的默认文档路径
	}

	switch paramType {
	case ParamRepo:
		docsURL.WriteString(fmt.Sprintf("%s?repo=%s", docsPath, strings.ToLower(value)))
	case ParamTag:
		docsURL.WriteString(fmt.Sprintf("%s?tag=%s", docsPath, value))
	case ParamType:
		docsURL.WriteString(fmt.Sprintf("%s?type=%s", docsPath, value))
	}

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
// [2025-04-25] gh table从repo维度改为type维度，所以直接跳转到该repo对应的type URL
func renderRepos(repos gh2.Repos, builder *alfred.ItemBuilder) {
	for _, repo := range repos {
		tp := repo.Type
		item := builder.BuildBasicItem(
			repo.FullName(),
			buildRepoDescription(repo),
			repo.URL,
			determineRepoIcon(repo),
		)
		// if repo.IsSubOrDepOrRelRepo() {
		//	resURL = repo.MainRepo
		//}
		docsURL := buildDocsURL(ParamType, tp)
		builder.AddRepoModifiers(item, repo, docsURL)
	}
}

// 渲染标签项
func renderTagItems(tags []string) {
	for _, tag := range tags {
		ss := fmt.Sprintf("#%s", tag)
		item := wf.NewItem(ss).
			Title(ss).
			Valid(false).
			Autocomplete(ss)

		docsURL := buildDocsURL(ParamTag, tag)
		if docsURL != "" {
			item.Cmd().Subtitle(fmt.Sprintf("Open tag: %s", docsURL)).Arg(docsURL)
		}
	}
}
