package sheinplatformrecovery

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"gorm.io/gorm"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/database"
	"task-processor/internal/listingadmin"
)

const recoveryStoreID int64 = 986

type Options struct {
	Config        string
	StoreID       int64
	ExpectedCount int
	Execute       bool
}

func (o Options) ConfigPath() string {
	if path := strings.TrimSpace(o.Config); path != "" {
		return path
	}
	return "config/config-dev.yaml"
}

type runtimeDependencies struct {
	LoadConfig func(string) (*config.Config, error)
	OpenDB     func(*config.DatabaseConfig) (*gorm.DB, error)
	CloseDB    func(*gorm.DB) error
	Recover    func(context.Context, *gorm.DB, listingadmin.PlatformRecoveryRequest) (listingadmin.PlatformRecoveryReport, error)
	Output     io.Writer
}

func defaultRuntimeDependencies() runtimeDependencies {
	return runtimeDependencies{
		LoadConfig: config.LoadConfigFromFileWithoutValidation,
		OpenDB:     database.NewDatabaseFromConfigWithoutCreate,
		CloseDB:    closeDB,
		Recover: func(ctx context.Context, db *gorm.DB, req listingadmin.PlatformRecoveryRequest) (listingadmin.PlatformRecoveryReport, error) {
			return listingadmin.NewGormImportTaskRepository(db).RecoverStore986PlatformCohort(ctx, req)
		},
		Output: os.Stdout,
	}
}

func Run(ctx context.Context, opts Options) error {
	return runWithDependencies(ctx, opts, defaultRuntimeDependencies())
}

func runWithDependencies(ctx context.Context, opts Options, deps runtimeDependencies) error {
	if opts.StoreID != recoveryStoreID {
		return fmt.Errorf("platform recovery is restricted to store_id %d", recoveryStoreID)
	}
	if opts.ExpectedCount <= 0 {
		return errors.New("platform recovery expected-count must be positive")
	}

	defaults := defaultRuntimeDependencies()
	if deps.LoadConfig == nil {
		deps.LoadConfig = defaults.LoadConfig
	}
	if deps.OpenDB == nil {
		deps.OpenDB = defaults.OpenDB
	}
	if deps.CloseDB == nil {
		deps.CloseDB = defaults.CloseDB
	}
	if deps.Recover == nil {
		deps.Recover = defaults.Recover
	}
	if deps.Output == nil {
		deps.Output = defaults.Output
	}

	cfg, err := deps.LoadConfig(opts.ConfigPath())
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if cfg == nil || cfg.Database == nil {
		return errors.New("database is not configured")
	}
	db, err := deps.OpenDB(cfg.Database)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	if db == nil {
		return errors.New("database is not configured")
	}
	defer func() { _ = deps.CloseDB(db) }()

	report, err := deps.Recover(ctx, db, listingadmin.PlatformRecoveryRequest{
		StoreID:       opts.StoreID,
		ExpectedCount: opts.ExpectedCount,
		Execute:       opts.Execute,
	})
	if err != nil {
		return fmt.Errorf("recover import task platforms: %w", err)
	}
	_, _ = fmt.Fprintln(deps.Output, report.String())
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
