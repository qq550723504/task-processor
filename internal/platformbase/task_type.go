// Package platformbase 提供多平台通用的基础功能
package platformbase

import (
	"fmt"
	"strings"

	appscheduler "task-processor/internal/app/scheduler"
)

// TaskTypeToString 任务类型转字符串
func TaskTypeToString(taskType appscheduler.TaskType) string {
	switch taskType {
	case appscheduler.TaskTypePricing:
		return "Pricing"
	case appscheduler.TaskTypeProductSync:
		return "ProductSync"
	case appscheduler.TaskTypeInventory:
		return "Inventory"
	case appscheduler.TaskTypeActivity:
		return "Activity"
	default:
		return string(taskType)
	}
}

// StringToTaskType 字符串转任务类型
func StringToTaskType(taskTypeStr string) (appscheduler.TaskType, error) {
	switch strings.ToLower(taskTypeStr) {
	case "pricing":
		return appscheduler.TaskTypePricing, nil
	case "productsync":
		return appscheduler.TaskTypeProductSync, nil
	case "inventory":
		return appscheduler.TaskTypeInventory, nil
	case "activity":
		return appscheduler.TaskTypeActivity, nil
	default:
		return "", fmt.Errorf("未知的任务类型: %s", taskTypeStr)
	}
}

// ValidateTaskConfig 验证任务配置
func ValidateTaskConfig(config appscheduler.TaskConfig) error {
	if config.Platform == "" {
		return fmt.Errorf("平台名称不能为空")
	}
	if config.TaskType == "" {
		return fmt.Errorf("任务类型不能为空")
	}
	if config.StoreID <= 0 {
		return fmt.Errorf("店铺ID必须大于0")
	}
	if config.TenantID <= 0 {
		return fmt.Errorf("租户ID必须大于0")
	}
	return nil
}
