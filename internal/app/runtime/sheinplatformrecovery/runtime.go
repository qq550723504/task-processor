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
	Config             string
	StoreID            int64
	ExpectedCount      int
	Execute            bool
	ConfirmFingerprint string
}

func (o Options) ConfigPath() string {
	if path := strings.TrimSpace(o.Config); path != "" {
		return path
	}
	return "config/config-dev.yaml"
}

type runtimeDependencies struct {
	LoadConfig     func(string) (*config.Config, error)
	OpenDB         func(*config.DatabaseConfig) (*gorm.DB, error)
	OpenWritableDB func(*config.DatabaseConfig) (*gorm.DB, error)
	CloseDB        func(*gorm.DB) error
	Recover        func(context.Context, *gorm.DB, listingadmin.PlatformRecoveryRequest) (listingadmin.PlatformRecoveryReport, error)
	Output         io.Writer
}

func defaultRuntimeDependencies() runtimeDependencies {
	return runtimeDependencies{
		LoadConfig:     config.LoadConfigFromFileWithoutValidation,
		OpenDB:         database.NewDatabaseFromConfigWithoutCreate,
		OpenWritableDB: database.NewDatabaseFromConfigWithoutCreateWritable,
		CloseDB:        closeDB,
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
	if opts.Execute && strings.TrimSpace(opts.ConfirmFingerprint) == "" {
		return errors.New("platform recovery execute requires -confirm-fingerprint from a dry run")
	}

	defaults := defaultRuntimeDependencies()
	customOpenDB := deps.OpenDB != nil
	if deps.LoadConfig == nil {
		deps.LoadConfig = defaults.LoadConfig
	}
	if deps.OpenDB == nil {
		deps.OpenDB = defaults.OpenDB
	}
	if deps.OpenWritableDB == nil {
		if customOpenDB {
			deps.OpenWritableDB = deps.OpenDB
		} else {
			deps.OpenWritableDB = defaults.OpenWritableDB
		}
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
	openDB := deps.OpenDB
	if opts.Execute {
		openDB = deps.OpenWritableDB
	}
	db, err := openDB(cfg.Database)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	if db == nil {
		return errors.New("database is not configured")
	}
	defer func() { _ = deps.CloseDB(db) }()

	report, err := deps.Recover(ctx, db, listingadmin.PlatformRecoveryRequest{
		StoreID:            opts.StoreID,
		ExpectedCount:      opts.ExpectedCount,
		Execute:            opts.Execute,
		ConfirmFingerprint: opts.ConfirmFingerprint,
	})
	if err != nil {
		return fmt.Errorf("recover import task platforms: %w", err)
	}
	if _, err := fmt.Fprintln(deps.Output, report.String()); err != nil {
		return fmt.Errorf("write recovery report: %w", err)
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
