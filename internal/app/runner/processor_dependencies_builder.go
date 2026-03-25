package runner

import (
	"context"

	"task-processor/internal/core/config"
	"task-processor/internal/shein/pipeline"
	"task-processor/internal/temu"

	"github.com/sirupsen/logrus"
)

func buildProcessorDependencies() ProcessorDependencies {
	return ProcessorDependencies{
		TemuProcessorCreator: func(ctx context.Context, cfg *config.Config, logger *logrus.Logger, deps temu.Dependencies) (*temu.TemuProcessor, error) {
			return temu.NewTemuProcessor(ctx, cfg, logger, deps)
		},
		SheinProcessorCreator: func(ctx context.Context, cfg *config.Config, logger *logrus.Logger, deps pipeline.Dependencies) (*pipeline.SheinProcessor, error) {
			return pipeline.NewSheinProcessor(ctx, cfg, logger, deps)
		},
	}
}
