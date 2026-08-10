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

func TestRunWithDependenciesRejectsExecuteUntilReportConfirmationIsImplemented(t *testing.T) {
	deps := runtimeDependencies{LoadConfig: func(string) (*config.Config, error) {
		return &config.Config{Database: &config.DatabaseConfig{Database: "app-db"}}, nil
	}}
	err := runWithDependencies(context.Background(), Options{Execute: true, ConfirmReport: "abc123"}, deps)
	if err == nil || !errors.Is(err, ErrExecuteRequiresConfirmation) {
		t.Fatalf("error = %v, want execute confirmation error", err)
	}
}
