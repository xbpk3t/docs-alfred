package goods

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/xbpk3t/docs-alfred/utils"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

type ConfigGoods []ConfigGoodsX

type ConfigGoodsX struct {
	Type  string   `yaml:"type"`
	Tag   string   `yaml:"tag"`
	Goods []GoodsX `yaml:"goods"`
	Des   string   `yaml:"des,omitempty"`
	Qs    []Qs     `yaml:"qs,omitempty"`
}

type GoodsX struct {
	Name  string   `yaml:"name"`
	Param string   `yaml:"param,omitempty"`
	Price string   `yaml:"price,omitempty"`
	Des   string   `yaml:"des,omitempty"`
	URL   string   `yaml:"url,omitempty"`
	Date  []string `yaml:"date,omitempty"`
	Use   bool     `yaml:"use,omitempty"`
}

type Qs struct {
	Q string   `yaml:"q"`
	X string   `yaml:"x"`
	S []string `yaml:"s"`
}

func NewConfigGoods(f []byte) (gk ConfigGoods) {
	d := yaml.NewDecoder(bytes.NewReader(f))
	for {
		// create new spec here
		spec := new(ConfigGoods)
		// pass a reference to spec reference
		if err := d.Decode(&spec); err != nil {
			// break the loop in case of EOF
			if errors.Is(err, io.EOF) {
				break
			}
			panic(err)
		}
		gk = append(gk, *spec...)
	}

	return gk
}

// AddMarkdownFormat converts a ConfigGoodsX item to a Markdown formatted string
func AddMarkdownFormat(gi ConfigGoodsX) string {
	var res strings.Builder
	for _, gd := range gi.Goods {
		summary := createMarkdownSummary(gd)
		details := createMarkdownDetails(gd)

		if details == "" {
			res.WriteString(fmt.Sprintf("- %s\n", summary))
		} else {
			res.WriteString(utils.RenderMarkdownFold(summary, details))
		}
	}
	return res.String()
}

// createMarkdownSummary formats the summary for a goods item
func createMarkdownSummary(gd GoodsX) string {
	mark := "~~"
	if gd.Use {
		mark = "***"
	}

	if gd.URL != "" {
		return fmt.Sprintf("%s[%s](%s)%s", mark, gd.Name, gd.URL, mark)
	}
	return fmt.Sprintf("%s%s%s", mark, gd.Name, mark)
}

// createMarkdownDetails formats the details for a goods item
func createMarkdownDetails(gd GoodsX) string {
	var details strings.Builder
	if gd.Param != "" {
		details.WriteString(fmt.Sprintf("- 参数: %s\n", gd.Param))
	}
	if gd.Price != "" {
		details.WriteString(fmt.Sprintf("- 价格: %s\n", gd.Price))
	}
	if gd.Date != nil {
		details.WriteString(fmt.Sprintf("- 购买时间: %s\n", strings.Join(gd.Date, ", ")))
	}
	if gd.Des != "" {
		details.WriteString("\n---\n")
		details.WriteString(gd.Des)
	}

	return details.String()
}

func AddTypeQs(gi ConfigGoodsX) string {
	var res strings.Builder
	if len(gi.Qs) == 0 {
		return ""
	}

	res.WriteString("--- \n")
	res.WriteString(":::tip \n")

	for _, q := range gi.Qs {
		details := formatDetailsWithWs(q)
		if details != "" {
			res.WriteString(utils.RenderMarkdownFold(q.Q, details))
		} else {
			res.WriteString(fmt.Sprintf("- %s \n", q.Q))
		}
	}
	res.WriteString("\n\n:::\n\n")

	return res.String()
}

func formatDetailsWithWs(q Qs) string {
	var parts []string

	if len(q.S) != 0 {
		var b strings.Builder
		for _, t := range q.S {
			b.WriteString(fmt.Sprintf("- %s\n", t))
		}
		parts = append(parts, b.String())
	}

	if len(q.S) != 0 && q.X != "" {
		parts = append(parts, "---")
	}

	if q.X != "" {
		parts = append(parts, q.X)
	}

	return strings.Join(parts, "\n\n")
}
