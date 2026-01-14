// Package service 提供工厂创建器
package service

import (
	"task-processor/internal/app/scheduler"
	"task-processor/internal/core/config"
	sheinscheduler "task-processor/internal/platforms/shein/scheduler"
	"task-processor/internal/platforms/temu"
	temuscheduler "task-processor/internal/platforms/temu/scheduler"
)

// createTemuFactory 创建TEMU任务工厂
func (s *pricingServiceImpl) createTemuFactory(cfg *config.Config) scheduler.TaskFactory {
	// 创建配置提供者
	var configProvider temu.ConfigProvider
	if cfg.Amazon.Enabled {
		amazonProcessor := GetSharedAmazonProcessor(cfg, s.logger)
		if amazonProcessor != nil {
			configProvider = temu.NewDefaultConfigProvider(&cfg.Amazon, amazonProcessor, &cfg.Platforms.Temu)
			s.logger.Info("✅ 启用Amazon增强版核价")
		}
	}

	return temuscheduler.NewTemuTaskFactory(s.managementClient, configProvider)
}

// createSheinFactory 创建SHEIN任务工厂
func (s *pricingServiceImpl) createSheinFactory() scheduler.TaskFactory {
	return sheinscheduler.NewSheinTaskFactory(s.managementClient)
}
