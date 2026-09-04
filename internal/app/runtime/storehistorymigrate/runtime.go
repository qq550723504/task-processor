package storehistorymigrate

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"task-processor/internal/app/configadapter"
	"task-processor/internal/core/config"
	"task-processor/internal/pkg/appenv"
	platformdatabase "task-processor/internal/platform/database"
	"task-processor/internal/storecenter"

	"gorm.io/gorm"
)

const (
	defaultConfigPath     = "config/config-dev.yaml"
	defaultAction         = "verify"
	defaultBatchSize      = 100
	migrationActorSubject = "store-service-history-migration"
)

type Options struct {
	Config    string
	LogLevel  string
	Action    string
	Manifest  string
	BatchSize int
	Version   string
	BuildTime string
}

func (options Options) ConfigPath() string {
	if options.Config != "" {
		return options.Config
	}
	return defaultConfigPath
}

type historyMigrator interface {
	BackfillBatch(context.Context, int) (storecenter.StoreHistoryMigrationReport, error)
	Verify(context.Context) (storecenter.StoreHistoryMigrationReport, error)
}

type runtimeDependencies struct {
	LoadConfig   func(string) (*config.Config, error)
	OpenDB       func(*config.DatabaseConfig) (*gorm.DB, error)
	CloseDB      func(*gorm.DB) error
	LoadManifest func(string) (storecenter.NoAuthoritativeHistorySourceManifest, error)
	NewMigrator  func(*gorm.DB, storecenter.NoAuthoritativeHistorySourceManifest, string, func() time.Time) (historyMigrator, error)
	Now          func() time.Time
}

func defaultRuntimeDependencies() runtimeDependencies {
	return runtimeDependencies{
		LoadConfig: config.LoadConfigFromFileWithoutValidation,
		OpenDB: func(databaseConfig *config.DatabaseConfig) (*gorm.DB, error) {
			return platformdatabase.Open(configadapter.Database(databaseConfig))
		},
		CloseDB: func(db *gorm.DB) error {
			sqlDB, err := db.DB()
			if err != nil {
				return err
			}
			return sqlDB.Close()
		},
		LoadManifest: LoadManifest,
		NewMigrator: func(db *gorm.DB, manifest storecenter.NoAuthoritativeHistorySourceManifest, actor string, now func() time.Time) (historyMigrator, error) {
			return storecenter.NewGormStoreHistoryMigrator(db, manifest, actor, now)
		},
		Now: time.Now,
	}
}

func Run(ctx context.Context, options Options, output io.Writer) error {
	return runWithDependencies(ctx, options, output, defaultRuntimeDependencies())
}

func runWithDependencies(ctx context.Context, options Options, output io.Writer, dependencies runtimeDependencies) error {
	action := strings.TrimSpace(options.Action)
	if action == "" {
		action = defaultAction
	}
	if action != "verify" && action != "backfill" {
		return flag.ErrHelp
	}
	if strings.TrimSpace(options.Manifest) == "" {
		return errors.New("approved no-authoritative-history-source manifest path is required")
	}
	if action == "backfill" && (options.BatchSize <= 0 || options.BatchSize > 1000) {
		return errors.New("backfill batch size must be between 1 and 1000")
	}
	if output == nil {
		return errors.New("migration report output is required")
	}

	defaults := defaultRuntimeDependencies()
	if dependencies.LoadConfig == nil {
		dependencies.LoadConfig = defaults.LoadConfig
	}
	if dependencies.OpenDB == nil {
		dependencies.OpenDB = defaults.OpenDB
	}
	if dependencies.CloseDB == nil {
		dependencies.CloseDB = defaults.CloseDB
	}
	if dependencies.LoadManifest == nil {
		dependencies.LoadManifest = defaults.LoadManifest
	}
	if dependencies.NewMigrator == nil {
		dependencies.NewMigrator = defaults.NewMigrator
	}
	if dependencies.Now == nil {
		dependencies.Now = defaults.Now
	}

	logger := appenv.SetupLoggerWithLevel(options.LogLevel)
	appenv.PrintVersionInfo(logger, appenv.VersionInfo{Version: options.Version, BuildTime: options.BuildTime})

	manifest, err := dependencies.LoadManifest(options.Manifest)
	if err != nil {
		return fmt.Errorf("load Store history manifest: %w", err)
	}
	cfg, err := dependencies.LoadConfig(options.ConfigPath())
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if cfg == nil || cfg.Database == nil {
		return errors.New("database config is required")
	}
	db, err := dependencies.OpenDB(cfg.Database)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	if db == nil {
		return errors.New("database config is required")
	}
	defer func() {
		if closeErr := dependencies.CloseDB(db); closeErr != nil {
			logger.WithError(closeErr).Warn("close database failed")
		}
	}()

	migrator, err := dependencies.NewMigrator(db, manifest, migrationActorSubject, dependencies.Now)
	if err != nil {
		return fmt.Errorf("construct Store history migrator: %w", err)
	}
	var report storecenter.StoreHistoryMigrationReport
	switch action {
	case "verify":
		report, err = migrator.Verify(ctx)
	case "backfill":
		report, err = migrator.BackfillBatch(ctx, options.BatchSize)
	}
	if encodeErr := json.NewEncoder(output).Encode(report); encodeErr != nil {
		return fmt.Errorf("write Store history migration report: %w", encodeErr)
	}
	if err != nil {
		return fmt.Errorf("Store history %s failed: %w", action, err)
	}
	return nil
}

func LoadManifest(path string) (storecenter.NoAuthoritativeHistorySourceManifest, error) {
	file, err := os.Open(path)
	if err != nil {
		return storecenter.NoAuthoritativeHistorySourceManifest{}, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var manifest storecenter.NoAuthoritativeHistorySourceManifest
	if err := decoder.Decode(&manifest); err != nil {
		return storecenter.NoAuthoritativeHistorySourceManifest{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return storecenter.NoAuthoritativeHistorySourceManifest{}, errors.New("manifest contains trailing JSON")
		}
		return storecenter.NoAuthoritativeHistorySourceManifest{}, err
	}
	if _, err := storecenter.NewNoAuthoritativeHistorySourceResolver(manifest); err != nil {
		return storecenter.NoAuthoritativeHistorySourceManifest{}, err
	}
	return manifest, nil
}
