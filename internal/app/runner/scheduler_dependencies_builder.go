package runner

import (
	"task-processor/internal/app/ports"
	"task-processor/internal/app/scheduler"
	"task-processor/internal/core/config"
	appfetcher "task-processor/internal/crawler/fetcher"
	"task-processor/internal/platform/queue/rabbitmq"
	sheinscheduler "task-processor/internal/shein/scheduler"
	temuscheduler "task-processor/internal/temu/scheduler"
)

func buildSchedulerDependencies(
	schedulerRuntime schedulerFactoryRuntimeProvider,
	cfg *config.Config,
	crawlSource ports.CrawlSource,
	rabbitmqClient *rabbitmq.Client,
) SchedulerDependencies {
	_ = cfg
	boundFetcherBuilder := appfetcher.NewProductFetcherBuilder(schedulerRuntime.GetRawJsonDataAdapter(), crawlSource)
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
