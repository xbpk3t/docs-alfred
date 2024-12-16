package cmd

import (
	"fmt"
	"log/slog"
	"net/url"
	"path"
	"slices"
	"strings"

	"github.com/xbpk3t/docs-alfred/alfred/internal/alfred"
	"github.com/xbpk3t/docs-alfred/alfred/internal/cons"
	gh2 "github.com/xbpk3t/docs-alfred/pkg/gh"
	"github.com/xbpk3t/docs-alfred/utils"

	aw "github.com/deanishe/awgo"
	"github.com/spf13/cobra"
)

var ghCmd = &cobra.Command{
	Use:   "gh",
	Short: "Searching from starred repositories and my repositories",
	Run:   handleGhCommand,
}

// 主命令处理函数
func handleGhCommand(cmd *cobra.Command, args []string) {
	builder := alfred.NewItemBuilder(wf)
	r, _ := gh2.ParseConfig(data)
	repos := r.ToRepos()

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
	tags := repos.ExtractTags()
	ptag := strings.TrimPrefix(args[0], "#")

	if slices.Contains(tags, ptag) {
		repos = repos.QueryReposByTag(ptag)
		renderRepos(repos, builder)
	} else {
		renderTagItems(tags)
		if len(args) > 0 {
			wf.Filter(args[0])
		}
	}
	wf.SendFeedback()
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

// URL 处理相关函数
func GetFileNameFromURL(urlString string) (string, error) {
	parsedURL, err := url.Parse(urlString)
	if err != nil {
		return "", fmt.Errorf("error parsing URL: %v", err)
	}
	return path.Base(parsedURL.Path), nil
}

// Item 创建与渲染相关函数
func createBaseItem(repo gh2.Repository) *aw.Item {
	name := repo.FullName()
	des := buildRepoDescription(repo)
	iconPath := determineRepoIcon(repo)

	return wf.NewItem(name).
		Title(name).
		Arg(repo.URL).
		Subtitle(des).
		Copytext(repo.URL).
		Valid(true).
		Autocomplete(name).
		Icon(&aw.Icon{Value: iconPath})
}

// 构建仓库描述
func buildRepoDescription(repo gh2.Repository) string {
	var des strings.Builder

	if repo.Type != "" {
		des.WriteString(fmt.Sprintf("【#%s】", repo.Type))
	} else {
		des.WriteString(repo.Des)
	}

	if repo.Des != "" {
		des.WriteString(fmt.Sprintf(" %s", repo.Des))
	}

	return des.String()
}

// 构建文档 URL
func buildDocsURL(repo gh2.Repository) string {
	var docsURL strings.Builder
	docsPath := ""

	if wf != nil {
		docsURL.WriteString(fmt.Sprintf("%s/%s#", docsPath, strings.ToLower(repo.Tag)))
	} else {
		slog.Error("wf is nil", slog.String("repo.Tag", repo.Tag))
		docsURL.WriteString(fmt.Sprintf("%s#", strings.ToLower(repo.Tag)))
	}

	if repo.Qs == nil {
		docsURL.WriteString(strings.ToLower(repo.Type))
	} else {
		docsURL.WriteString(strings.ToLower(utils.JoinSlashParts(repo.FullName())))
	}

	return docsURL.String()
}

// 确定仓库图标
func determineRepoIcon(repo gh2.Repository) string {
	switch {
	case repo.Qs != nil && repo.Doc != "":
		return cons.IconQsDoc
	case repo.Qs != nil:
		return cons.IconQs
	case repo.Doc != "":
		return cons.IconDoc
	case repo.IsStar:
		return cons.IconStar
	default:
		return cons.IconRepo
	}
}

// 添加修饰键操作
func addModifierActions(item *aw.Item, repo gh2.Repository, docsURL string) {
	item.Cmd().
		Subtitle(fmt.Sprintf("打开该Repo在Docs的URL: %s", docsURL)).
		Arg(docsURL)

	item.Opt().
		Subtitle(fmt.Sprintf("复制URL: %s", repo.URL)).
		Arg(repo.URL)

	item.Shift().
		Subtitle(fmt.Sprintf("打开文档: %s", repo.Doc)).
		Arg(repo.Doc)
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
