package ws

import (
	"fmt"
	"github.com/xbpk3t/docs-alfred/utils"
)

type WsRenderer struct {
	utils.MarkdownRenderer
	Config Webstack
}

func (w *WsRenderer) Render(data []byte) (string, error) {
	config, _ := NewConfigWs(data)

	for _, urls := range config {
		w.RenderHeader(2, urls.Type)

		for _, url := range urls.URLs {
			// if url.Name == "" {
			// 	url.Name = url.URL
			// }
			// // res.WriteString(fmt.Sprintf("- [%s](%s) %s\n", url.Name, url.URL, url.Des))
			// if url.URL != "" {
			// 	res.WriteString(fmt.Sprintf("- [%s](%s) %s\n", url.Name, url.URL, url.Des))
			// } else {
			// 	res.WriteString(fmt.Sprintf("- [%s](%s) %s\n", url.Name, url.Feed, url.Des))
			// }

			name := url.Name
			if name == "" {
				name = url.URL
			}

			link := url.URL
			if link == "" {
				link = url.Feed
			}

			w.RenderListItem(fmt.Sprintf("%s %s", w.RenderLink(name, link), url.Des))
		}
	}
	return w.String(), nil
}
