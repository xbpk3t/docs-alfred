package task

import (
	"fmt"
	"sort"
	"time"

	"github.com/mitchellh/mapstructure"

	"github.com/xbpk3t/docs-alfred/pkg/render"
	"github.com/xbpk3t/docs-alfred/service"
	"sigs.k8s.io/yaml"
)

// TaskYAMLRender 任务 YAML 渲染器
type TaskYAMLRender struct {
	*render.YAMLRenderer
}

// NewTaskYAMLRender 创建新的任务 YAML 渲染器
func NewTaskYAMLRender() *TaskYAMLRender {
	return &TaskYAMLRender{
		YAMLRenderer: render.NewYAMLRenderer(string(service.ServiceTask), true),
	}
}

// Flatten 将数据打平成一层
func (j *TaskYAMLRender) Flatten(data []byte) ([]interface{}, error) {
	raw, err := j.ParseData(data)
	if err != nil {
		return nil, err
	}

	result := make([]interface{}, 0)

	// 处理顶层数据
	switch v := raw.(type) {
	case []interface{}:
		// 递归处理每个元素
		for _, item := range v {
			if nestedSlice, ok := item.([]interface{}); ok {
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

// processTasksWithParentID recursively processes tasks and assigns parent IDs
func processTasksWithParentID(task *Task) {
	if len(task.Sub) > 0 {
		// Sort sub tasks by date ascending
		sort.Slice(task.Sub, func(i, j int) bool {
			if task.Sub[i].Date == "" {
				return false
			}
			if task.Sub[j].Date == "" {
				return true
			}
			dateI, _ := time.Parse("2006-01-02", task.Sub[i].Date)
			dateJ, _ := time.Parse("2006-01-02", task.Sub[j].Date)
			return dateI.Before(dateJ)
		})

		// Assign parent ID to all sub tasks
		for i := range task.Sub {
			task.Sub[i].Pid = task.Pid
			processTasksWithParentID(&task.Sub[i])
		}
	}
}

// Render 渲染任务数据
func (r *TaskYAMLRender) Render(data []byte) (string, error) {
	// 先使用基础的 YAML 渲染
	content, err := r.Flatten(data)
	if err != nil {
		return "", fmt.Errorf("base render error: %w", err)
	}

	var tasks Tasks
	for _, c := range content {
		task := &Task{}
		err = mapstructure.Decode(c, task)
		if err != nil {
			return "", fmt.Errorf("mapstructure decode %s error: %w", task.Task, err)
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
	var interfaceContent []interface{}
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
