package bootstrap

import (
	bootstrapresources "task-processor/internal/app/bootstrap/resources"
	"task-processor/internal/core/config"

	"github.com/sirupsen/logrus"
)

type SharedResourceOptions = bootstrapresources.SharedResourceOptions
type SharedResources = bootstrapresources.SharedResources

func BuildSharedResources(cfg *config.Config, logger *logrus.Logger, options SharedResourceOptions) (*SharedResources, error) {
	return bootstrapresources.BuildSharedResources(cfg, logger, options)
}
