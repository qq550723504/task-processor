// Package service 提供平台调度配置定义
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

	// TEMU 平台配置
	// 注意：使用 SchedulerEnabled 控制调度任务，Enabled 控制处理器（上架任务）
	if cfg.Platforms.Temu.SchedulerEnabled {
		temuConfig := platformTaskConfig{
			PlatformName: "TEMU",
			Enabled:      true,
			AutoPricing: taskTypeConfig{
				Enabled:  cfg.Platforms.Temu.AutoPricing.Enabled,
				Interval: cfg.Platforms.Temu.AutoPricing.Interval,
			},
			ProductSync: taskTypeConfig{
				Enabled:  cfg.Platforms.Temu.ProductSync.Enabled,
				Interval: cfg.Platforms.Temu.ProductSync.Interval,
			},
			InventorySync: taskTypeConfig{
				Enabled:  cfg.Platforms.Temu.InventorySync.Enabled,
				Interval: cfg.Platforms.Temu.InventorySync.Interval,
			},
			ActivityRegistration: taskTypeConfig{
				Enabled:  cfg.Platforms.Temu.ActivityRegistration.Enabled,
				Interval: cfg.Platforms.Temu.ActivityRegistration.Interval,
			},
			FactoryCreator: func() scheduler.TaskFactory {
				return s.createTemuFactory(cfg)
			},
		}
		configs = append(configs, temuConfig)
	}

	// SHEIN 平台配置
	// 注意：使用 SchedulerEnabled 控制调度任务，Enabled 控制处理器（上架任务）
	if cfg.Platforms.Shein.SchedulerEnabled {
		sheinConfig := platformTaskConfig{
			PlatformName: "SHEIN",
			Enabled:      true,
			AutoPricing: taskTypeConfig{
				Enabled:  cfg.Platforms.Shein.AutoPricing.Enabled,
				Interval: cfg.Platforms.Shein.AutoPricing.Interval,
			},
			ProductSync: taskTypeConfig{
				Enabled:  cfg.Platforms.Shein.ProductSync.Enabled,
				Interval: cfg.Platforms.Shein.ProductSync.Interval,
			},
			InventorySync: taskTypeConfig{
				Enabled:  cfg.Platforms.Shein.InventorySync.Enabled,
				Interval: cfg.Platforms.Shein.InventorySync.Interval,
			},
			ActivityRegistration: taskTypeConfig{
				Enabled:  cfg.Platforms.Shein.ActivityRegistration.Enabled,
				Interval: cfg.Platforms.Shein.ActivityRegistration.Interval,
			},
			FactoryCreator: func() scheduler.TaskFactory {
				return s.createSheinFactory(cfg)
			},
		}
		configs = append(configs, sheinConfig)
	}

	return configs
}

// getDefaultInterval 获取默认间隔
func getDefaultInterval(interval int) time.Duration {
	if interval <= 0 {
		return 86400 * time.Minute // 默认24小时
	}
	return time.Duration(interval) * time.Second
}
