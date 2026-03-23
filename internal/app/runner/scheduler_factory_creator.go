// Package runner 提供处理器和调度器的运行管理功能
package runner

import (
	"task-processor/internal/app/scheduler"
	"task-processor/internal/core/config"
	sheinscheduler "task-processor/internal/shein/scheduler"
	temuscheduler "task-processor/internal/temu/scheduler"
)

// createTemuFactory 创建TEMU任务工厂
func (s *schedulerServiceImpl) createTemuFactory(cfg *config.Config) scheduler.TaskFactory {
	if cfg.Amazon.Enabled && s.amazonProcessor != nil {
		s.logger.Info("✅ TEMU启用Amazon库存监控")
	}

	return temuscheduler.NewTemuTaskFactory(
		s.managementClient,
		s.amazonProcessor,
		&cfg.Amazon,
		&cfg.Platforms.Temu.Monitor,
		s.rabbitmqClient,
	)
}

// createSheinFactory 创建SHEIN任务工厂
func (s *schedulerServiceImpl) createSheinFactory(cfg *config.Config) scheduler.TaskFactory {
	if cfg.Amazon.Enabled && s.amazonProcessor != nil {
		s.logger.Info("✅ SHEIN启用Amazon库存监控")
	}

	return sheinscheduler.NewSheinTaskFactory(
		s.managementClient,
		s.amazonProcessor,
		&cfg.Amazon,
		&cfg.Platforms.Shein.Monitor,
		s.rabbitmqClient,
	)
}
