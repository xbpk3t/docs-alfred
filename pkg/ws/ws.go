package ws

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"gopkg.in/yaml.v3"
)

type URL struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
	Des  string `yaml:"des,omitempty"`
}

type Webstack []struct {
	Type string `yaml:"type"`
	URLs []URL  `yaml:"urls"`
}

func NewConfigWs(data []byte) (Webstack, error) {
	var ws Webstack
	err := yaml.Unmarshal(data, &ws)
	if err != nil {
		return nil, errors.Wrap(err, "")
	}

	return ws, nil
}

func (ws Webstack) ExtractURLs() []URL {
	var tk []URL
	for _, wk := range ws {
		tk = append(tk, wk.URLs...)
	}

	return tk
}

func (ws Webstack) ExtractURLsCustomDes() []URL {
	var tk []URL
	for _, wk := range ws {
		for _, u := range wk.URLs {
			u.Des = fmt.Sprintf("[#%s] %s %s", wk.Type, u.Des, u.URL)
			tk = append(tk, u)
		}
	}

	return tk
}

// SearchWs 无限添加args
func (ws Webstack) SearchWs(args []string) []URL {
	var searched []URL

	urls := ws.ExtractURLsCustomDes()

	if len(args) == 0 {
		return urls
	}

	searched = urls
	for _, arg := range args {
		var filtered []URL
		for _, tk := range searched {
			arg = strings.ToLower(arg)
			name := strings.ToLower(tk.Name)
			// url := strings.ToLower(tk.URL)
			des := strings.ToLower(tk.Des)
			if strings.Contains(name, arg) || strings.Contains(des, arg) {
				filtered = append(filtered, tk)
			}
		}
		searched = filtered
	}

	return searched
}
