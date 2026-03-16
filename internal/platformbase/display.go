// Package factory 提供平台任务工厂的公共实现
package platformbase

import (
	"fmt"
	"strings"

	appscheduler "task-processor/internal/app/scheduler"
)

// GetTaskTypeDisplayName 获取任务类型显示名称
func GetTaskTypeDisplayName(taskType appscheduler.TaskType) string {
	switch taskType {
	case appscheduler.TaskTypePricing:
		return "核价任务"
	case appscheduler.TaskTypeProductSync:
		return "产品同步任务"
	case appscheduler.TaskTypeInventory:
		return "库存监控任务"
	case appscheduler.TaskTypeActivity:
		return "活动报名任务"
	default:
		return string(taskType)
	}
}

// GetPlatformDisplayName 获取平台显示名称
func GetPlatformDisplayName(platform string) string {
	switch strings.ToUpper(platform) {
	case "SHEIN":
		return "SHEIN"
	case "TEMU":
		return "TEMU"
	case "AMAZON":
		return "Amazon"
	default:
		return platform
	}
}

// FormatTaskInfo 格式化任务信息
func FormatTaskInfo(config appscheduler.TaskConfig) string {
	return fmt.Sprintf("%s - %s (店铺: %d, 租户: %d)",
		GetPlatformDisplayName(config.Platform),
		GetTaskTypeDisplayName(config.TaskType),
		config.StoreID,
		config.TenantID,
	)
}
