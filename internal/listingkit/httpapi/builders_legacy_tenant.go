package httpapi

import (
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"task-processor/internal/core/config"
	tenantbridgebootstrap "task-processor/internal/tenantbridge/bootstrap"
)

func ConfigureLegacyTenantResolver(cfg *config.Config, logger *logrus.Logger) (func() error, error) {
	return tenantbridgebootstrap.ConfigureFromConfig(cfg, logger)
}

func shouldDisableLegacyTenantResolver(cfg *config.Config) bool {
	return tenantbridgebootstrap.ShouldDisableLegacyTenantResolver(cfg)
}

func legacyTenantResolverDatabaseConfigs(cfg *config.Config) []config.DatabaseConfig {
	return tenantbridgebootstrap.LegacyTenantResolverDatabaseConfigs(cfg)
}

func legacyTenantMetadataTableExists(db *gorm.DB) bool {
	return tenantbridgebootstrap.LegacyTenantMetadataTableExists(db)
}
