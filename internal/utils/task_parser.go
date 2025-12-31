// Package utils 提供任务解析工具，统一处理任务数据解析
package utils

import (
	"encoding/json"
	"fmt"
	"strconv"

	types "task-processor/internal/domain/model"
)

// TaskParser 任务解析器
type TaskParser struct{}

// NewTaskParser 创建任务解析器
func NewTaskParser() *TaskParser {
	return &TaskParser{}
}

// ParseTask 解析任务数据为Task结构体
func (p *TaskParser) ParseTask(taskData string) (*types.Task, error) {
	if taskData == "" {
		return nil, fmt.Errorf("任务数据不能为空")
	}

	var task types.Task
	if err := json.Unmarshal([]byte(taskData), &task); err != nil {
		return nil, fmt.Errorf("解析任务数据失败: %w", err)
	}

	return &task, nil
}

// ParseTaskMap 解析任务数据为map
func (p *TaskParser) ParseTaskMap(taskData string) (map[string]interface{}, error) {
	if taskData == "" {
		return nil, fmt.Errorf("任务数据不能为空")
	}

	var taskMap map[string]interface{}
	if err := json.Unmarshal([]byte(taskData), &taskMap); err != nil {
		return nil, fmt.Errorf("解析任务数据失败: %w", err)
	}

	return taskMap, nil
}

// ExtractTaskID 从任务数据中提取任务ID
func (p *TaskParser) ExtractTaskID(taskData string) (string, error) {
	taskMap, err := p.ParseTaskMap(taskData)
	if err != nil {
		return "", err
	}

	// 尝试多种可能的ID字段
	idFields := []string{"id", "taskId", "task_id", "ID", "TaskID"}

	for _, field := range idFields {
		if id, exists := taskMap[field]; exists {
			switch v := id.(type) {
			case string:
				return v, nil
			case float64:
				return strconv.FormatInt(int64(v), 10), nil
			case int64:
				return strconv.FormatInt(v, 10), nil
			case int:
				return strconv.Itoa(v), nil
			}
		}
	}

	return "", fmt.Errorf("无法从任务数据中提取任务ID")
}

// ExtractPriority 从任务数据中提取优先级
func (p *TaskParser) ExtractPriority(taskData string) (int, error) {
	taskMap, err := p.ParseTaskMap(taskData)
	if err != nil {
		return 0, err
	}

	// 尝试多种可能的优先级字段
	priorityFields := []string{"priority", "Priority", "task_priority"}

	for _, field := range priorityFields {
		if priority, exists := taskMap[field]; exists {
			switch v := priority.(type) {
			case float64:
				return int(v), nil
			case int64:
				return int(v), nil
			case int:
				return v, nil
			case string:
				if p, err := strconv.Atoi(v); err == nil {
					return p, nil
				}
			}
		}
	}

	return 0, nil // 默认优先级为0
}

// ExtractStoreID 从任务数据中提取店铺ID
func (p *TaskParser) ExtractStoreID(taskData string) (int64, error) {
	taskMap, err := p.ParseTaskMap(taskData)
	if err != nil {
		return 0, err
	}

	// 尝试多种可能的店铺ID字段
	storeIDFields := []string{"storeId", "store_id", "StoreID", "shopId", "shop_id"}

	for _, field := range storeIDFields {
		if storeID, exists := taskMap[field]; exists {
			switch v := storeID.(type) {
			case float64:
				return int64(v), nil
			case int64:
				return v, nil
			case int:
				return int64(v), nil
			case string:
				if id, err := strconv.ParseInt(v, 10, 64); err == nil {
					return id, nil
				}
			}
		}
	}

	return 0, fmt.Errorf("无法从任务数据中提取店铺ID")
}

// ValidateTaskData 验证任务数据的基本格式
func (p *TaskParser) ValidateTaskData(taskData string) error {
	if taskData == "" {
		return fmt.Errorf("任务数据不能为空")
	}

	// 检查是否为有效的JSON
	var temp interface{}
	if err := json.Unmarshal([]byte(taskData), &temp); err != nil {
		return fmt.Errorf("任务数据不是有效的JSON格式: %w", err)
	}

	return nil
}

// 全局解析器实例
var globalTaskParser = NewTaskParser()

// ParseTask 解析任务数据 (全局函数，便于使用)
func ParseTask(taskData string) (*types.Task, error) {
	return globalTaskParser.ParseTask(taskData)
}

// ParseTaskMap 解析任务数据为map (全局函数)
func ParseTaskMap(taskData string) (map[string]interface{}, error) {
	return globalTaskParser.ParseTaskMap(taskData)
}

// ExtractTaskID 提取任务ID (全局函数)
func ExtractTaskID(taskData string) (string, error) {
	return globalTaskParser.ExtractTaskID(taskData)
}

// ExtractPriority 提取优先级 (全局函数)
func ExtractPriority(taskData string) (int, error) {
	return globalTaskParser.ExtractPriority(taskData)
}
