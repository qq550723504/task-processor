package listingkitownerexceptions

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"

	"task-processor/internal/core/config"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestRunWithDependenciesRejectsUnapprovedReportBeforeDatabaseAccess(t *testing.T) {
	path := t.TempDir() + "\\report.json"
	if err := os.WriteFile(path, []byte(`{"report_fingerprint":"deadbeefdead","summary":{"finding_groups":0,"unresolved_rows":0},"findings":[]}`), 0600); err != nil {
		t.Fatal(err)
	}

	err := runWithDependencies(context.Background(), Options{Report: path, ConfirmReport: "648cdfab03c4"}, runtimeDependencies{})
	if err == nil || !strings.Contains(err.Error(), "approved") {
		t.Fatalf("err = %v, want approved report validation error", err)
	}
}

func TestOpenMetadataDBRejectsAmbiguousCandidates(t *testing.T) {
	authDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer authDB.Close()
	zitadelDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer zitadelDB.Close()
	deps := defaultRuntimeDependencies()
	deps.OpenMetadataDB = func(cfg *config.DatabaseConfig) (*sql.DB, error) {
		if cfg.Database == "zitadel_auth" {
			return authDB, nil
		}
		return zitadelDB, nil
	}
	deps.MetadataTableExists = func(context.Context, *sql.DB) (bool, error) { return true, nil }
	deps.CloseDB = func(db *sql.DB) error { return nil }

	if _, err := openMetadataDB(context.Background(), &config.DatabaseConfig{}, deps); err == nil || !strings.Contains(err.Error(), "multiple") {
		t.Fatalf("err = %v, want ambiguous metadata databases to fail closed", err)
	}
}

func TestOpenMetadataDBRejectsNonMissingConnectionError(t *testing.T) {
	deps := defaultRuntimeDependencies()
	deps.OpenMetadataDB = func(cfg *config.DatabaseConfig) (*sql.DB, error) {
		if cfg.Database == "zitadel_auth" {
			return nil, errors.New("connection refused")
		}
		return nil, errors.New("database does not exist")
	}
	if _, err := openMetadataDB(context.Background(), &config.DatabaseConfig{}, deps); err == nil || !strings.Contains(err.Error(), "connect") {
		t.Fatalf("err = %v, want non-missing metadata connection failure", err)
	}
}
