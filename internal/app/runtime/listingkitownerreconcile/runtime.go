package listingkitownerreconcile

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/database"
	"task-processor/internal/listingkit/ownerreconcile"
)

type runtimeDependencies struct {
	LoadConfig          func(string) (*config.Config, error)
	OpenDB              func(*config.DatabaseConfig) (*sql.DB, error)
	OpenMetadataDB      func(*config.DatabaseConfig) (*sql.DB, error)
	MetadataTableExists func(context.Context, *sql.DB) (bool, error)
	CloseDB             func(*sql.DB) error
	RunReconciliation   func(context.Context, *sql.DB, *sql.DB) (ownerreconcile.Report, error)
	ApplyReconciliation func(context.Context, *sql.DB, *sql.DB, string, string, int) (ownerreconcile.ApplySummary, error)
	Output              io.Writer
}

func defaultRuntimeDependencies() runtimeDependencies {
	open := func(databaseConfig *config.DatabaseConfig) (*sql.DB, error) {
		gormDB, err := database.NewDatabaseFromConfigWithoutCreate(databaseConfig)
		if err != nil || gormDB == nil {
			return nil, err
		}
		sqlDB, err := gormDB.DB()
		if err != nil {
			return nil, err
		}
		return sqlDB, nil
	}
	return runtimeDependencies{
		LoadConfig:     config.LoadConfigFromFileWithoutValidation,
		OpenDB:         open,
		OpenMetadataDB: open,
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
		CloseDB: func(db *sql.DB) error {
			if db == nil {
				return nil
			}
			return db.Close()
		},
		RunReconciliation: func(ctx context.Context, ownerDB, metadataDB *sql.DB) (ownerreconcile.Report, error) {
			repository, err := reconciliationRepository(ctx, ownerDB, metadataDB)
			if err != nil {
				return ownerreconcile.Report{}, err
			}
			return repository.DryRun(ctx, repository.Identities)
		},
		ApplyReconciliation: func(ctx context.Context, ownerDB, metadataDB *sql.DB, reportFingerprint, expected string, batchSize int) (ownerreconcile.ApplySummary, error) {
			repository, err := reconciliationRepository(ctx, ownerDB, metadataDB)
			if err != nil {
				return ownerreconcile.ApplySummary{}, err
			}
			return repository.ApplyUnique(ctx, reportFingerprint, expected, batchSize)
		},
		Output: os.Stdout,
	}
}

func reconciliationRepository(ctx context.Context, ownerDB, metadataDB *sql.DB) (ownerreconcile.Repository, error) {
	identities, err := ownerreconcile.LoadLegacyIdentities(ctx, metadataDB)
	if err != nil {
		return ownerreconcile.Repository{}, err
	}
	return ownerreconcile.Repository{Queryer: ownerDB, Inventory: ownerreconcile.Inventory(), Identities: identities, Beginner: ownerDB}, nil
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
	if deps.MetadataTableExists == nil {
		deps.MetadataTableExists = defaults.MetadataTableExists
	}
	if deps.CloseDB == nil {
		deps.CloseDB = defaults.CloseDB
	}
	if deps.RunReconciliation == nil {
		deps.RunReconciliation = defaults.RunReconciliation
	}
	if deps.ApplyReconciliation == nil {
		deps.ApplyReconciliation = defaults.ApplyReconciliation
	}
	if deps.Output == nil {
		deps.Output = defaults.Output
	}
	if options.BatchSize <= 0 {
		return errors.New("batch size must be positive")
	}
	cfg, err := deps.LoadConfig(options.ConfigPath())
	if err != nil {
		return errors.New("load config failed")
	}
	if cfg == nil || cfg.Database == nil {
		return errors.New("database is required for owner reconciliation")
	}
	ownerDB, err := deps.OpenDB(cfg.Database)
	if err != nil || ownerDB == nil {
		return errors.New("connect application database failed")
	}
	defer func() { _ = deps.CloseDB(ownerDB) }()
	metadataDB, err := openMetadataDB(ctx, cfg.Database, deps)
	if err != nil {
		return errors.New("connect legacy identity metadata database failed")
	}
	defer func() { _ = deps.CloseDB(metadataDB) }()
	report, err := deps.RunReconciliation(ctx, ownerDB, metadataDB)
	if err != nil {
		return err
	}
	report.SetMetadata(options.ConfigPath(), cfg.Database.Database)
	if err := report.SetFingerprint(); err != nil {
		return errors.New("fingerprint report failed")
	}
	if err := writeArtifacts(options, report); err != nil {
		return err
	}
	if options.Execute {
		if strings.TrimSpace(options.ConfirmReport) == "" {
			return ownerreconcile.ErrReportConfirmationMismatch
		}
		applied, err := deps.ApplyReconciliation(ctx, ownerDB, metadataDB, report.ReportFingerprint, options.ConfirmReport, options.BatchSize)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(deps.Output, "owner_reconciliation_report=%s rows=%d unresolved=%d updated=%d batches=%d\n", report.ReportFingerprint, report.Summary.AffectedRows, report.Summary.UnresolvedRows, applied.RowsUpdated, applied.Batches)
		return nil
	}
	_, _ = fmt.Fprintf(deps.Output, "owner_reconciliation_report=%s rows=%d unresolved=%d\n", report.ReportFingerprint, report.Summary.AffectedRows, report.Summary.UnresolvedRows)
	return nil
}

func openMetadataDB(ctx context.Context, base *config.DatabaseConfig, deps runtimeDependencies) (*sql.DB, error) {
	if base == nil {
		return nil, errors.New("metadata database base config is missing")
	}
	var selected *sql.DB
	for _, name := range []string{"zitadel_auth", "zitadel"} {
		candidate := *base
		candidate.Database = name
		db, err := deps.OpenMetadataDB(&candidate)
		if err != nil || db == nil {
			if db != nil {
				_ = deps.CloseDB(db)
			}
			continue
		}
		exists, probeErr := deps.MetadataTableExists(ctx, db)
		if probeErr != nil || !exists {
			_ = deps.CloseDB(db)
			continue
		}
		if selected != nil {
			_ = deps.CloseDB(db)
			_ = deps.CloseDB(selected)
			return nil, errors.New("multiple legacy identity metadata databases are available")
		}
		selected = db
	}
	if selected != nil {
		return selected, nil
	}
	return nil, errors.New("no legacy identity metadata database available")
}

func writeArtifacts(options Options, report ownerreconcile.Report) error {
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return errors.New("marshal owner reconciliation report failed")
	}
	if err := writeFile(options.Output, append(encoded, '\n')); err != nil {
		return err
	}
	preview := []byte("# Read-only owner reconciliation preview. No executable SQL is emitted.\n" + "# report_fingerprint=" + report.ReportFingerprint + "\n")
	for _, path := range []string{options.SQLOutput, options.SchemaOutput, options.BackfillOutput, options.SafeBackfillOutput} {
		if err := writeFile(path, preview); err != nil {
			return err
		}
	}
	if err := writeFile(options.ManualReviewOutput, append(encoded, '\n')); err != nil {
		return err
	}
	if err := writeFile(options.UnresolvedTasksJSON, append(encoded, '\n')); err != nil {
		return err
	}
	if err := writeFile(options.UnresolvedStudioJSON, append(encoded, '\n')); err != nil {
		return err
	}
	if err := writeFindingCSV(options.UnresolvedTasksCSV, report); err != nil {
		return err
	}
	if err := writeFindingCSV(options.UnresolvedStudioCSV, report); err != nil {
		return err
	}
	return writeFile(options.UnresolvedSummaryJSON, append(encoded, '\n'))
}

func writeFindingCSV(path string, report ownerreconcile.Report) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	if err := writer.Write([]string{"table", "tenant_fingerprint", "owner_fingerprint", "rows", "reason"}); err != nil {
		return err
	}
	for _, finding := range report.Findings {
		if err := writer.Write([]string{finding.Table, finding.TenantFingerprint, finding.OwnerFingerprint, fmt.Sprint(finding.Rows), finding.Reason}); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func writeFile(path string, contents []byte) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		return err
	}
	return nil
}
