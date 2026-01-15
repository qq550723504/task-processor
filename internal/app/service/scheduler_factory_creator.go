// Package service 提供调度任务工厂创建器
package service

import (
	"task-processor/internal/app/scheduler"
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	sheinscheduler "task-processor/internal/platforms/shein/scheduler"
	"task-processor/internal/platforms/temu"
	temuscheduler "task-processor/internal/platforms/temu/scheduler"
)

// createTemuFactory 创建TEMU任务工厂
func (s *schedulerServiceImpl) createTemuFactory(cfg *config.Config) scheduler.TaskFactory {
	// 创建配置提供者
	var configProvider temu.ConfigProvider
	if cfg.Amazon.Enabled {
		amazonProcessor := GetSharedAmazonProcessor(cfg, s.logger)
		if amazonProcessor != nil {
			configProvider = temu.NewDefaultConfigProvider(&cfg.Amazon, amazonProcessor, &cfg.Platforms.Temu)
			s.logger.Info("✅ TEMU启用Amazon增强版核价")
		}
	}

	return temuscheduler.NewTemuTaskFactory(s.managementClient, configProvider)
}

// createSheinFactory 创建SHEIN任务工厂
func (s *schedulerServiceImpl) createSheinFactory(cfg *config.Config) scheduler.TaskFactory {
	// 获取共享的Amazon处理器（用于库存监控）
	var amazonProcessor *amazon.AmazonProcessor
	if cfg.Amazon.Enabled {
		amazonProcessor = GetSharedAmazonProcessor(cfg, s.logger)
		if amazonProcessor != nil {
			s.logger.Info("✅ SHEIN启用Amazon库存监控")
		}
	}

	return sheinscheduler.NewSheinTaskFactory(s.managementClient, amazonProcessor, &cfg.Amazon, &cfg.Platforms.Shein.Monitor)
}
