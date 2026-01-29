// Package service 提供调度任务工厂创建器
package service

import (
	"task-processor/internal/app/scheduler"
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	sheinscheduler "task-processor/internal/platforms/shein/scheduler"
	temuscheduler "task-processor/internal/platforms/temu/scheduler"
)

// createTemuFactory 创建TEMU任务工厂
func (s *schedulerServiceImpl) createTemuFactory(cfg *config.Config) scheduler.TaskFactory {
	// 使用注入的Amazon处理器（用于库存监控）
	var amazonProcessor *amazon.AmazonProcessor
	if cfg.Amazon.Enabled && s.amazonProcessor != nil {
		amazonProcessor = s.amazonProcessor
		s.logger.Info("✅ TEMU启用Amazon库存监控")
	}

	return temuscheduler.NewTemuTaskFactory(
		s.managementClient,
		amazonProcessor,
		&cfg.Amazon,
		&cfg.Platforms.Temu.Monitor,
	)
}

// createSheinFactory 创建SHEIN任务工厂
func (s *schedulerServiceImpl) createSheinFactory(cfg *config.Config) scheduler.TaskFactory {
	// 使用注入的Amazon处理器（用于库存监控）
	var amazonProcessor *amazon.AmazonProcessor
	if cfg.Amazon.Enabled && s.amazonProcessor != nil {
		amazonProcessor = s.amazonProcessor
		s.logger.Info("✅ SHEIN启用Amazon库存监控")
	}

	return sheinscheduler.NewSheinTaskFactory(s.managementClient, amazonProcessor, &cfg.Amazon, &cfg.Platforms.Shein.Monitor)
}
