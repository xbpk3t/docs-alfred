package pkg

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Tk struct {
	Name string
	URL  string
	Des  string `json:"des,omitempty"`
}

type ws struct {
	Feat string     `yaml:"feat"`
	URLs [][]string `yaml:"urls"`
}

func processYaml(data []byte) []Tk {
	var ws []ws
	err := yaml.Unmarshal(data, &ws)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	var tk []Tk
	for _, wk := range ws {
		for _, w := range wk.URLs {
			if len(w) <= 2 {
				tk = append(tk, Tk{Name: w[0], URL: w[1]})
			} else {
				tk = append(tk, Tk{Name: w[0], URL: w[1], Des: w[2]})
			}
		}
	}
	return tk
}

func x(dest string) []byte {
	file, err := os.ReadFile(dest)
	if err != nil {
		return nil
	}
	return file
}

func SaveToLocal(URL string, dest string) error {
	resp, err := http.Get(URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// bt, err := io.ReadAll(resp.Body)
	// if err != nil {
	// 	return nil
	// }
	// return bt

	// 创建本地文件
	file, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer file.Close()

	// 将响应体复制到本地文件
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func SearchWebstack(dest string, args []string) []Tk {
	tks := processYaml(x(dest))
	var searched []Tk

	if len(args) == 0 {
		return tks
	}

	// for _, tk := range tks {
	// 	for _, arg := range args {
	// 		if strings.Contains(strings.ToLower(tk.Name), strings.ToLower(arg)) || strings.Contains(strings.ToLower(tk.URL), strings.ToLower(arg)) {
	// 			searched = append(searched, tk)
	// 		}
	// 	}
	// }
	// return searched

	searched = tks
	for _, arg := range args {
		var filtered []Tk
		for _, tk := range searched {
			if strings.Contains(strings.ToLower(tk.Name), strings.ToLower(arg)) || strings.Contains(strings.ToLower(tk.URL), strings.ToLower(arg)) {
				filtered = append(filtered, tk)
			}
		}
		searched = filtered
	}

	return searched
}
