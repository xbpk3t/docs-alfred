package alfred

import (
	"fmt"
	aw "github.com/deanishe/awgo"
	"github.com/xbpk3t/docs-alfred/pkg/gh"
)

type ItemBuilder struct {
	wf *aw.Workflow
}

func NewItemBuilder(wf *aw.Workflow) *ItemBuilder {
	return &ItemBuilder{wf: wf}
}

func (b *ItemBuilder) BuildBasicItem(name, des, url, iconPath string) *aw.Item {
	return b.wf.NewItem(name).
		Title(name).
		Subtitle(des).
		Valid(true).
		Quicklook(url).
		Autocomplete(name).
		Arg(url).
		Icon(&aw.Icon{Value: iconPath})
}

func (b *ItemBuilder) AddCommonModifiers(item *aw.Item, url, des string) {
	item.Cmd().Subtitle(fmt.Sprintf("Quicklook: %s", url)).Arg(des)
	item.Opt().Subtitle(fmt.Sprintf("Copy URL: %s", url)).Arg(url)
}

func (b *ItemBuilder) AddRepoModifiers(item *aw.Item, repo gh.Repository, docsURL string) {
	item.Cmd().Subtitle(fmt.Sprintf("打开该Repo在Docs的URL: %s", docsURL)).Arg(docsURL)
	item.Opt().Subtitle(fmt.Sprintf("复制URL: %s", repo.URL)).Arg(repo.URL)
	item.Shift().Subtitle(fmt.Sprintf("打开文档: %s", repo.Doc)).Arg(repo.Doc)
}
