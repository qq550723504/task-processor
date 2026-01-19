// Package scheduler 提供TEMU平台的任务工厂
package scheduler

import (
	"context"
	"fmt"

	appscheduler "task-processor/internal/app/scheduler"
	"task-processor/internal/pkg/management"
	"task-processor/internal/platforms/temu"

	"github.com/sirupsen/logrus"
)

// TemuTaskFactory TEMU平台任务工厂
type TemuTaskFactory struct {
	managementClient *management.ClientManager
	configProvider   temu.ConfigProvider
	logger           *logrus.Entry
}

// NewTemuTaskFactory 创建TEMU任务工厂
func NewTemuTaskFactory(
	managementClient *management.ClientManager,
	configProvider temu.ConfigProvider,
) *TemuTaskFactory {
	return &TemuTaskFactory{
		managementClient: managementClient,
		configProvider:   configProvider,
		logger: logrus.WithFields(logrus.Fields{
			"component": "TemuTaskFactory",
		}),
	}
}

// CreateTask 创建任务
func (f *TemuTaskFactory) CreateTask(ctx context.Context, config appscheduler.TaskConfig) (appscheduler.Task, error) {
	if config.Platform != "TEMU" {
		return nil, fmt.Errorf("不支持的平台: %s", config.Platform)
	}

	switch config.TaskType {
	case appscheduler.TaskTypePricing:
		return NewPricingTask(ctx, config, f.managementClient, f.configProvider), nil
	case appscheduler.TaskTypeProductSync:
		// 创建产品同步服务
		// TODO: 需要创建clientManager，这里暂时传nil
		return NewProductSyncTask(ctx, config, f.managementClient, nil, nil), nil
	case appscheduler.TaskTypeInventory:
		return NewInventoryTask(ctx, config, f.managementClient), nil
	case appscheduler.TaskTypeActivity:
		return NewActivityTask(ctx, config, f.managementClient), nil
	default:
		return nil, fmt.Errorf("不支持的任务类型: %s", config.TaskType)
	}
}

// SupportedPlatform 支持的平台
func (f *TemuTaskFactory) SupportedPlatform() string {
	return "TEMU"
}

// SupportedTaskTypes 支持的任务类型
func (f *TemuTaskFactory) SupportedTaskTypes() []appscheduler.TaskType {
	return []appscheduler.TaskType{
		appscheduler.TaskTypePricing,
		appscheduler.TaskTypeProductSync,
		appscheduler.TaskTypeInventory,
		appscheduler.TaskTypeActivity,
	}
}
