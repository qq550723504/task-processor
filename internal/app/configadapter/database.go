package configadapter

import (
	coreconfig "task-processor/internal/core/config"
	platformdatabase "task-processor/internal/platform/database"
)

// Database translates the application database schema into platform runtime configuration.
func Database(cfg *coreconfig.DatabaseConfig) *platformdatabase.Config {
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
