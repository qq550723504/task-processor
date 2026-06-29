package httpapi

import (
	"fmt"

	"github.com/sirupsen/logrus"

	bootstrapresources "task-processor/internal/app/bootstrap/resources"
	"task-processor/internal/core/config"
	"task-processor/internal/listingadmin"
)

func buildHTTPAPISharedResources(cfg *config.Config, logger *logrus.Logger) (listingadmin.StoreAPI, error) {
	shared, err := bootstrapresources.BuildSharedResources(cfg, logger, bootstrapresources.SharedResourceOptions{})
	if err != nil {
		return nil, fmt.Errorf("build shared resources: %w", err)
	}
	return shared.StoreAPI(), nil
}
