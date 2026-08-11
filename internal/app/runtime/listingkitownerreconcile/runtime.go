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
	OpenWritableDB      func(*config.DatabaseConfig) (*sql.DB, error)
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
		LoadConfig: config.LoadConfigFromFileWithoutValidation,
		OpenDB:     open,
		OpenWritableDB: func(databaseConfig *config.DatabaseConfig) (*sql.DB, error) {
			gormDB, err := database.NewDatabaseFromConfigWithoutCreateWritable(databaseConfig)
			if err != nil || gormDB == nil {
				return nil, err
			}
			return gormDB.DB()
		},
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
	return ownerreconcile.Repository{Queryer: ownerDB, Inventory: ownerreconcile.Inventory(), Identities: identities, Exceptions: ownerreconcile.NewPostgresExceptionStore(ownerDB), Beginner: ownerDB}, nil
}

func Run(ctx context.Context, options Options) error {
	return runWithDependencies(ctx, options, defaultRuntimeDependencies())
}

func runWithDependencies(ctx context.Context, options Options, deps runtimeDependencies) error {
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
	openOwnerDB := deps.OpenDB
	if options.Execute {
		openOwnerDB = deps.OpenWritableDB
	}
	ownerDB, err := openOwnerDB(cfg.Database)
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
		if report.Summary.UnresolvedRows > 0 {
			return errors.New("owner reconciliation report contains unresolved rows")
		}
		if strings.TrimSpace(options.ConfirmReport) == "" {
			return ownerreconcile.ErrReportConfirmationMismatch
		}
		applied, err := deps.ApplyReconciliation(ctx, ownerDB, metadataDB, report.ReportFingerprint, options.ConfirmReport, options.BatchSize)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(deps.Output, "owner_reconciliation_report=%s rows=%d unresolved=%d updated=%d batches=%d\n", report.ReportFingerprint, report.Summary.AffectedRows, report.Summary.UnresolvedRows, applied.RowsUpdated, applied.Batches); err != nil {
			return errors.New("write owner reconciliation summary failed")
		}
		return nil
	}
	if _, err := fmt.Fprintf(deps.Output, "owner_reconciliation_report=%s rows=%d unresolved=%d\n", report.ReportFingerprint, report.Summary.AffectedRows, report.Summary.UnresolvedRows); err != nil {
		return errors.New("write owner reconciliation summary failed")
	}
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
		if err != nil {
			if !isMissingMetadataDatabaseError(err) {
				return nil, errors.New("connect legacy identity metadata database failed")
			}
			continue
		}
		if db == nil {
			return nil, errors.New("connect legacy identity metadata database failed")
		}
		exists, probeErr := deps.MetadataTableExists(ctx, db)
		if probeErr != nil {
			_ = deps.CloseDB(db)
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}
			return nil, errors.New("probe legacy identity metadata database failed")
		}
		if !exists {
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

func isMissingMetadataDatabaseError(err error) bool {
	if err == nil {
		return false
	}
	var stateErr interface{ SQLState() string }
	if errors.As(err, &stateErr) && stateErr.SQLState() == "3D000" {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "sqlstate 3d000") ||
		strings.Contains(message, "sqlstate=3d000") ||
		(strings.Contains(message, `database "`) && strings.Contains(message, " does not exist"))
}

func writeArtifacts(options Options, report ownerreconcile.Report) error {
	encoded, err := marshalReport(report)
	if err != nil {
		return errors.New("marshal owner reconciliation report failed")
	}
	tasksReport := filteredFindingReport(report, findingTask)
	tasksEncoded, err := marshalReport(tasksReport)
	if err != nil {
		return errors.New("marshal unresolved task report failed")
	}
	studioReport := filteredFindingReport(report, findingStudio)
	studioEncoded, err := marshalReport(studioReport)
	if err != nil {
		return errors.New("marshal unresolved studio report failed")
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
	if err := writeFile(options.UnresolvedTasksJSON, append(tasksEncoded, '\n')); err != nil {
		return err
	}
	if err := writeFile(options.UnresolvedStudioJSON, append(studioEncoded, '\n')); err != nil {
		return err
	}
	if err := writeFindingCSV(options.UnresolvedTasksCSV, tasksReport); err != nil {
		return err
	}
	if err := writeFindingCSV(options.UnresolvedStudioCSV, studioReport); err != nil {
		return err
	}
	return writeFile(options.UnresolvedSummaryJSON, append(encoded, '\n'))
}

type findingFamily uint8

const (
	findingOther findingFamily = iota
	findingTask
	findingStudio
)

func filteredFindingReport(report ownerreconcile.Report, family findingFamily) ownerreconcile.Report {
	findings := make([]ownerreconcile.Finding, 0, len(report.Findings))
	for _, finding := range report.Findings {
		if classifyFindingTable(finding.Table) == family {
			findings = append(findings, finding)
		}
	}
	filtered := ownerreconcile.NewReport(report.ConfigName, report.DatabaseName, findings, 0)
	filtered.GeneratedAt = report.GeneratedAt
	_ = filtered.SetFingerprint()
	return filtered
}

func classifyFindingTable(table string) findingFamily {
	switch strings.TrimSpace(table) {
	case "listing_kit_tasks", "listing_product_import_task", "listing_product_import_mapping", "listing_product_data":
		return findingTask
	case "listingkit_studio_async_jobs", "listingkit_studio_batches", "listingkit_studio_batch_items", "listingkit_studio_generation_attempts", "listingkit_studio_materialized_designs", "listingkit_studio_batch_task_links", "listingkit_studio_batch_runs", "listingkit_studio_batch_run_items", "shein_studio_sessions":
		return findingStudio
	default:
		return findingOther
	}
}

func marshalReport(report ownerreconcile.Report) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
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
