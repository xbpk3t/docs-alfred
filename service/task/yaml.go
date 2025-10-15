package task

import (
	"fmt"

	"github.com/mitchellh/mapstructure"

	yaml "github.com/goccy/go-yaml"
	"github.com/xbpk3t/docs-alfred/pkg/render"
	"github.com/xbpk3t/docs-alfred/service"
)

// TaskYAMLRender 任务 YAML 渲染器.
type TaskYAMLRender struct {
	*render.YAMLRenderer
}

// NewTaskYAMLRender 创建新的任务 YAML 渲染器.
func NewTaskYAMLRender() *TaskYAMLRender {
	return &TaskYAMLRender{
		YAMLRenderer: render.NewYAMLRenderer(string(service.ServiceTask), true),
	}
}

// Flatten 将数据打平成一层.
func (t *TaskYAMLRender) Flatten(data []byte) ([]any, error) {
	raw, err := t.ParseData(data)
	if err != nil {
		return nil, err
	}

	result := make([]any, 0)

	// 处理顶层数据
	switch v := raw.(type) {
	case []any:
		// 递归处理每个元素
		for _, item := range v {
			if nestedSlice, ok := item.([]any); ok {
				result = append(result, nestedSlice...)
			} else {
				result = append(result, item)
			}
		}
	default:
		return nil, fmt.Errorf("unsupported data type for flattening: %T", raw)
	}

	return result, nil
}

// Render 渲染任务数据.
func (t *TaskYAMLRender) Render(data []byte) (string, error) {
	// 先使用基础的 YAML 渲染
	content, err := t.Flatten(data)
	if err != nil {
		return "", fmt.Errorf("base render error: %w", err)
	}

	tasks := make(Tasks, 0)
	for _, c := range content {
		task := &Task{}
		config := &mapstructure.DecoderConfig{
			Result:           task,
			TagName:          "yaml",
			WeaklyTypedInput: true,
		}
		decoder, decErr := mapstructure.NewDecoder(config)
		if decErr != nil {
			return "", fmt.Errorf("create decoder error: %w", decErr)
		}
		if decodeErr := decoder.Decode(c); decodeErr != nil {
			return "", fmt.Errorf("mapstructure decode %s error: %w", task.Task, decodeErr)
		}
		tasks = append(tasks, *task)
	}

	// Apply all options to the tasks
	tasks.ApplyOptions(
		WithParentID(),             // Set parent IDs
		SortMainTasksByDate(false), // Sort main tasks by date descending
		SortSubTasksByDate(false),  // Sort sub-tasks by date ascending
	)

	// Convert back to interface{} for YAML marshaling
	var interfaceContent []any
	err = mapstructure.Decode(tasks, &interfaceContent)
	if err != nil {
		return "", err
	}

	result, err := yaml.Marshal(interfaceContent)
	if err != nil {
		return "", err
	}

	return string(result), nil
}
