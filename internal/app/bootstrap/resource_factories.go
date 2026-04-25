package bootstrap

import (
	"context"

	bootstrapresources "task-processor/internal/app/bootstrap/resources"
	"task-processor/internal/core/config"

	"github.com/sirupsen/logrus"
)

type SharedResourceOptions = bootstrapresources.SharedResourceOptions
type SharedResources = bootstrapresources.SharedResources

func BuildSharedResources(cfg *config.Config, logger *logrus.Logger, options SharedResourceOptions) (*SharedResources, error) {
	return bootstrapresources.BuildSharedResources(cfg, logger, options)
}

func InitializePrompts(ctx context.Context, cfg *config.Config, logger *logrus.Logger) error {
	return bootstrapresources.InitializePrompts(ctx, cfg, logger)
}
