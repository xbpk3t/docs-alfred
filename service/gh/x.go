package gh

import "github.com/xbpk3t/docs-alfred/pkg/render"

type XRenderer struct {
	render.MarkdownRenderer
	Config ConfigRepos
}

func (x XRenderer) Render(data []byte) (string, error) {
	return "", nil
}
