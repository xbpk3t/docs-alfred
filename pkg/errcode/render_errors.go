package errcode

const (
	ErrCodeRender = 13000 + iota
	ErrCodeRenderTemplate
	ErrCodeEncodeYAML
	ErrCodeEncodeJSON
	ErrCodeRenderMarkdown
)

// 渲染错误码 (13000-13999).
var (
	// ErrRender 渲染失败.
	ErrRender = NewError(ErrCodeRender, "渲染失败")
	// ErrRenderTemplate 渲染模板失败.
	ErrRenderTemplate = NewError(ErrCodeRenderTemplate, "渲染模板失败")
	// ErrEncodeYAML 编码YAML失败.
	ErrEncodeYAML = NewError(ErrCodeEncodeYAML, "编码YAML失败")
	// ErrEncodeJSON 编码JSON失败.
	ErrEncodeJSON = NewError(ErrCodeEncodeJSON, "编码JSON失败")
	// ErrRenderMarkdown 渲染Markdown失败.
	ErrRenderMarkdown = NewError(ErrCodeRenderMarkdown, "渲染Markdown失败")
)
