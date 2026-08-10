package database

import (
	"errors"
	"strings"
	"testing"

	"task-processor/internal/core/config"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestNewDatabaseFromConfigWithoutCreateDoesNotCreateOrRetryMissingDatabase(t *testing.T) {
	t.Parallel()

	cfg := &config.DatabaseConfig{
		Host:     "db.internal",
		Port:     5432,
		User:     "preflight",
		Password: "private-password",
		Database: "missing_listingkit",
	}
	openCalls := 0
	var openedDSN string

	_, err := newDatabaseFromConfigWithoutCreate(cfg, func(dsn string) (*gorm.DB, error) {
		openCalls++
		openedDSN = dsn
		return nil, errors.New(`database "missing_listingkit" does not exist`)
	})
	if err == nil {
		t.Fatal("strict database open error = nil, want missing database failure")
	}
	if openCalls != 1 {
		t.Fatalf("strict database open calls = %d, want 1", openCalls)
	}
	if !strings.Contains(openedDSN, "default_transaction_read_only=on") {
		t.Fatalf("strict database DSN does not enable a read-only session: %q", openedDSN)
	}
}

func TestNewDatabaseFromConfigStillCreatesAndRetriesMissingDatabase(t *testing.T) {
	t.Parallel()

	cfg := &config.DatabaseConfig{Database: "missing_listingkit"}
	openCalls := 0
	createCalls := 0
	sqlDB, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("open SQL mock: %v", err)
	}
	wantDB, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("open GORM test database: %v", err)
	}
	mock.ExpectPing()
	t.Cleanup(func() {
		mock.ExpectClose()
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close SQL mock: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("SQL expectations: %v", err)
		}
	})

	db, err := newDatabaseFromConfig(cfg, databaseOpenOptions{createIfMissing: true}, func(string) (*gorm.DB, error) {
		openCalls++
		if openCalls == 1 {
			return nil, errors.New(`database "missing_listingkit" does not exist`)
		}
		return wantDB, nil
	}, func(*config.DatabaseConfig) error {
		createCalls++
		return nil
	})
	if err != nil {
		t.Fatalf("normal database open error = %v", err)
	}
	if db != wantDB {
		t.Fatalf("normal database = %p, want %p", db, wantDB)
	}
	if openCalls != 2 {
		t.Fatalf("normal database open calls = %d, want 2", openCalls)
	}
	if createCalls != 1 {
		t.Fatalf("normal database create calls = %d, want 1", createCalls)
	}
}
