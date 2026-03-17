// Package runner 提供处理器和调度器的运行管理功能
package runner

import (
	"time"

	"task-processor/internal/app/scheduler"
	"task-processor/internal/core/config"
)

// taskTypeConfig 任务类型配置
type taskTypeConfig struct {
	Enabled  bool // 是否启用
	Interval int  // 执行间隔（秒）
}

// platformTaskConfig 平台任务配置
type platformTaskConfig struct {
	PlatformName         string                       // 平台名称
	Enabled              bool                         // 是否启用
	AutoPricing          taskTypeConfig               // 自动核价配置
	ProductSync          taskTypeConfig               // 产品同步配置
	InventorySync        taskTypeConfig               // 库存同步配置
	ActivityRegistration taskTypeConfig               // 活动报名配置
	FactoryCreator       func() scheduler.TaskFactory // 工厂创建函数
}

// getPlatformConfigs 获取所有平台配置
func (s *schedulerServiceImpl) getPlatformConfigs(cfg *config.Config) []platformTaskConfig {
	configs := make([]platformTaskConfig, 0, 2)

	// 注意：使用 SchedulerEnabled 控制调度任务，Enabled 控制处理器（上架任务）
	if cfg.Platforms.Temu.SchedulerEnabled {
		configs = append(configs, buildPlatformTaskConfig("TEMU", cfg.Platforms.Temu, func() scheduler.TaskFactory {
			return s.createTemuFactory(cfg)
		}))
	}

	if cfg.Platforms.Shein.SchedulerEnabled {
		configs = append(configs, buildPlatformTaskConfig("SHEIN", cfg.Platforms.Shein, func() scheduler.TaskFactory {
			return s.createSheinFactory(cfg)
		}))
	}

	return configs
}

// buildPlatformTaskConfig 从通用的 PlatformConfig 构建 platformTaskConfig
func buildPlatformTaskConfig(name string, pc config.PlatformConfig, factory func() scheduler.TaskFactory) platformTaskConfig {
	return platformTaskConfig{
		PlatformName: name,
		Enabled:      true,
		AutoPricing: taskTypeConfig{
			Enabled:  pc.AutoPricing.Enabled,
			Interval: pc.AutoPricing.Interval,
		},
		ProductSync: taskTypeConfig{
			Enabled:  pc.ProductSync.Enabled,
			Interval: pc.ProductSync.Interval,
		},
		InventorySync: taskTypeConfig{
			Enabled:  pc.InventorySync.Enabled,
			Interval: pc.InventorySync.Interval,
		},
		ActivityRegistration: taskTypeConfig{
			Enabled:  pc.ActivityRegistration.Enabled,
			Interval: pc.ActivityRegistration.Interval,
		},
		FactoryCreator: factory,
	}
}

// getDefaultInterval 获取默认间隔
func getDefaultInterval(interval int) time.Duration {
	if interval <= 0 {
		return 86400 * time.Minute // 默认24小时
	}
	return time.Duration(interval) * time.Second
}
