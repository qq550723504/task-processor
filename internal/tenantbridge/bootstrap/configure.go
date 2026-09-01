package bootstrap

import (
	"strings"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"task-processor/internal/core/config"
	platformdatabase "task-processor/internal/platform/database"
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
		databaseConfig := platformDatabaseConfig(&zitadelCfg)
		db, err := platformdatabase.OpenShared(databaseConfig)
		if err != nil {
			continue
		}
		if !LegacyTenantMetadataTableExists(db) {
			_ = platformdatabase.CloseShared(databaseConfig, db)
			continue
		}
		tenantbridge.ConfigureLegacyTenantResolver(tenantbridge.NewMetadataResolver(db))
		if logger != nil {
			logger.Infof("legacy tenant resolver connected: %s:%d/%s", zitadelCfg.Host, zitadelCfg.Port, zitadelCfg.Database)
		}
		return func() error { return platformdatabase.CloseShared(databaseConfig, db) }, nil
	}
	tenantbridge.ConfigureLegacyTenantResolver(nil)
	if logger != nil {
		logger.Warn("legacy tenant resolver metadata table not found; legacy tenant bridge disabled")
	}
	return nil, nil
}

func platformDatabaseConfig(cfg *config.DatabaseConfig) *platformdatabase.Config {
	if cfg == nil {
		return nil
	}
	return &platformdatabase.Config{
		Host:                  cfg.Host,
		Port:                  cfg.Port,
		User:                  cfg.User,
		Password:              cfg.Password,
		Database:              cfg.Database,
		MaxConnections:        cfg.MaxConnections,
		MaxIdleConnections:    cfg.MaxIdleConnections,
		ConnectionMaxLifetime: cfg.ConnectionMaxLifetime,
	}
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
