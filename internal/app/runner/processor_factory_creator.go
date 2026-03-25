package runner

import (
	"context"

	"task-processor/internal/core/config"
	"task-processor/internal/shein/pipeline"
	"task-processor/internal/temu"

	"github.com/sirupsen/logrus"
)

type TemuProcessorCreator func(ctx context.Context, cfg *config.Config, logger *logrus.Logger, deps temu.Dependencies) (*temu.TemuProcessor, error)
type SheinProcessorCreator func(ctx context.Context, cfg *config.Config, logger *logrus.Logger, deps pipeline.Dependencies) (*pipeline.SheinProcessor, error)

type ProcessorDependencies struct {
	TemuProcessorCreator  TemuProcessorCreator
	SheinProcessorCreator SheinProcessorCreator
}

func (s *processorServiceImpl) resolveTemuProcessorCreator() TemuProcessorCreator {
	return s.temuProcessorCreator
}

func (s *processorServiceImpl) resolveSheinProcessorCreator() SheinProcessorCreator {
	return s.sheinProcessorCreator
}
