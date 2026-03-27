package bootstrap

import (
	"context"

	"task-processor/internal/app/consumer"
	"task-processor/internal/app/ports"
	"task-processor/internal/app/runner"
	appscheduler "task-processor/internal/app/scheduler"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/shein/pipeline"
	sheinscheduler "task-processor/internal/shein/scheduler"
	"task-processor/internal/temu"
	temuscheduler "task-processor/internal/temu/scheduler"

	"github.com/sirupsen/logrus"
)

type processorModule struct {
	name string

	assignConsumer func(consumer.ProcessorCreators, consumer.TemuProcessorCreator, consumer.SheinProcessorCreator) consumer.ProcessorCreators
	assignRunner   func(runner.ProcessorDependencies, runner.TemuProcessorCreator, runner.SheinProcessorCreator) runner.ProcessorDependencies

	temuCreator  consumer.TemuProcessorCreator
	sheinCreator consumer.SheinProcessorCreator
}

type schedulerModule struct {
	name   string
	assign func(runner.SchedulerDependencies, runner.TaskFactoryCreator) runner.SchedulerDependencies
	build  func(*management.ClientManager, *config.Config, ports.ProductSource, *rabbitmq.Client) runner.TaskFactoryCreator
}

func platformProcessorModules() []processorModule {
	return []processorModule{
		{
			name:        "temu",
			temuCreator: createTemuProcessor,
			assignConsumer: func(creators consumer.ProcessorCreators, temuCreator consumer.TemuProcessorCreator, _ consumer.SheinProcessorCreator) consumer.ProcessorCreators {
				creators.TemuProcessorCreator = temuCreator
				return creators
			},
			assignRunner: func(deps runner.ProcessorDependencies, temuCreator runner.TemuProcessorCreator, _ runner.SheinProcessorCreator) runner.ProcessorDependencies {
				deps.TemuProcessorCreator = temuCreator
				return deps
			},
		},
		{
			name:         "shein",
			sheinCreator: createSheinProcessor,
			assignConsumer: func(creators consumer.ProcessorCreators, _ consumer.TemuProcessorCreator, sheinCreator consumer.SheinProcessorCreator) consumer.ProcessorCreators {
				creators.SheinProcessorCreator = sheinCreator
				return creators
			},
			assignRunner: func(deps runner.ProcessorDependencies, _ runner.TemuProcessorCreator, sheinCreator runner.SheinProcessorCreator) runner.ProcessorDependencies {
				deps.SheinProcessorCreator = sheinCreator
				return deps
			},
		},
	}
}

func platformSchedulerModules() []schedulerModule {
	return []schedulerModule{
		{
			name: "temu",
			assign: func(deps runner.SchedulerDependencies, creator runner.TaskFactoryCreator) runner.SchedulerDependencies {
				deps.TemuFactoryCreator = creator
				return deps
			},
			build: func(managementClient *management.ClientManager, fallbackCfg *config.Config, amazonProcessor ports.ProductSource, rabbitmqClient *rabbitmq.Client) runner.TaskFactoryCreator {
				return func(currentCfg *config.Config) appscheduler.TaskFactory {
					effectiveCfg := currentCfg
					if effectiveCfg == nil {
						effectiveCfg = fallbackCfg
					}
					return temuscheduler.NewTemuTaskFactory(
						managementClient,
						amazonProcessor,
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
			build: func(managementClient *management.ClientManager, fallbackCfg *config.Config, amazonProcessor ports.ProductSource, rabbitmqClient *rabbitmq.Client) runner.TaskFactoryCreator {
				return func(currentCfg *config.Config) appscheduler.TaskFactory {
					effectiveCfg := currentCfg
					if effectiveCfg == nil {
						effectiveCfg = fallbackCfg
					}
					return sheinscheduler.NewSheinTaskFactory(
						managementClient,
						amazonProcessor,
						&effectiveCfg.Amazon,
						&effectiveCfg.Platforms.Shein.Monitor,
						rabbitmqClient,
					)
				}
			},
		},
	}
}

func createTemuProcessor(ctx context.Context, cfg *config.Config, logger *logrus.Logger, deps temu.Dependencies) (*temu.TemuProcessor, error) {
	return temu.NewTemuProcessor(ctx, cfg, logger, deps)
}

func createSheinProcessor(ctx context.Context, cfg *config.Config, logger *logrus.Logger, deps pipeline.Dependencies) (*pipeline.SheinProcessor, error) {
	return pipeline.NewSheinProcessor(ctx, cfg, logger, deps)
}
