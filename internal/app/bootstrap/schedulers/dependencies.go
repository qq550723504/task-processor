package schedulers

import (
	"task-processor/internal/app/consumer"
	"task-processor/internal/app/ports"
	"task-processor/internal/app/runner"
	appscheduler "task-processor/internal/app/scheduler"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/platformbase"
	sheinscheduler "task-processor/internal/shein/scheduler"
	temuscheduler "task-processor/internal/temu/scheduler"
)

type schedulerModule struct {
	name   string
	assign func(runner.SchedulerDependencies, runner.TaskFactoryCreator) runner.SchedulerDependencies
	build  func(consumer.SchedulerFactoryRuntime, *config.Config, platformbase.ProductFetcherBuilder, *rabbitmq.Client) runner.TaskFactoryCreator
}

func BuildDependencies(
	schedulerRuntime consumer.SchedulerFactoryRuntime,
	cfg *config.Config,
	crawlSource ports.CrawlSource,
	rabbitmqClient *rabbitmq.Client,
) runner.SchedulerDependencies {
	boundFetcherBuilder := platformbase.BindProductFetcherBuilder(platformbase.NewDefaultProductFetcherBuilder(), crawlSource)
	deps := runner.SchedulerDependencies{}
	for _, module := range platformSchedulerModules() {
		deps = module.assign(deps, module.build(schedulerRuntime, cfg, boundFetcherBuilder, rabbitmqClient))
	}
	return deps
}

func platformSchedulerModules() []schedulerModule {
	return []schedulerModule{
		{
			name: "temu",
			assign: func(deps runner.SchedulerDependencies, creator runner.TaskFactoryCreator) runner.SchedulerDependencies {
				deps.TemuFactoryCreator = creator
				return deps
			},
			build: func(schedulerRuntime consumer.SchedulerFactoryRuntime, fallbackCfg *config.Config, fetcherBuilder platformbase.ProductFetcherBuilder, rabbitmqClient *rabbitmq.Client) runner.TaskFactoryCreator {
				return func(currentCfg *config.Config) appscheduler.TaskFactory {
					effectiveCfg := currentCfg
					if effectiveCfg == nil {
						effectiveCfg = fallbackCfg
					}
					return temuscheduler.NewTemuTaskFactoryWithFetcherBuilder(
						schedulerRuntime,
						fetcherBuilder,
						&effectiveCfg.Amazon,
						&effectiveCfg.Platforms.Temu.Monitor,
						rabbitmqClient,
					)
				}
			},
		},
		{
			name: "shein",
			assign: func(deps runner.SchedulerDependencies, creator runner.TaskFactoryCreator) runner.SchedulerDependencies {
				deps.SheinFactoryCreator = creator
				return deps
			},
			build: func(schedulerRuntime consumer.SchedulerFactoryRuntime, fallbackCfg *config.Config, fetcherBuilder platformbase.ProductFetcherBuilder, rabbitmqClient *rabbitmq.Client) runner.TaskFactoryCreator {
				return func(currentCfg *config.Config) appscheduler.TaskFactory {
					effectiveCfg := currentCfg
					if effectiveCfg == nil {
						effectiveCfg = fallbackCfg
					}
					return sheinscheduler.NewSheinTaskFactoryWithFetcherBuilder(
						schedulerRuntime,
						fetcherBuilder,
						&effectiveCfg.Amazon,
						&effectiveCfg.Platforms.Shein.Monitor,
						rabbitmqClient,
					)
				}
			},
		},
	}
}
