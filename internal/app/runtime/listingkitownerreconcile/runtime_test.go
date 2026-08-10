package listingkitownerreconcile

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"task-processor/internal/core/config"
	"task-processor/internal/listingkit/ownerreconcile"
)

func TestRunWithDependenciesWritesReportInDryRunAndDoesNotExecute(t *testing.T) {
	outputDir := t.TempDir()
	ownerDB := &sql.DB{}
	metadataDB := &sql.DB{}
	called := false
	deps := runtimeDependencies{
		LoadConfig: func(string) (*config.Config, error) {
			return &config.Config{Database: &config.DatabaseConfig{Database: "app-db"}}, nil
		},
		OpenDB:         func(*config.DatabaseConfig) (*sql.DB, error) { return ownerDB, nil },
		OpenMetadataDB: func(*config.DatabaseConfig) (*sql.DB, error) { return metadataDB, nil },
		CloseDB:        func(*sql.DB) error { return nil },
		RunReconciliation: func(context.Context, *sql.DB, *sql.DB) (ownerreconcile.Report, error) {
			called = true
			return ownerreconcile.NewReport("config.yaml", "app-db", nil, 4), nil
		},
		Output: nil,
	}
	opts := Options{Config: "config.yaml", Output: filepath.Join(outputDir, "report.json"), SQLOutput: filepath.Join(outputDir, "read.sql"), SafeBackfillOutput: filepath.Join(outputDir, "safe.sql"), BatchSize: 500}
	if err := runWithDependencies(context.Background(), opts, deps); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("expected reconciliation to run")
	}
	if _, err := os.Stat(opts.Output); err != nil {
		t.Fatalf("report was not written: %v", err)
	}
}

func TestRunWithDependenciesRejectsExecuteWithoutConfirmationBeforeApply(t *testing.T) {
	applyCalled := false
	deps := runtimeDependencies{
		LoadConfig: func(string) (*config.Config, error) {
			return &config.Config{Database: &config.DatabaseConfig{Database: "app-db"}}, nil
		},
		OpenDB:         func(*config.DatabaseConfig) (*sql.DB, error) { return &sql.DB{}, nil },
		OpenMetadataDB: func(*config.DatabaseConfig) (*sql.DB, error) { return &sql.DB{}, nil },
		CloseDB:        func(*sql.DB) error { return nil },
		RunReconciliation: func(context.Context, *sql.DB, *sql.DB) (ownerreconcile.Report, error) {
			return ownerreconcile.NewReport("config.yaml", "app-db", nil, 0), nil
		},
		ApplyReconciliation: func(context.Context, *sql.DB, *sql.DB, string, string, int) (ownerreconcile.ApplySummary, error) {
			applyCalled = true
			return ownerreconcile.ApplySummary{}, nil
		},
	}
	err := runWithDependencies(context.Background(), Options{Config: "config.yaml", Execute: true, Output: filepath.Join(t.TempDir(), "report.json"), BatchSize: 500}, deps)
	if err == nil || !errors.Is(err, ownerreconcile.ErrReportConfirmationMismatch) {
		t.Fatalf("error = %v, want confirmation mismatch", err)
	}
	if applyCalled {
		t.Fatal("apply must not run without confirmation")
	}
}

func TestRunWithDependenciesExecutesOnlyAfterFreshReportConfirmation(t *testing.T) {
	outputDir := t.TempDir()
	var confirmed string
	deps := runtimeDependencies{
		LoadConfig: func(string) (*config.Config, error) {
			return &config.Config{Database: &config.DatabaseConfig{Database: "app-db"}}, nil
		},
		OpenDB:         func(*config.DatabaseConfig) (*sql.DB, error) { return &sql.DB{}, nil },
		OpenMetadataDB: func(*config.DatabaseConfig) (*sql.DB, error) { return &sql.DB{}, nil },
		CloseDB:        func(*sql.DB) error { return nil },
		RunReconciliation: func(context.Context, *sql.DB, *sql.DB) (ownerreconcile.Report, error) {
			return ownerreconcile.NewReport("config.yaml", "app-db", nil, 4), nil
		},
		ApplyReconciliation: func(_ context.Context, _ *sql.DB, _ *sql.DB, reportFingerprint, expected string, batchSize int) (ownerreconcile.ApplySummary, error) {
			if batchSize != 20 || reportFingerprint != expected || reportFingerprint == "" {
				t.Fatalf("unsafe apply arguments: report=%q expected=%q batch=%d", reportFingerprint, expected, batchSize)
			}
			confirmed = reportFingerprint
			return ownerreconcile.ApplySummary{RowsUpdated: 4, Batches: 1}, nil
		},
	}
	opts := Options{Config: "config.yaml", Execute: true, ConfirmReport: "", Output: filepath.Join(outputDir, "report.json"), BatchSize: 20}
	first := runWithDependencies(context.Background(), opts, deps)
	if first == nil {
		t.Fatal("expected missing confirmation to fail")
	}

	// Generate the expected confirmation from the same deterministic report
	// shape; the runtime must still pass it through the callback unchanged.
	report := ownerreconcile.NewReport("config.yaml", "app-db", nil, 4)
	report.SetMetadata("config.yaml", "app-db")
	if err := report.SetFingerprint(); err != nil {
		t.Fatal(err)
	}
	opts.ConfirmReport = report.ReportFingerprint
	if err := runWithDependencies(context.Background(), opts, deps); err != nil {
		t.Fatal(err)
	}
	if confirmed != report.ReportFingerprint {
		t.Fatalf("confirmed fingerprint = %q, want %q", confirmed, report.ReportFingerprint)
	}
}
