package processors

import (
	"context"

	"task-processor/internal/app/runner"
	"task-processor/internal/core/config"
	"task-processor/internal/shein/pipeline"
	"task-processor/internal/temu"

	"github.com/sirupsen/logrus"
)

type processorModule struct {
	name string

	assignRunner func(runner.ProcessorDependencies, runner.TemuProcessorCreator, runner.SheinProcessorCreator) runner.ProcessorDependencies

	temuCreator  runner.TemuProcessorCreator
	sheinCreator runner.SheinProcessorCreator
}

func BuildRunnerProcessorDependencies() runner.ProcessorDependencies {
	deps := runner.ProcessorDependencies{}
	for _, module := range platformProcessorModules() {
		deps = module.assignRunner(deps, module.temuCreator, module.sheinCreator)
	}
	return deps
}

func CreateTemuProcessor(ctx context.Context, cfg *config.Config, logger *logrus.Logger, deps temu.Dependencies) (*temu.TemuProcessor, error) {
	return temu.NewTemuProcessor(ctx, cfg, logger, deps)
}

func CreateSheinProcessor(ctx context.Context, cfg *config.Config, logger *logrus.Logger, deps pipeline.Dependencies) (*pipeline.SheinProcessor, error) {
	return pipeline.NewSheinProcessor(ctx, cfg, logger, deps)
}

func platformProcessorModules() []processorModule {
	return []processorModule{
		{
			name:        "temu",
			temuCreator: CreateTemuProcessor,
			assignRunner: func(deps runner.ProcessorDependencies, temuCreator runner.TemuProcessorCreator, _ runner.SheinProcessorCreator) runner.ProcessorDependencies {
				deps.TemuProcessorCreator = temuCreator
				return deps
			},
		},
		{
			name:         "shein",
			sheinCreator: CreateSheinProcessor,
			assignRunner: func(deps runner.ProcessorDependencies, _ runner.TemuProcessorCreator, sheinCreator runner.SheinProcessorCreator) runner.ProcessorDependencies {
				deps.SheinProcessorCreator = sheinCreator
				return deps
			},
		},
	}
}
