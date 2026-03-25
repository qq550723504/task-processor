// Package runner 提供处理器和调度器的运行管理功能
package runner

import (
	"task-processor/internal/app/scheduler"
	"task-processor/internal/core/config"
	sheinscheduler "task-processor/internal/shein/scheduler"
	temuscheduler "task-processor/internal/temu/scheduler"
)

// TaskFactoryCreator 定义平台任务工厂的创建函数。
type TaskFactoryCreator func(cfg *config.Config) scheduler.TaskFactory

// SchedulerDependencies 描述调度服务需要的可注入依赖。
type SchedulerDependencies struct {
	TemuFactoryCreator  TaskFactoryCreator
	SheinFactoryCreator TaskFactoryCreator
}

func (s *schedulerServiceImpl) resolveTemuFactoryCreator() TaskFactoryCreator {
	if s.temuFactoryCreator != nil {
		return s.temuFactoryCreator
	}
	return func(cfg *config.Config) scheduler.TaskFactory {
		if cfg.Amazon.Enabled && s.amazonProcessor != nil {
			s.logger.Info("TEMU 启用 Amazon 库存监控")
		}

		return temuscheduler.NewTemuTaskFactory(
			s.managementClient,
			s.amazonProcessor,
			&cfg.Amazon,
			&cfg.Platforms.Temu.Monitor,
			s.rabbitmqClient,
		)
	}
}

func (s *schedulerServiceImpl) resolveSheinFactoryCreator() TaskFactoryCreator {
	if s.sheinFactoryCreator != nil {
		return s.sheinFactoryCreator
	}
	return func(cfg *config.Config) scheduler.TaskFactory {
		if cfg.Amazon.Enabled && s.amazonProcessor != nil {
			s.logger.Info("SHEIN 启用 Amazon 库存监控")
		}

		return sheinscheduler.NewSheinTaskFactory(
			s.managementClient,
			s.amazonProcessor,
			&cfg.Amazon,
			&cfg.Platforms.Shein.Monitor,
			s.rabbitmqClient,
		)
	}
}
