package httpapi

import (
	"strings"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/database"
	"task-processor/internal/tenantbridge"
)

func ConfigureLegacyTenantResolver(cfg *config.Config, logger *logrus.Logger) (func() error, error) {
	if shouldDisableLegacyTenantResolver(cfg) {
		tenantbridge.ConfigureLegacyTenantResolver(nil)
		return nil, nil
	}
	for _, zitadelCfg := range legacyTenantResolverDatabaseConfigs(cfg) {
		db, err := database.NewSharedDatabaseFromConfig(&zitadelCfg)
		if err != nil {
			continue
		}
		if !legacyTenantMetadataTableExists(db) {
			_ = database.CloseSharedDatabase(&zitadelCfg, db)
			continue
		}
		tenantbridge.ConfigureLegacyTenantResolver(tenantbridge.NewMetadataResolver(db))
		logger.Infof("listingkit legacy tenant resolver connected: %s:%d/%s", zitadelCfg.Host, zitadelCfg.Port, zitadelCfg.Database)
		return func() error { return database.CloseSharedDatabase(&zitadelCfg, db) }, nil
	}
	tenantbridge.ConfigureLegacyTenantResolver(nil)
	logger.Warn("listingkit legacy tenant resolver metadata table not found; legacy tenant bridge disabled")
	return nil, nil
}

func shouldDisableLegacyTenantResolver(cfg *config.Config) bool {
	return cfg == nil || cfg.Database == nil || strings.TrimSpace(cfg.Database.Host) == ""
}

func legacyTenantResolverDatabaseConfigs(cfg *config.Config) []config.DatabaseConfig {
	if shouldDisableLegacyTenantResolver(cfg) {
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

func legacyTenantMetadataTableExists(db *gorm.DB) bool {
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
