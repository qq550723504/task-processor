package productlistingschemamigrate

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"gorm.io/gorm"

	"task-processor/internal/app/configadapter"
	"task-processor/internal/app/runtime/currentapplication"
	"task-processor/internal/app/schema/productlisting"
	"task-processor/internal/core/config"
	imagestore "task-processor/internal/imageagent/store"
	platformdatabase "task-processor/internal/platform/database"
)

type Dependencies struct {
	LoadConfig func(string) (*config.Config, error)
	OpenDB     func(*config.DatabaseConfig) (*gorm.DB, error)
	CloseDB    func(*gorm.DB) error
	MigrateAll func(context.Context, *gorm.DB) error
}

func Run(ctx context.Context, configPath string) error {
	return runWithDependencies(ctx, configPath, Dependencies{})
}

// GrantImageAgentRuntime requires both an explicit owner credential and the
// current application's validated ImageAgent owner identity. It does not use
// the schema-migrator's legacy default config or migrate during this action.
func GrantImageAgentRuntime(ctx context.Context, ownerConfigPath, currentManifestPath string) error {
	if ctx == nil || !filepath.IsAbs(ownerConfigPath) || !filepath.IsAbs(currentManifestPath) || ownerConfigPath == currentManifestPath {
		return errors.New("explicit owner config and current application manifest are required")
	}
	owner, err := config.LoadConfigFromFileWithoutValidation(ownerConfigPath)
	if err != nil {
		return fmt.Errorf("load explicit owner config: %w", err)
	}
	current, err := currentapplication.LoadConfig(currentManifestPath)
	if err != nil {
		return fmt.Errorf("load current application manifest: %w", err)
	}
	if err := validateImageAgentGrantTarget(owner, current); err != nil {
		return err
	}
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	db, err := platformdatabase.OpenExistingWritableContext(bounded, configadapter.Database(owner.Database))
	if err != nil {
		return fmt.Errorf("connect explicit image agent owner database: %w", err)
	}
	defer func() { _ = closeDB(db) }()
	if err := imagestore.GrantOrganizationRuntimePermissions(bounded, db); err != nil {
		return fmt.Errorf("grant image agent runtime permissions: %w", err)
	}
	return nil
}

// GrantImageAgentWorkerRuntime grants only the pre-created organization-v1
// worker role on the exact current ImageAgent owner DB. The API role is not
// widened, and ordinary worker startup performs verification only.
func GrantImageAgentWorkerRuntime(ctx context.Context, ownerConfigPath, currentManifestPath string) error {
	if ctx == nil || !filepath.IsAbs(ownerConfigPath) || !filepath.IsAbs(currentManifestPath) || ownerConfigPath == currentManifestPath {
		return errors.New("explicit owner config and current application manifest are required")
	}
	owner, err := config.LoadConfigFromFileWithoutValidation(ownerConfigPath)
	if err != nil {
		return fmt.Errorf("load explicit image agent worker owner config: %w", err)
	}
	current, err := currentapplication.LoadConfig(currentManifestPath)
	if err != nil {
		return fmt.Errorf("load current application manifest: %w", err)
	}
	if err := validateImageAgentGrantTarget(owner, current); err != nil {
		return err
	}
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	db, err := platformdatabase.OpenExistingWritableContext(bounded, configadapter.Database(owner.Database))
	if err != nil {
		return fmt.Errorf("connect explicit image agent worker owner database: %w", err)
	}
	defer func() { _ = closeDB(db) }()
	if err := imagestore.GrantOrganizationWorkerRuntimePermissions(bounded, db); err != nil {
		return fmt.Errorf("grant image agent worker runtime permissions: %w", err)
	}
	return nil
}

func validateImageAgentGrantTarget(owner *config.Config, current *currentapplication.Config) error {
	if owner == nil || owner.Database == nil || current == nil || current.ImageAgent == nil {
		return errors.New("current image agent owner database is not configured")
	}
	actual, intended := owner.Database, current.ImageAgent.Database
	if actual.Host == "" || actual.Port < 1 || actual.Database == "" || actual.User == "" || actual.User == imagestore.OrganizationRuntimeRole || actual.User == imagestore.OrganizationWorkerRuntimeRole ||
		actual.Host != intended.Host || actual.Port != intended.Port || actual.Database != intended.Database {
		return errors.New("explicit owner database does not match current image agent owner")
	}
	return nil
}

func runWithDependencies(ctx context.Context, configPath string, deps Dependencies) error {
	if deps.LoadConfig == nil {
		deps.LoadConfig = config.LoadConfigFromFileWithoutValidation
	}
	if deps.OpenDB == nil {
		deps.OpenDB = func(cfg *config.DatabaseConfig) (*gorm.DB, error) {
			return platformdatabase.Open(configadapter.Database(cfg))
		}
	}
	if deps.CloseDB == nil {
		deps.CloseDB = closeDB
	}
	if deps.MigrateAll == nil {
		deps.MigrateAll = productlisting.Migrate
	}
	cfg, err := deps.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if cfg == nil || cfg.Database == nil {
		return fmt.Errorf("database is not configured")
	}
	db, err := deps.OpenDB(cfg.Database)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	if db == nil {
		return fmt.Errorf("database is not configured")
	}
	defer func() { _ = deps.CloseDB(db) }()
	if err := deps.MigrateAll(ctx, db); err != nil {
		return fmt.Errorf("migrate product listing API schema: %w", err)
	}
	return nil
}

func closeDB(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
