package work

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xbpk3t/docs-a
	"github.com/xbpk3t/docs-alfred/utils"
	"testing"
)

func TestParseConfig(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Docs
		wantErr bool
	}{
		{
			name: "基础配置解析",
			input: `
- type: "类型1"
  tag: "标签1"
  qs:
    - q: "问题1"
      x: "答案1"
    - q: "问题2"
      x: "答案2"
      u: "http://example.com"`,
			want: Docs{
				{
					Type: "类型1",
					Tag:  "标签1",
					Qs: []QA{
						{Question: "问题1", Answer: "答案1"},
						{Question: "问题2", Answer: "答案2", URL: "http://example.com"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "带子问题和图片的配置",
			input: `
- type: "类型2"
  tag: "标签2"
  qs:
    - q: "主问题"
      x: "主答案"
      s: ["子问题1", "子问题2"]
      p: ["image1.jpg", "image2.jpg"]`,
			want: Docs{
				{
					Type: "类型2",
					Tag:  "标签2",
					Qs: []QA{
						{
							Question: "主问题",
							Answer:   "主答案",
							SubQs:    []string{"子问题1", "子问题2"},
							Pictures: []string{"image1.jpg", "image2.jpg"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "无效的YAML",
			input:   `invalid: yaml: [`,
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// got, err := ParseConfig([]byte(tt.input))
			got, err := utils.Parse[Doc]([]byte(tt.input))
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestQA_Render(t *testing.T) {
	tests := []struct {
		name string
		qa   QA
		want string
	}{
		{
			name: "基础问答",
			qa: QA{
				Question: "问题1",
				Answer:   "答案1",
			},
			want: "\n<details>\n<summary>问题1</summary>\n\n答案1\n\n</details>\n\n",
		},
		{
			name: "带URL的问答",
			qa: QA{
				Question: "问题2",
				Answer:   "答案2",
				URL:      "http://example.com",
			},
			want: "\n<details>\n<summary>[问题2](http://example.com)</summary>\n\n答案2\n\n</details>\n\n",
		},
		{
			name: "带子问题的问答",
			qa: QA{
				Question: "主问题",
				Answer:   "主答案",
				SubQs:    []string{"子问题1", "子问题2"},
			},
			want: "\n<details>\n<summary>主问题</summary>\n\n- 子问题1\n- 子问题2\n\n---\n\n主答案\n\n</details>\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.qa.Render()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDocs_GetTypes(t *testing.T) {
	tests := []struct {
		name string
		docs Docs
		want []string
	}{
		{
			name: "获取不同类型",
			docs: Docs{
				{Type: "类型1", Tag: "标签1"},
				{Type: "类型2", Tag: "标签1"},
				{Type: "类型1", Tag: "标签2"},
			},
			want: []string{"类型1", "类型2"},
		},
		{
			name: "空文档",
			docs: Docs{},
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.docs.GetTypes()
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}

func TestDocs_GetTypesByTag(t *testing.T) {
	tests := []struct {
		name string
		docs Docs
		tag  string
		want []string
	}{
		{
			name: "根据标签获取类型",
			docs: Docs{
				{Type: "类型1", Tag: "标签1"},
				{Type: "类型2", Tag: "标签1"},
				{Type: "类型3", Tag: "标签2"},
			},
			tag:  "标签1",
			want: []string{"类型1", "类型2"},
		},
		{
			name: "不存在的标签",
			docs: Docs{
				{Type: "类型1", Tag: "标签1"},
			},
			tag:  "不存在",
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.docs.GetTypesByTag(tt.tag)
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}

func TestDocs_SearchQuestions(t *testing.T) {
	tests := []struct {
		name  string
		docs  Docs
		query string
		want  []string
	}{
		{
			name: "搜索问题",
			docs: Docs{
				{
					Qs: []QA{
						{Question: "如何使用Go"},
						{Question: "Go的优势"},
					},
				},
				{
					Qs: []QA{
						{Question: "Python教程"},
					},
				},
			},
			query: "go",
			want:  []string{"如何使用Go", "Go的优势"},
		},
		{
			name: "不存在的问题",
			docs: Docs{
				{
					Qs: []QA{
						{Question: "如何使用Go"},
					},
				},
			},
			query: "java",
			want:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.docs.SearchQuestions(tt.query)
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}

func TestWorkRenderer_Render(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name: "基础渲染",
			input: `
- type: "类型1"
  tag: "标签1"
  qs:
    - q: "问题1"
      x: "答案1"`,
			want:    "## 标签1\n### 类型1\n\n<details>\n<summary>问题1</summary>\n\n答案1\n\n</details>\n\n",
			wantErr: false,
		},
		{
			name: "相同标签不同类型",
			input: `
- type: "类型1"
  tag: "标签1"
  qs:
    - q: "问题1"
      x: "答案1"
- type: "类型2"
  tag: "标签1"
  qs:
    - q: "问题2"
      x: "答案2"`,
			want:    "## 标签1\n### 类型1\n\n<details>\n<summary>问题1</summary>\n\n答案1\n\n</details>\n\n### 类型2\n\n<details>\n<summary>问题2</summary>\n\n答案2\n\n</details>\n\n",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := NewWorkRenderer()
			got, err := renderer.Render([]byte(tt.input))
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
