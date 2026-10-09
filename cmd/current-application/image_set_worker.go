package main

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"task-processor/internal/app/runtime/currentapplication"
	"task-processor/internal/core/config"
	"task-processor/internal/platform/database"
)

// The existing private worker YAML supplies storage credentials and the
// bounded worker role. The HTTP pool cannot be promoted into a worker pool.
func openImageSetWorker(ctx context.Context, path string, expected currentapplication.DatabaseConfig) (*config.Config, *gorm.DB, error) {
	cfg, err := config.LoadConfigFromFile(path)
	if err != nil {
		return nil, nil, errors.New("load private image worker configuration failed")
	}
	if err = validateImageSetWorkerDatabase(cfg, expected); err != nil {
		return nil, nil, err
	}
	db := cfg.Database
	pool, err := database.OpenExistingWritableContext(ctx, &database.Config{Host: db.Host, Port: db.Port, Database: db.Database, User: db.User, Password: db.Password, MaxConnections: db.MaxConnections, MaxIdleConnections: db.MaxIdleConnections, ConnectionMaxLifetime: db.ConnectionMaxLifetime})
	if err != nil {
		return nil, nil, errors.New("open image worker role failed")
	}
	return cfg, pool, nil
}

func validateImageSetWorkerDatabase(cfg *config.Config, expected currentapplication.DatabaseConfig) error {
	if cfg == nil || cfg.Database == nil {
		return errors.New("image worker database is required")
	}
	db := cfg.Database
	if db.Host != expected.Host || db.Port != expected.Port || db.Database != expected.Database || db.User != "image_agent_worker_runtime" || db.MaxConnections < 1 || db.MaxConnections > 8 {
		return errors.New("image worker requires the same Image owner and a bounded worker role")
	}
	return nil
}
