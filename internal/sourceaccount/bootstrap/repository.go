package bootstrap

import (
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"task-processor/internal/core/config"
	platformdatabase "task-processor/internal/platform/database"
	"task-processor/internal/sourceaccount"
)

// BuildRepository creates the source-account repository used by marketplace
// runtimes without coupling them to ListingKit HTTPAPI assembly.
func BuildRepository(cfg *config.Config, logger *logrus.Logger) (sourceaccount.Repository, []func() error, error) {
	if cfg == nil || cfg.Database == nil || strings.TrimSpace(cfg.Database.Host) == "" {
		return nil, nil, nil
	}

	db, closer, err := openRepositoryDatabase(cfg.Database, logger)
	if err != nil {
		return nil, nil, err
	}
	if err := autoMigrateRepository(db); err != nil {
		_ = closer()
		return nil, nil, fmt.Errorf("source account schema bootstrap failed: %w", err)
	}
	return sourceaccount.NewGormRepository(db), []func() error{closer}, nil
}

func openRepositoryDatabase(cfg *config.DatabaseConfig, logger *logrus.Logger) (*gorm.DB, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	databaseConfig := platformDatabaseConfig(cfg)
	db, err := platformdatabase.OpenShared(databaseConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	if logger != nil {
		logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	}
	return db, func() error { return platformdatabase.CloseShared(databaseConfig, db) }, nil
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

func autoMigrateRepository(db *gorm.DB) error {
	if !shouldAutoMigrate() {
		return nil
	}
	return sourceaccount.AutoMigrateRepository(db)
}

func shouldAutoMigrate() bool {
	raw := strings.TrimSpace(os.Getenv("TASK_PROCESSOR_LISTINGKIT_RUNTIME_AUTOMIGRATE"))
	if raw == "" {
		return true
	}
	switch strings.ToLower(raw) {
	case "0", "false", "no", "n", "off", "disabled":
		return false
	default:
		return true
	}
}
