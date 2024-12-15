package cmd

import (
	"fmt"
	"log/slog"
	"net/url"
	"path"
	"slices"
	"strings"

	aw "github.com/deanishe/awgo"
	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/pkg/gh"
	"github.com/xbpk3t/docs-alfred/utils"
)

const (
	RepoSearch = "https://github.com/search?q=%s&type=repositories"
	FaStar     = "icons/check.svg"
	FaRepo     = "icons/repo.png"
	FaSearch   = "icons/search.svg"
	FaQs       = "icons/a.svg"
	FaDoc      = "icons/b.svg"
	FaQsAndDoc = "icons/ab.svg"
)

var ghCmd = &cobra.Command{
	Use:   "gh",
	Short: "Searching from starred repositories and my repositories",
	Run:   handleGhCommand,
}

// 主命令处理函数
func handleGhCommand(cmd *cobra.Command, args []string) {
	repos := gh.NewConfigRepos(data).ToRepos()

	if len(args) > 0 && strings.HasPrefix(args[0], "#") {
		handleTagSearch(repos, args)
		return
	}

	RenderRepos(repos)
	handleSearchFilter(args)
	renderSearchGithub(args)
	wf.SendFeedback()
}

// 处理标签搜索
func handleTagSearch(repos gh.Repos, args []string) {
	tags := repos.ExtractTags()
	ptag := strings.TrimPrefix(args[0], "#")

	if slices.Contains(tags, ptag) {
		repos = repos.QueryReposByTag(ptag)
		RenderRepos(repos)
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
		Arg(fmt.Sprintf(RepoSearch, searchQuery)).
		Valid(true).
		Icon(&aw.Icon{Value: FaSearch}).
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
func createBaseItem(repo gh.Repository) *aw.Item {
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
func buildRepoDescription(repo gh.Repository) string {
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
func buildDocsURL(repo gh.Repository) string {
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
func determineRepoIcon(repo gh.Repository) string {
	switch {
	case repo.Qs != nil && repo.Doc != "":
		return FaQsAndDoc
	case repo.Qs != nil:
		return FaQs
	case repo.Doc != "":
		return FaDoc
	case repo.IsStar:
		return FaStar
	default:
		return FaRepo
	}
}

// 添加修饰键操作
func addModifierActions(item *aw.Item, repo gh.Repository, docsURL string) {
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
func RenderRepos(repos gh.Repos) (item *aw.Item) {
	for _, repo := range repos {
		item = createBaseItem(repo)
		docsURL := buildDocsURL(repo)
		addModifierActions(item, repo, docsURL)
	}
	return item
}
