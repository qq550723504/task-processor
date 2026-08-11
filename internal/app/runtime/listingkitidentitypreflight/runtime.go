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
	"task-processor/internal/listingkit/ownerreconcile"
	"task-processor/internal/listingkit/userdirectory"
	"task-processor/internal/pkg/appenv"
	"task-processor/internal/tenantbridge"

	"gorm.io/gorm"
)

type preflightRunner interface {
	Run(context.Context) error
}

type runtimeDependencies struct {
	LoadConfig              func(string) (*config.Config, error)
	OpenDB                  func(*config.DatabaseConfig) (*gorm.DB, error)
	OpenMetadataDB          func(*config.DatabaseConfig) (*gorm.DB, error)
	CloseDB                 func(*gorm.DB) error
	DatabaseSQL             func(*gorm.DB) (*sql.DB, error)
	MetadataTableExists     func(*gorm.DB) (bool, error)
	NewDirectory            func(userdirectory.ClientConfig) (userdirectory.Directory, error)
	NewOwnerRepository      func(*sql.DB) identitypreflight.OwnerRepository
	RunOwnerReconciliation  func(context.Context, *gorm.DB, *gorm.DB) (ownerreconcile.Report, error)
	NewLegacyTenantResolver func(*gorm.DB) identitypreflight.LegacyTenantOrganizationResolver
	NewPreflight            func(identitypreflight.OwnerRepository, userdirectory.Directory, identitypreflight.LegacyTenantOrganizationResolver, io.Writer) preflightRunner
	Output                  io.Writer
}

func defaultRuntimeDependencies() runtimeDependencies {
	return runtimeDependencies{
		LoadConfig:     config.LoadConfigFromFileWithoutValidation,
		OpenDB:         database.NewDatabaseFromConfigWithoutCreate,
		OpenMetadataDB: database.NewDatabaseFromConfigWithoutCreate,
		CloseDB: func(db *gorm.DB) error {
			sqlDB, err := db.DB()
			if err != nil {
				return err
			}
			return sqlDB.Close()
		},
		DatabaseSQL:            func(db *gorm.DB) (*sql.DB, error) { return db.DB() },
		MetadataTableExists:    legacyTenantMetadataTableExists,
		NewDirectory:           userdirectory.NewClient,
		NewOwnerRepository:     identitypreflight.NewPostgresOwnerRepository,
		RunOwnerReconciliation: runOwnerReconciliation,
		NewLegacyTenantResolver: func(db *gorm.DB) identitypreflight.LegacyTenantOrganizationResolver {
			return tenantbridge.NewMetadataResolver(db)
		},
		NewPreflight: func(owners identitypreflight.OwnerRepository, directory userdirectory.Directory, resolver identitypreflight.LegacyTenantOrganizationResolver, output io.Writer) preflightRunner {
			return identitypreflight.NewService(owners, directory, resolver, output)
		},
		Output: os.Stdout,
	}
}

func Run(ctx context.Context, opts Options) error {
	return runWithDependencies(ctx, opts, defaultRuntimeDependencies())
}

func runWithDependencies(ctx context.Context, opts Options, deps runtimeDependencies) error {
	defaults := defaultRuntimeDependencies()
	customOpenDB := deps.OpenDB != nil
	if deps.LoadConfig == nil {
		deps.LoadConfig = defaults.LoadConfig
	}
	if deps.OpenDB == nil {
		deps.OpenDB = defaults.OpenDB
	}
	if deps.OpenMetadataDB == nil {
		deps.OpenMetadataDB = defaults.OpenMetadataDB
	}
	if deps.CloseDB == nil {
		deps.CloseDB = defaults.CloseDB
	}
	if deps.DatabaseSQL == nil {
		deps.DatabaseSQL = defaults.DatabaseSQL
	}
	if deps.MetadataTableExists == nil {
		deps.MetadataTableExists = defaults.MetadataTableExists
	}
	if deps.NewDirectory == nil {
		deps.NewDirectory = defaults.NewDirectory
	}
	if deps.NewOwnerRepository == nil {
		deps.NewOwnerRepository = defaults.NewOwnerRepository
	}
	if deps.RunOwnerReconciliation == nil && !customOpenDB {
		deps.RunOwnerReconciliation = defaults.RunOwnerReconciliation
	}
	if deps.NewLegacyTenantResolver == nil {
		deps.NewLegacyTenantResolver = defaults.NewLegacyTenantResolver
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
	metadataDB, err := openLegacyTenantMetadataDatabase(cfg.Database, deps)
	if err != nil || metadataDB == nil {
		return errors.New("configure legacy tenant metadata database failed")
	}
	defer func() {
		if err := deps.CloseDB(metadataDB); err != nil {
			logger.WithError(err).Warn("close legacy tenant metadata database failed")
		}
	}()
	if deps.RunOwnerReconciliation != nil {
		report, reconcileErr := deps.RunOwnerReconciliation(ctx, db, metadataDB)
		if reconcileErr != nil {
			return errors.New("owner reconciliation preflight failed")
		}
		if report.Summary.UnresolvedRows > 0 || report.Summary.AutoRows > 0 {
			if err := report.SetFingerprint(); err != nil {
				return errors.New("owner reconciliation preflight failed")
			}
			if _, err := fmt.Fprintf(deps.Output, "status=blocked owner_reconciliation=unresolved rows=%d auto_rows=%d system_owned_rows=%d report=%s\n", report.Summary.UnresolvedRows, report.Summary.AutoRows, report.Summary.SystemOwnedRows, report.ReportFingerprint); err != nil {
				return errors.New("write owner reconciliation summary failed")
			}
			return errors.New("owner reconciliation preflight blocked")
		}
	}
	directory, err := deps.NewDirectory(userdirectory.ClientConfig{
		IssuerURL: zitadel.IssuerURL,
		Token:     zitadel.TenantDirectoryToken,
	})
	if err != nil || directory == nil {
		return errors.New("configure ZITADEL user directory failed")
	}
	legacyTenantResolver := deps.NewLegacyTenantResolver(metadataDB)
	if legacyTenantResolver == nil {
		return errors.New("configure legacy tenant resolver failed")
	}
	preflight := deps.NewPreflight(deps.NewOwnerRepository(sqlDB), directory, legacyTenantResolver, deps.Output)
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

func runOwnerReconciliation(ctx context.Context, ownerDB, metadataDB *gorm.DB) (ownerreconcile.Report, error) {
	ownerSQL, err := ownerDB.DB()
	if err != nil {
		return ownerreconcile.Report{}, err
	}
	metadataSQL, err := metadataDB.DB()
	if err != nil {
		return ownerreconcile.Report{}, err
	}
	identities, err := ownerreconcile.LoadLegacyIdentities(ctx, metadataSQL)
	if err != nil {
		return ownerreconcile.Report{}, err
	}
	repository := ownerreconcile.Repository{
		Queryer:    ownerSQL,
		Inventory:  ownerreconcile.Inventory(),
		Identities: identities,
		Exceptions: ownerreconcile.NewPostgresExceptionStore(ownerSQL),
		Beginner:   ownerSQL,
	}
	return repository.DryRun(ctx, identities)
}

func openLegacyTenantMetadataDatabase(base *config.DatabaseConfig, deps runtimeDependencies) (*gorm.DB, error) {
	var selected *gorm.DB
	for _, candidate := range legacyTenantMetadataDatabaseConfigs(base) {
		db, err := deps.OpenMetadataDB(&candidate)
		if err != nil || db == nil {
			continue
		}
		exists, probeErr := deps.MetadataTableExists(db)
		if probeErr != nil || !exists {
			_ = deps.CloseDB(db)
			continue
		}
		if selected != nil {
			_ = deps.CloseDB(db)
			_ = deps.CloseDB(selected)
			return nil, errors.New("multiple legacy tenant metadata databases are available")
		}
		selected = db
	}
	if selected == nil {
		return nil, errors.New("legacy tenant metadata database is unavailable")
	}
	return selected, nil
}

func legacyTenantMetadataDatabaseConfigs(base *config.DatabaseConfig) []config.DatabaseConfig {
	if base == nil {
		return nil
	}
	const firstCandidate = "zitadel_auth"
	const secondCandidate = "zitadel"
	result := make([]config.DatabaseConfig, 0, 2)
	for _, databaseName := range [...]string{firstCandidate, secondCandidate} {
		candidate := *base
		candidate.Database = databaseName
		result = append(result, candidate)
	}
	return result
}

func legacyTenantMetadataTableExists(db *gorm.DB) (bool, error) {
	if db == nil {
		return false, nil
	}
	for _, table := range []string{"projections.org_metadata2", "projections.user_metadata5"} {
		result := struct {
			Name *string `gorm:"column:name"`
		}{}
		if err := db.Raw("select to_regclass(?) as name", table).Scan(&result).Error; err != nil {
			return false, err
		}
		if result.Name == nil || strings.TrimSpace(*result.Name) == "" {
			return false, nil
		}
	}
	return true, nil
}

