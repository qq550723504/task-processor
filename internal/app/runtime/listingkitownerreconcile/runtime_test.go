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
		OpenDB: func(*config.DatabaseConfig) (*sql.DB, error) { return ownerDB, nil },
		OpenMetadataDB: func(candidate *config.DatabaseConfig) (*sql.DB, error) {
			if candidate.Database != "zitadel_auth" {
				return nil, errors.New("missing")
			}
			return metadataDB, nil
		},
		MetadataTableExists: func(context.Context, *sql.DB) (bool, error) { return true, nil },
		CloseDB:             func(*sql.DB) error { return nil },
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
		OpenDB: func(*config.DatabaseConfig) (*sql.DB, error) { return &sql.DB{}, nil },
		OpenMetadataDB: func(candidate *config.DatabaseConfig) (*sql.DB, error) {
			if candidate.Database != "zitadel_auth" {
				return nil, errors.New("missing")
			}
			return &sql.DB{}, nil
		},
		MetadataTableExists: func(context.Context, *sql.DB) (bool, error) { return true, nil },
		CloseDB:             func(*sql.DB) error { return nil },
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
	readOpens := 0
	writableOpens := 0
	deps := runtimeDependencies{
		LoadConfig: func(string) (*config.Config, error) {
			return &config.Config{Database: &config.DatabaseConfig{Database: "app-db"}}, nil
		},
		OpenDB:         func(*config.DatabaseConfig) (*sql.DB, error) { readOpens++; return &sql.DB{}, nil },
		OpenWritableDB: func(*config.DatabaseConfig) (*sql.DB, error) { writableOpens++; return &sql.DB{}, nil },
		OpenMetadataDB: func(candidate *config.DatabaseConfig) (*sql.DB, error) {
			if candidate.Database != "zitadel_auth" {
				return nil, errors.New("missing")
			}
			return &sql.DB{}, nil
		},
		MetadataTableExists: func(context.Context, *sql.DB) (bool, error) { return true, nil },
		CloseDB:             func(*sql.DB) error { return nil },
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
	if readOpens != 0 || writableOpens != 2 {
		t.Fatalf("read opens=%d writable opens=%d, want 0 and 2", readOpens, writableOpens)
	}
}

func TestOpenMetadataDBFailsClosedOnProbeError(t *testing.T) {
	db := &sql.DB{}
	opened := 0
	closed := 0
	deps := runtimeDependencies{
		OpenMetadataDB: func(*config.DatabaseConfig) (*sql.DB, error) {
			opened++
			return db, nil
		},
		MetadataTableExists: func(context.Context, *sql.DB) (bool, error) {
			return false, errors.New("metadata probe denied")
		},
		CloseDB: func(*sql.DB) error {
			closed++
			return nil
		},
	}
	if _, err := openMetadataDB(context.Background(), &config.DatabaseConfig{Database: "app-db"}, deps); err == nil {
		t.Fatal("expected metadata probe error to fail closed")
	}
	if opened != 1 || closed != 1 {
		t.Fatalf("opened=%d closed=%d, want one attempted and closed candidate", opened, closed)
	}
}

func TestRunWithDependenciesBlocksExecuteWhenReportHasUnresolvedRows(t *testing.T) {
	applyCalled := false
	deps := runtimeDependencies{
		LoadConfig: func(string) (*config.Config, error) {
			return &config.Config{Database: &config.DatabaseConfig{Database: "app-db"}}, nil
		},
		OpenWritableDB: func(*config.DatabaseConfig) (*sql.DB, error) { return &sql.DB{}, nil },
		OpenMetadataDB: func(candidate *config.DatabaseConfig) (*sql.DB, error) {
			if candidate.Database != "zitadel_auth" {
				return nil, errors.New("missing")
			}
			return &sql.DB{}, nil
		},
		MetadataTableExists: func(context.Context, *sql.DB) (bool, error) { return true, nil },
		CloseDB:             func(*sql.DB) error { return nil },
		RunReconciliation: func(context.Context, *sql.DB, *sql.DB) (ownerreconcile.Report, error) {
			return ownerreconcile.NewReport("config.yaml", "app-db", []ownerreconcile.Finding{{Table: "listing_store", Rows: 1, Reason: "no_candidate"}}, 0), nil
		},
		ApplyReconciliation: func(context.Context, *sql.DB, *sql.DB, string, string, int) (ownerreconcile.ApplySummary, error) {
			applyCalled = true
			return ownerreconcile.ApplySummary{}, nil
		},
	}
	err := runWithDependencies(context.Background(), Options{Config: "config.yaml", Execute: true, ConfirmReport: "confirmed", Output: filepath.Join(t.TempDir(), "report.json"), BatchSize: 10}, deps)
	if err == nil || err.Error() != "owner reconciliation report contains unresolved rows" {
		t.Fatalf("error = %v, want unresolved-row rejection", err)
	}
	if applyCalled {
		t.Fatal("apply must not run when unresolved rows remain")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("closed") }

func TestRunWithDependenciesPropagatesSummaryWriteFailure(t *testing.T) {
	deps := runtimeDependencies{
		LoadConfig: func(string) (*config.Config, error) {
			return &config.Config{Database: &config.DatabaseConfig{Database: "app-db"}}, nil
		},
		OpenDB: func(*config.DatabaseConfig) (*sql.DB, error) { return &sql.DB{}, nil },
		OpenMetadataDB: func(candidate *config.DatabaseConfig) (*sql.DB, error) {
			if candidate.Database != "zitadel_auth" {
				return nil, errors.New("missing")
			}
			return &sql.DB{}, nil
		},
		MetadataTableExists: func(context.Context, *sql.DB) (bool, error) { return true, nil },
		CloseDB:             func(*sql.DB) error { return nil },
		RunReconciliation: func(context.Context, *sql.DB, *sql.DB) (ownerreconcile.Report, error) {
			return ownerreconcile.NewReport("config.yaml", "app-db", nil, 0), nil
		},
		Output: failingWriter{},
	}
	err := runWithDependencies(context.Background(), Options{Config: "config.yaml", Output: filepath.Join(t.TempDir(), "report.json"), BatchSize: 10}, deps)
	if err == nil || err.Error() != "write owner reconciliation summary failed" {
		t.Fatalf("error = %v, want summary write failure", err)
	}
}
