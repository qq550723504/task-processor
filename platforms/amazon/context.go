package amazon

import (
	"context"
	"task-processor/common/types"
)

// TaskContext 任务上下文
type TaskContext struct {
	Context context.Context
	Task    *types.Task
	Data    map[string]interface{}
}

// SetData 设置上下文数据
func (c *TaskContext) SetData(key string, value interface{}) {
	c.Data[key] = value
}

// GetData 获取上下文数据
func (c *TaskContext) GetData(key string) (interface{}, bool) {
	value, exists := c.Data[key]
	return value, exists
}

// GetString 获取字符串类型数据
func (c *TaskContext) GetString(key string) string {
	if value, exists := c.Data[key]; exists {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return ""
}

// GetInt 获取整数类型数据
func (c *TaskContext) GetInt(key string) int {
	if value, exists := c.Data[key]; exists {
		if num, ok := value.(int); ok {
			return num
		}
	}
	return 0
}

// GetBool 获取布尔类型数据
func (c *TaskContext) GetBool(key string) bool {
	if value, exists := c.Data[key]; exists {
		if b, ok := value.(bool); ok {
			return b
		}
	}
	return false
}
