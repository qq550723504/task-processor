package httpapi

import (
	"github.com/sirupsen/logrus"

	bootstrapresources "task-processor/internal/app/bootstrap/resources"
	"task-processor/internal/core/config"
	"task-processor/internal/prompt"
)

func newDBTenantPromptStore(cfg *config.DatabaseConfig, logger *logrus.Logger) (prompt.TenantPromptStore, func() error, error) {
	return bootstrapresources.NewDBTenantPromptStore(cfg, logger)
}
