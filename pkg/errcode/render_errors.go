package errcode

var (
	// 渲染错误码 (13000-13999)
	ErrRender         = NewError(13000, "渲染失败")
	ErrRenderTemplate = NewError(13001, "渲染模板失败")
	ErrEncodeYAML     = NewError(13002, "编码YAML失败")
	ErrEncodeJSON     = NewError(13003, "编码JSON失败")
	ErrRenderMarkdown = NewError(13004, "渲染Markdown失败")
)
