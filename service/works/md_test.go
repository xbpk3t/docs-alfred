package works

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQA_Render(t *testing.T) {
	tests := []struct {
		name string
		want string
		qa   QA
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
				Question:     "主问题",
				Answer:       "主答案",
				SubQuestions: []string{"子问题1", "子问题2"},
			},
			want: "\n<details>\n<summary>主问题</summary>\n\n- 子问题1\n- 子问题2\n\n\n---\n\n主答案\n\n</details>\n\n",
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

// func TestParsePoetryConfig(t *testing.T) {
// 	// 读取测试文件
// 	content, err := os.ReadFile("testdata/test.yml")
// 	require.NoError(t, err)
//
// 	tests := []struct {
// 		name     string
// 		validate func(*testing.T, Docs)
// 	}{
// 		{
// 			name: "parse poetry structure",
// 			validate: func(t *testing.T, docs Docs) {
// 				// 验证文档数量
// 				assert.Len(t, docs, 4)
//
// 				// 验证第一个poetry部分
// 				assert.Equal(t, "poetry", docs[0].Category)
// 				assert.Equal(t, "verse", docs[0].Tag)
// 				assert.Len(t, docs[0].Qs, 1)
// 				assert.Equal(t, "What are common poetic devices in English poetry?", docs[0].Qs[0].Question)
//
// 				// 验证classic部分
// 				classicDoc := docs[2]
// 				assert.Equal(t, "classic", classicDoc.Category)
// 				assert.Equal(t, "literature", classicDoc.Tag)
// 				assert.Contains(t, classicDoc.Qs[0].Answer, "iambic pentameter")
// 			},
// 		},
// 		{
// 			name: "verify tag references",
// 			validate: func(t *testing.T, docs Docs) {
// 				// 验证相同tag的文档
// 				verseDocs := lo.Filter(docs, func(d Doc, _ int) bool {
// 					return d.Tag == "verse"
// 				})
// 				assert.Len(t, verseDocs, 2)
//
// 				literatureDocs := lo.Filter(docs, func(d Doc, _ int) bool {
// 					return d.Tag == "literature"
// 				})
// 				assert.Len(t, literatureDocs, 2)
// 			},
// 		},
// 		{
// 			name: "test type retrieval",
// 			validate: func(t *testing.T, docs Docs) {
// 				types := docs.GetTypes()
// 				assert.ElementsMatch(t, []string{"poetry", "classic", "modern"}, types)
//
// 				// 测试按标签获取类型
// 				literatureTypes := docs.GetTypesByTag("literature")
// 				assert.ElementsMatch(t, []string{"classic", "modern"}, literatureTypes)
// 			},
// 		},
// 		{
// 			name: "test question search",
// 			validate: func(t *testing.T, docs Docs) {
// 				results := docs.SearchQuestions("verse")
// 				assert.Len(t, results, 2)
//
// 				results = docs.SearchQuestions("poetry")
// 				assert.Contains(t, results, "What are common poetic devices in English poetry?")
// 			},
// 		},
// 	}
//
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			docs, err := parser.NewParser[Docs](content).ParseMulti()
// 			require.NoError(t, err)
// 			for _, doc := range docs {
// 				tt.validate(t, doc)
// 			}
// 		})
// 	}
// }

func TestRenderPoetryContent(t *testing.T) {
	content, err := os.ReadFile("testdata/test.yml")
	require.NoError(t, err)

	tests := []struct {
		validate func(*testing.T, string)
		name     string
	}{
		{
			name: "render markdown structure",
			validate: func(t *testing.T, output string) {
				// 验证标题结构
				assert.Contains(t, output, "## verse")
				assert.Contains(t, output, "### poetry")
				assert.Contains(t, output, "## literature")

				// 验证问答格式
				assert.Contains(t, output, "<details>")
				assert.Contains(t, output, "<summary>")
				assert.Contains(t, output, "iambic pentameter")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := NewWorkRenderer()
			output, err := renderer.Render(content)
			require.NoError(t, err)
			tt.validate(t, output)
		})
	}
}
