package bootstrap

import (
	"context"

	"task-processor/internal/app/consumer"
	"task-processor/internal/app/runner"
	"task-processor/internal/core/config"
	"task-processor/internal/shein/pipeline"
	"task-processor/internal/temu"

	"github.com/sirupsen/logrus"
)

func createTemuProcessor(ctx context.Context, cfg *config.Config, logger *logrus.Logger, deps temu.Dependencies) (*temu.TemuProcessor, error) {
	return temu.NewTemuProcessor(ctx, cfg, logger, deps)
}

func createSheinProcessor(ctx context.Context, cfg *config.Config, logger *logrus.Logger, deps pipeline.Dependencies) (*pipeline.SheinProcessor, error) {
	return pipeline.NewSheinProcessor(ctx, cfg, logger, deps)
}

func BuildConsumerProcessorCreators() consumer.ProcessorCreators {
	return consumer.ProcessorCreators{
		TemuProcessorCreator:  createTemuProcessor,
		SheinProcessorCreator: createSheinProcessor,
	}
}

// BuildProcessorDependencies keeps runner-side processor wiring in bootstrap.
func BuildProcessorDependencies() runner.ProcessorDependencies {
	return runner.ProcessorDependencies{
		TemuProcessorCreator:  createTemuProcessor,
		SheinProcessorCreator: createSheinProcessor,
	}
}
