package render

// Renderer 定义通用渲染器接口
type Renderer interface {
	// Render 将输入数据渲染为指定格式
	Render(data []byte) (string, error)
}
