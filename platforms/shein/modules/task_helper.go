package modules

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// TaskInfo contains parsed task information for logging and processing
type TaskInfo struct {
	TaskID   string
	Priority int
	TenantID string
	ShopID   string
}

// ParseTaskInfo extracts common fields from task JSON data
// This helper reduces code duplication across redis/client.go, worker/pool.go, and processor/processor.go
func ParseTaskInfo(taskData string) (*TaskInfo, error) {
	var task map[string]interface{}
	if err := json.Unmarshal([]byte(taskData), &task); err != nil {
		return nil, fmt.Errorf("parse task data failed: %w", err)
	}

	info := &TaskInfo{}

	// Extract task ID
	if taskIDValue, exists := task["taskId"]; exists {
		info.TaskID = formatTaskID(taskIDValue)
	} else {
		info.TaskID = "unknown"
	}

	// Extract priority
	if priorityValue, exists := task["priority"]; exists {
		info.Priority = formatPriority(priorityValue)
	} else {
		info.Priority = 0
	}

	// Extract tenant ID
	if tenantIDValue, exists := task["tenantId"]; exists {
		info.TenantID = formatID(tenantIDValue)
	} else {
		info.TenantID = "unknown"
	}

	// Extract shop/store ID
	if shopIDValue, exists := task["storeId"]; exists {
		info.ShopID = formatID(shopIDValue)
	} else {
		info.ShopID = "unknown"
	}

	return info, nil
}

// formatTaskID formats task ID value to string
func formatTaskID(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int64:
		return strconv.FormatInt(v, 10)
	case int:
		return strconv.Itoa(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// formatPriority formats priority value to int
func formatPriority(value interface{}) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	default:
		return 0
	}
}

// formatID formats ID value to string
func formatID(value interface{}) string {
	switch v := value.(type) {
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int64:
		return strconv.FormatInt(v, 10)
	case int:
		return strconv.Itoa(v)
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

// ExtractPriorityFromJSON extracts priority from JSON string without full parsing
// Useful for quick priority checks in Lua scripts or lightweight operations
func ExtractPriorityFromJSON(taskData string) int {
	var task map[string]interface{}
	if err := json.Unmarshal([]byte(taskData), &task); err != nil {
		return 0
	}

	if priorityValue, exists := task["priority"]; exists {
		return formatPriority(priorityValue)
	}

	return 0
}

// ExtractTaskIDFromJSON extracts task ID from JSON string without full parsing
// Useful for quick ID checks in logging or error handling
func ExtractTaskIDFromJSON(taskData string) string {
	var task map[string]interface{}
	if err := json.Unmarshal([]byte(taskData), &task); err != nil {
		return "unknown"
	}

	if taskIDValue, exists := task["taskId"]; exists {
		return formatTaskID(taskIDValue)
	}

	return "unknown"
}
