package consumer

import (
	"context"

	"task-processor/internal/core/config"
	"task-processor/internal/shein/pipeline"
	"task-processor/internal/temu"

	"github.com/sirupsen/logrus"
)

type TemuProcessorCreator func(ctx context.Context, cfg *config.Config, logger *logrus.Logger, deps temu.Dependencies) (*temu.TemuProcessor, error)
type SheinProcessorCreator func(ctx context.Context, cfg *config.Config, logger *logrus.Logger, deps pipeline.Dependencies) (*pipeline.SheinProcessor, error)

type ProcessorCreators struct {
	TemuProcessorCreator  TemuProcessorCreator
	SheinProcessorCreator SheinProcessorCreator
}

func defaultProcessorCreators() ProcessorCreators {
	return ProcessorCreators{
		TemuProcessorCreator: func(ctx context.Context, cfg *config.Config, logger *logrus.Logger, deps temu.Dependencies) (*temu.TemuProcessor, error) {
			return temu.NewTemuProcessor(ctx, cfg, logger, deps)
		},
		SheinProcessorCreator: func(ctx context.Context, cfg *config.Config, logger *logrus.Logger, deps pipeline.Dependencies) (*pipeline.SheinProcessor, error) {
			return pipeline.NewSheinProcessor(ctx, cfg, logger, deps)
		},
	}
}
