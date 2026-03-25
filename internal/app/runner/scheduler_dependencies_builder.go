package runner

import (
	"task-processor/internal/app/scheduler"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/infra/rabbitmq"
	sheinscheduler "task-processor/internal/shein/scheduler"
	temuscheduler "task-processor/internal/temu/scheduler"
)

func buildSchedulerDependencies(
	managementClient *management.ClientManager,
	cfg *config.Config,
	amazonProcessor amazonCrawler,
	rabbitmqClient *rabbitmq.Client,
) SchedulerDependencies {
	_ = cfg
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
