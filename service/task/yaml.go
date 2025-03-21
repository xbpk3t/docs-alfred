package task

import (
	"fmt"

	"github.com/xbpk3t/docs-alfred/pkg/render"
	"github.com/xbpk3t/docs-alfred/service"
	"sigs.k8s.io/yaml"
)

// TaskYAMLRender 任务 YAML 渲染器
type TaskYAMLRender struct {
	*render.YAMLRenderer
	parentIDField string // 父任务 ID 字段名
	taskIDField   string // 任务 ID 字段名
}

// Option 定义渲染器的配置选项
type Option func(*TaskYAMLRender)

// WithParentIDField 设置父任务 ID 字段名
func WithParentIDField(field string) Option {
	return func(r *TaskYAMLRender) {
		r.parentIDField = field
	}
}

// WithTaskIDField 设置任务 ID 字段名
func WithTaskIDField(field string) Option {
	return func(r *TaskYAMLRender) {
		r.taskIDField = field
	}
}

// NewTaskYAMLRender 创建新的任务 YAML 渲染器
func NewTaskYAMLRender(opts ...Option) *TaskYAMLRender {
	r := &TaskYAMLRender{
		YAMLRenderer:  render.NewYAMLRenderer(string(service.ServiceTask), true),
		parentIDField: "parent_id", // 默认父任务 ID 字段名
		taskIDField:   "id",        // 默认任务 ID 字段名
	}

	// 应用配置选项
	for _, opt := range opts {
		opt(r)
	}

	return r
}

// Render 渲染任务数据
func (r *TaskYAMLRender) Render(data []byte) (string, error) {
	// 先使用基础的 YAML 渲染
	content, err := r.YAMLRenderer.Render(data)
	if err != nil {
		return "", fmt.Errorf("base render error: %w", err)
	}

	// 扁平化任务结构
	flattened, err := r.flattenTasks(content)
	if err != nil {
		return "", fmt.Errorf("flatten tasks error: %w", err)
	}

	return flattened, nil
}

// flattenTasks 扁平化任务结构
func (r *TaskYAMLRender) flattenTasks(content string) (string, error) {
	// 先尝试解析为嵌套数组
	var nestedTasks [][]map[string]interface{}
	if err := yaml.Unmarshal([]byte(content), &nestedTasks); err == nil {
		// 如果成功解析为嵌套数组，展平它
		flattened := make([]map[string]interface{}, 0)
		for _, taskGroup := range nestedTasks {
			for _, task := range taskGroup {
				// 如果任务有 pid，将其用作子任务的父 ID
				parentID := ""
				if pid, ok := task["pid"].(string); ok {
					parentID = pid
				}
				flattened = append(flattened, task)

				// 处理子任务
				if subtasks, ok := task["sub"].([]interface{}); ok {
					for _, subtask := range subtasks {
						if st, ok := subtask.(map[string]interface{}); ok {
							if parentID != "" {
								st["pid"] = parentID
							}
							flattened = append(flattened, st)
						}
					}
				}
			}
		}
		result, err := yaml.Marshal(flattened)
		if err != nil {
			return "", fmt.Errorf("marshal flattened nested tasks error: %w", err)
		}
		return string(result), nil
	}

	// 如果不是嵌套数组，尝试解析为普通数组
	var tasks []map[string]interface{}
	if err := yaml.Unmarshal([]byte(content), &tasks); err != nil {
		return "", fmt.Errorf("unmarshal tasks error: %w", err)
	}

	// 扁平化处理
	flattened := make([]map[string]interface{}, 0)
	for _, task := range tasks {
		// 获取父任务 ID
		parentID := ""
		if pid, ok := task["pid"].(string); ok {
			parentID = pid
		}
		flattened = append(flattened, task)

		// 处理子任务
		if subtasks, ok := task["sub"].([]interface{}); ok {
			for _, subtask := range subtasks {
				if st, ok := subtask.(map[string]interface{}); ok {
					if parentID != "" {
						st["pid"] = parentID
					}
					flattened = append(flattened, st)
				}
			}
		}
	}

	// 重新序列化为 YAML
	result, err := yaml.Marshal(flattened)
	if err != nil {
		return "", fmt.Errorf("marshal flattened tasks error: %w", err)
	}

	return string(result), nil
}
