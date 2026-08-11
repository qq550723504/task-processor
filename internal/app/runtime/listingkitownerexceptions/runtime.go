package listingkitownerexceptions

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/database"
	"task-processor/internal/listingkit/ownerreconcile"
)

const approvedExceptionReason = "approved current orphaned owner"

type runtimeDependencies struct {
	LoadConfig          func(string) (*config.Config, error)
	OpenDB              func(*config.DatabaseConfig) (*sql.DB, error)
	OpenMetadataDB      func(*config.DatabaseConfig) (*sql.DB, error)
	CloseDB             func(*sql.DB) error
	MetadataTableExists func(context.Context, *sql.DB) (bool, error)
	RunReconciliation   func(context.Context, *sql.DB, *sql.DB) (ownerreconcile.Report, error)
	InsertExceptions    func(context.Context, *sql.DB, ownerreconcile.Report, string) (int, error)
	Output              io.Writer
}

func openSQL(databaseConfig *config.DatabaseConfig) (*sql.DB, error) {
	gormDB, err := database.NewDatabaseFromConfigWithoutCreate(databaseConfig)
	if err != nil || gormDB == nil {
		return nil, err
	}
	return gormDB.DB()
}

func defaultRuntimeDependencies() runtimeDependencies {
	return runtimeDependencies{
		LoadConfig:     config.LoadConfigFromFileWithoutValidation,
		OpenDB:         openSQL,
		OpenMetadataDB: openSQL,
		CloseDB: func(db *sql.DB) error {
			if db == nil {
				return nil
			}
			return db.Close()
		},
		MetadataTableExists: func(ctx context.Context, db *sql.DB) (bool, error) {
			if db == nil {
				return false, nil
			}
			var name sql.NullString
			if err := db.QueryRowContext(ctx, "SELECT to_regclass($1)", "projections.org_metadata2").Scan(&name); err != nil {
				return false, err
			}
			return name.Valid && strings.TrimSpace(name.String) != "", nil
		},
		RunReconciliation: func(ctx context.Context, ownerDB, metadataDB *sql.DB) (ownerreconcile.Report, error) {
			identities, err := ownerreconcile.LoadLegacyIdentities(ctx, metadataDB)
			if err != nil {
				return ownerreconcile.Report{}, err
			}
			repository := ownerreconcile.Repository{Queryer: ownerDB, Inventory: ownerreconcile.Inventory(), Identities: identities, Beginner: ownerDB}
			return repository.DryRun(ctx, identities)
		},
		InsertExceptions: ownerreconcile.InsertSystemOwnedExceptions,
		Output:           os.Stdout,
	}
}

func loadReport(path string) (ownerreconcile.Report, error) {
	if strings.TrimSpace(path) == "" {
		return ownerreconcile.Report{}, errors.New("owner exception report is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ownerreconcile.Report{}, errors.New("read owner exception report failed")
	}
	var report ownerreconcile.Report
	if err := json.Unmarshal(data, &report); err != nil {
		return ownerreconcile.Report{}, errors.New("parse owner exception report failed")
	}
	return report, nil
}

func Run(ctx context.Context, options Options) error {
	return runWithDependencies(ctx, options, defaultRuntimeDependencies())
}

func runWithDependencies(ctx context.Context, options Options, deps runtimeDependencies) error {
	defaults := defaultRuntimeDependencies()
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
	if deps.MetadataTableExists == nil {
		deps.MetadataTableExists = defaults.MetadataTableExists
	}
	if deps.RunReconciliation == nil {
		deps.RunReconciliation = defaults.RunReconciliation
	}
	if deps.InsertExceptions == nil {
		deps.InsertExceptions = defaults.InsertExceptions
	}
	if deps.Output == nil {
		deps.Output = defaults.Output
	}
	report, err := loadReport(options.Report)
	if err != nil {
		return err
	}
	if err := ownerreconcile.ValidateApprovedExceptionReport(report, options.ConfirmReport); err != nil {
		return err
	}
	cfg, err := deps.LoadConfig(options.ConfigPath())
	if err != nil || cfg == nil || cfg.Database == nil {
		return errors.New("load owner exception database config failed")
	}
	ownerDB, err := deps.OpenDB(cfg.Database)
	if err != nil || ownerDB == nil {
		return errors.New("connect owner exception database failed")
	}
	defer func() { _ = deps.CloseDB(ownerDB) }()
	metadataDB, err := openMetadataDB(ctx, cfg.Database, deps)
	if err != nil {
		return err
	}
	defer func() { _ = deps.CloseDB(metadataDB) }()
	liveReport, err := deps.RunReconciliation(ctx, ownerDB, metadataDB)
	if err != nil {
		return errors.New("run live owner reconciliation failed")
	}
	if err := liveReport.SetFingerprint(); err != nil || liveReport.ReportFingerprint != report.ReportFingerprint {
		return errors.New("live owner reconciliation report changed")
	}
	inserted, err := deps.InsertExceptions(ctx, ownerDB, report, approvedExceptionReason)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(deps.Output, "seeded_groups=%d report=%s rows=%d\n", len(report.Findings), report.ReportFingerprint, inserted); err != nil {
		return errors.New("write owner exception summary failed")
	}
	return nil
}

func openMetadataDB(ctx context.Context, base *config.DatabaseConfig, deps runtimeDependencies) (*sql.DB, error) {
	for _, name := range []string{"zitadel_auth", "zitadel"} {
		candidate := *base
		candidate.Database = name
		db, err := deps.OpenMetadataDB(&candidate)
		if err != nil || db == nil {
			continue
		}
		exists, probeErr := deps.MetadataTableExists(ctx, db)
		if probeErr != nil {
			_ = deps.CloseDB(db)
			return nil, errors.New("probe owner exception metadata failed")
		}
		if exists {
			return db, nil
		}
		_ = deps.CloseDB(db)
	}
	return nil, errors.New("owner exception metadata database is unavailable")
}
