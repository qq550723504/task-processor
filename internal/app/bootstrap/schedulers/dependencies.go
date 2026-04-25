package schedulers

import (
	"task-processor/internal/app/runner"
	appscheduler "task-processor/internal/app/scheduler"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/platformbase"
	sheinscheduler "task-processor/internal/shein/scheduler"
	temuscheduler "task-processor/internal/temu/scheduler"
)

type schedulerModule struct {
	name   string
	assign func(runner.SchedulerDependencies, runner.TaskFactoryCreator) runner.SchedulerDependencies
	build  func(*management.ClientManager, *config.Config, platformbase.ProductFetcherBuilder, *rabbitmq.Client) runner.TaskFactoryCreator
}

func BuildDependencies(
	managementClient *management.ClientManager,
	cfg *config.Config,
	crawlSource runner.CrawlSource,
	rabbitmqClient *rabbitmq.Client,
) runner.SchedulerDependencies {
	boundFetcherBuilder := platformbase.BindProductFetcherBuilder(platformbase.NewDefaultProductFetcherBuilder(), crawlSource)
	deps := runner.SchedulerDependencies{}
	for _, module := range platformSchedulerModules() {
		deps = module.assign(deps, module.build(managementClient, cfg, boundFetcherBuilder, rabbitmqClient))
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
			build: func(managementClient *management.ClientManager, fallbackCfg *config.Config, fetcherBuilder platformbase.ProductFetcherBuilder, rabbitmqClient *rabbitmq.Client) runner.TaskFactoryCreator {
				return func(currentCfg *config.Config) appscheduler.TaskFactory {
					effectiveCfg := currentCfg
					if effectiveCfg == nil {
						effectiveCfg = fallbackCfg
					}
					return temuscheduler.NewTemuTaskFactoryWithFetcherBuilder(
						managementClient,
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
			build: func(managementClient *management.ClientManager, fallbackCfg *config.Config, fetcherBuilder platformbase.ProductFetcherBuilder, rabbitmqClient *rabbitmq.Client) runner.TaskFactoryCreator {
				return func(currentCfg *config.Config) appscheduler.TaskFactory {
					effectiveCfg := currentCfg
					if effectiveCfg == nil {
						effectiveCfg = fallbackCfg
					}
					return sheinscheduler.NewSheinTaskFactoryWithFetcherBuilder(
						managementClient,
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
