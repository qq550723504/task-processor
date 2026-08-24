package bootstrap

import (
	"strings"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/database"
	"task-processor/internal/tenantbridge"
)

// ConfigureFromConfig installs the legacy tenant resolver when the configured
// ZITADEL database exposes the legacy organization metadata projection.
func ConfigureFromConfig(cfg *config.Config, logger *logrus.Logger) (func() error, error) {
	if ShouldDisableLegacyTenantResolver(cfg) {
		tenantbridge.ConfigureLegacyTenantResolver(nil)
		return nil, nil
	}
	for _, zitadelCfg := range LegacyTenantResolverDatabaseConfigs(cfg) {
		db, err := database.NewSharedDatabaseFromConfig(&zitadelCfg)
		if err != nil {
			continue
		}
		if !LegacyTenantMetadataTableExists(db) {
			_ = database.CloseSharedDatabase(&zitadelCfg, db)
			continue
		}
		tenantbridge.ConfigureLegacyTenantResolver(tenantbridge.NewMetadataResolver(db))
		if logger != nil {
			logger.Infof("legacy tenant resolver connected: %s:%d/%s", zitadelCfg.Host, zitadelCfg.Port, zitadelCfg.Database)
		}
		return func() error { return database.CloseSharedDatabase(&zitadelCfg, db) }, nil
	}
	tenantbridge.ConfigureLegacyTenantResolver(nil)
	if logger != nil {
		logger.Warn("legacy tenant resolver metadata table not found; legacy tenant bridge disabled")
	}
	return nil, nil
}

func ShouldDisableLegacyTenantResolver(cfg *config.Config) bool {
	return cfg == nil || cfg.Database == nil || strings.TrimSpace(cfg.Database.Host) == ""
}

func LegacyTenantResolverDatabaseConfigs(cfg *config.Config) []config.DatabaseConfig {
	if ShouldDisableLegacyTenantResolver(cfg) {
		return nil
	}
	candidates := []string{"zitadel_auth", "zitadel"}
	configs := make([]config.DatabaseConfig, 0, len(candidates))
	for _, databaseName := range candidates {
		zitadelCfg := *cfg.Database
		zitadelCfg.Database = databaseName
		configs = append(configs, zitadelCfg)
	}
	return configs
}

func LegacyTenantMetadataTableExists(db *gorm.DB) bool {
	if db == nil {
		return false
	}
	result := struct {
		Name *string `gorm:"column:name"`
	}{}
	if err := db.Raw("select to_regclass(?) as name", "projections.org_metadata2").Scan(&result).Error; err != nil {
		return false
	}
	return result.Name != nil && strings.TrimSpace(*result.Name) != ""
}
