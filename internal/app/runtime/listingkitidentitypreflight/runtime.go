package listingkitidentitypreflight

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/database"
	"task-processor/internal/listingkit/identitypreflight"
	"task-processor/internal/listingkit/userdirectory"
	"task-processor/internal/pkg/appenv"

	"gorm.io/gorm"
)

type preflightRunner interface {
	Run(context.Context) error
}

type runtimeDependencies struct {
	LoadConfig         func(string) (*config.Config, error)
	OpenDB             func(*config.DatabaseConfig) (*gorm.DB, error)
	CloseDB            func(*gorm.DB) error
	DatabaseSQL        func(*gorm.DB) (*sql.DB, error)
	NewDirectory       func(userdirectory.ClientConfig) (userdirectory.Directory, error)
	NewOwnerRepository func(*sql.DB) identitypreflight.OwnerRepository
	NewPreflight       func(identitypreflight.OwnerRepository, userdirectory.Directory, io.Writer) preflightRunner
	Output             io.Writer
}

func defaultRuntimeDependencies() runtimeDependencies {
	return runtimeDependencies{
		LoadConfig: config.LoadConfigFromFile,
		OpenDB:     database.NewDatabaseFromConfig,
		CloseDB: func(db *gorm.DB) error {
			sqlDB, err := db.DB()
			if err != nil {
				return err
			}
			return sqlDB.Close()
		},
		DatabaseSQL:        func(db *gorm.DB) (*sql.DB, error) { return db.DB() },
		NewDirectory:       userdirectory.NewClient,
		NewOwnerRepository: identitypreflight.NewPostgresOwnerRepository,
		NewPreflight: func(owners identitypreflight.OwnerRepository, directory userdirectory.Directory, output io.Writer) preflightRunner {
			return identitypreflight.NewService(owners, directory, output)
		},
		Output: os.Stdout,
	}
}

func Run(ctx context.Context, opts Options) error {
	return runWithDependencies(ctx, opts, defaultRuntimeDependencies())
}

func runWithDependencies(ctx context.Context, opts Options, deps runtimeDependencies) error {
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
	if deps.DatabaseSQL == nil {
		deps.DatabaseSQL = defaults.DatabaseSQL
	}
	if deps.NewDirectory == nil {
		deps.NewDirectory = defaults.NewDirectory
	}
	if deps.NewOwnerRepository == nil {
		deps.NewOwnerRepository = defaults.NewOwnerRepository
	}
	if deps.NewPreflight == nil {
		deps.NewPreflight = defaults.NewPreflight
	}
	if deps.Output == nil {
		deps.Output = defaults.Output
	}

	logger := appenv.SetupLoggerWithLevel(opts.LogLevel)
	appenv.PrintVersionInfo(logger, appenv.VersionInfo{Version: opts.Version, BuildTime: opts.BuildTime})

	cfg, err := deps.LoadConfig(opts.ConfigPath())
	if err != nil {
		return errors.New("load config failed")
	}
	if cfg == nil || cfg.Database == nil {
		return errors.New("database is required for identity preflight")
	}
	zitadel := cfg.ListingKit.Zitadel
	if strings.TrimSpace(zitadel.IssuerURL) == "" {
		return errors.New("ZITADEL issuer is required for the identity directory")
	}
	if strings.TrimSpace(zitadel.TenantDirectoryToken) == "" {
		return errors.New("ZITADEL directory token is required for the identity directory")
	}

	directory, err := deps.NewDirectory(userdirectory.ClientConfig{
		IssuerURL: zitadel.IssuerURL,
		Token:     zitadel.TenantDirectoryToken,
	})
	if err != nil || directory == nil {
		return errors.New("configure ZITADEL user directory failed")
	}
	db, err := deps.OpenDB(cfg.Database)
	if err != nil || db == nil {
		return errors.New("connect database failed")
	}
	defer func() {
		if err := deps.CloseDB(db); err != nil {
			logger.WithError(err).Warn("close database failed")
		}
	}()

	sqlDB, err := deps.DatabaseSQL(db)
	if err != nil || sqlDB == nil {
		return errors.New("access database failed")
	}
	preflight := deps.NewPreflight(deps.NewOwnerRepository(sqlDB), directory, deps.Output)
	if preflight == nil {
		return errors.New("configure identity preflight failed")
	}
	if err := preflight.Run(ctx); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(deps.Output, "status=ok identity_preflight=passed"); err != nil {
		return errors.New("write identity preflight summary failed")
	}
	return nil
}
