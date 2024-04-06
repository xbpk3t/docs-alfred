package ws

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type URL struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
	Des  string `yaml:"des,omitempty"`
}

type Webstack struct {
	Type string `yaml:"type"`
	URLs []URL  `yaml:"urls"`
}

type Wss []Webstack

func NewConfigWs(data []byte) Wss {
	var ws []Webstack
	err := yaml.Unmarshal(data, &ws)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	return ws
}

func (wss Wss) ExtractURLs() []URL {
	var tk []URL
	for _, wk := range wss {
		tk = append(tk, wk.URLs...)
	}

	return tk
}

func (wss Wss) ExtractURLsCustomDes() []URL {
	var tk []URL
	for _, wk := range wss {
		for _, u := range wk.URLs {
			u.Des = fmt.Sprintf("[#%s] %s %s", wk.Type, u.Des, u.URL)
			tk = append(tk, u)
		}
	}

	return tk
}

func (wss Wss) SearchWs(args []string) []URL {
	var searched []URL

	urls := wss.ExtractURLsCustomDes()

	if len(args) == 0 {
		return urls
	}

	searched = urls
	for _, arg := range args {
		var filtered []URL
		for _, tk := range searched {
			if strings.Contains(strings.ToLower(tk.Name), strings.ToLower(arg)) || strings.Contains(strings.ToLower(tk.URL), strings.ToLower(arg)) {
				filtered = append(filtered, tk)
			}
		}
		searched = filtered
	}

	return searched
}
