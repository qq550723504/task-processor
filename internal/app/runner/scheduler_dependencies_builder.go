package runner

import (
	"task-processor/internal/app/scheduler"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/platformbase"
	sheinscheduler "task-processor/internal/shein/scheduler"
	temuscheduler "task-processor/internal/temu/scheduler"
)

func buildSchedulerDependencies(
	schedulerRuntime schedulerFactoryRuntimeProvider,
	cfg *config.Config,
	crawlSource crawlSource,
	rabbitmqClient *rabbitmq.Client,
) SchedulerDependencies {
	_ = cfg
	boundFetcherBuilder := platformbase.BindProductFetcherBuilder(platformbase.NewDefaultProductFetcherBuilder(), crawlSource)
	return SchedulerDependencies{
		TemuFactoryCreator: func(cfg *config.Config) scheduler.TaskFactory {
			return temuscheduler.NewTemuTaskFactoryWithFetcherBuilder(
				schedulerRuntime,
				boundFetcherBuilder,
				&cfg.Amazon,
				&cfg.Platforms.Temu.Monitor,
				rabbitmqClient,
			)
		},
		SheinFactoryCreator: func(cfg *config.Config) scheduler.TaskFactory {
			return sheinscheduler.NewSheinTaskFactoryWithFetcherBuilder(
				schedulerRuntime,
				boundFetcherBuilder,
				&cfg.Amazon,
				&cfg.Platforms.Shein.Monitor,
				rabbitmqClient,
			)
		},
	}
}
