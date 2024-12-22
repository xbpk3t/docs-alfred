package gh

import "github.com/xbpk3t/docs-alfred/pkg/render"

type XRenderer struct {
	render.MarkdownRenderer
	Config ConfigRepos
}

// TODO 1、实现该方法。2、移除x的--folder参数
func (x XRenderer) Render(data []byte) (string, error) {
	return "", nil
}
