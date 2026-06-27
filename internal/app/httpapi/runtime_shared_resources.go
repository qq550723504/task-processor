package httpapi

import (
	"fmt"

	"github.com/sirupsen/logrus"

	appbootstrap "task-processor/internal/app/bootstrap"
	"task-processor/internal/core/config"
)

func buildHTTPAPISharedResources(cfg *config.Config, logger *logrus.Logger) (*appbootstrap.SharedResources, error) {
	shared, err := appbootstrap.BuildSharedResources(cfg, logger, appbootstrap.SharedResourceOptions{})
	if err != nil {
		return nil, fmt.Errorf("build shared resources: %w", err)
	}
	return shared, nil
}
