// Package runner 提供处理器和调度器的运行管理功能
package runner

import (
	"task-processor/internal/app/scheduler"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/infra/rabbitmq"
	sheinscheduler "task-processor/internal/shein/scheduler"
	temuscheduler "task-processor/internal/temu/scheduler"
)

// BuildDefaultSchedulerDependencies 为旧入口提供默认的平台工厂创建器。
func BuildDefaultSchedulerDependencies(
	managementClient *management.ClientManager,
	amazonProcessor amazonCrawler,
	rabbitmqClient *rabbitmq.Client,
) SchedulerDependencies {
	return SchedulerDependencies{
		TemuFactoryCreator: func(cfg *config.Config) scheduler.TaskFactory {
			return temuscheduler.NewTemuTaskFactory(
				managementClient,
				amazonProcessor,
				&cfg.Amazon,
				&cfg.Platforms.Temu.Monitor,
				rabbitmqClient,
			)
		},
		SheinFactoryCreator: func(cfg *config.Config) scheduler.TaskFactory {
			return sheinscheduler.NewSheinTaskFactory(
				managementClient,
				amazonProcessor,
				&cfg.Amazon,
				&cfg.Platforms.Shein.Monitor,
				rabbitmqClient,
			)
		},
	}
}
