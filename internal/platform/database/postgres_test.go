package database

import (
	"errors"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestOpenExistingReadOnlyDoesNotCreateOrRetryMissingDatabase(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Host:     "db.internal",
		Port:     5432,
		User:     "preflight",
		Password: "private-password",
		Database: "missing_listingkit",
	}
	openCalls := 0
	var openedDSN string

	_, err := openExistingReadOnly(cfg, func(dsn string) (*gorm.DB, error) {
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

func TestOpenStillCreatesAndRetriesMissingDatabase(t *testing.T) {
	t.Parallel()

	cfg := &Config{Database: "missing_listingkit"}
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

	db, err := openDatabase(cfg, databaseOpenOptions{createIfMissing: true}, func(string) (*gorm.DB, error) {
		openCalls++
		if openCalls == 1 {
			return nil, errors.New(`database "missing_listingkit" does not exist`)
		}
		return wantDB, nil
	}, func(*Config) error {
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

func TestOpenExistingWritableDoesNotCreateOrForceReadOnly(t *testing.T) {
	t.Parallel()

	cfg := &Config{Database: "missing_listingkit"}
	openCalls := 0
	var openedDSN string
	_, err := openDatabase(cfg, databaseOpenOptions{createIfMissing: false}, func(dsn string) (*gorm.DB, error) {
		openCalls++
		openedDSN = dsn
		return nil, errors.New(`database "missing_listingkit" does not exist`)
	}, nil)
	if err == nil {
		t.Fatal("writable strict database open error = nil, want missing database failure")
	}
	if openCalls != 1 {
		t.Fatalf("writable strict database open calls = %d, want 1", openCalls)
	}
	if strings.Contains(openedDSN, "default_transaction_read_only=on") {
		t.Fatalf("writable strict database DSN unexpectedly forces read-only: %q", openedDSN)
	}
}

func TestOpenReturnsNilForNilConfig(t *testing.T) {
	t.Parallel()

	db, err := Open(nil)
	if err != nil {
		t.Fatalf("Open(nil) error = %v", err)
	}
	if db != nil {
		t.Fatalf("Open(nil) database = %p, want nil", db)
	}
}

func TestOpenSharedReusesDatabaseUntilLastReferenceCloses(t *testing.T) {
	cfg := &Config{Host: "db", Port: 5432, User: "worker", Database: "tasks"}
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("open SQL mock: %v", err)
	}
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("open GORM test database: %v", err)
	}

	key := sharedDatabaseKey(cfg)
	sharedDatabases.mu.Lock()
	sharedDatabases.entries[key] = &sharedDatabaseEntry{db: db, refs: 1}
	sharedDatabases.mu.Unlock()
	t.Cleanup(func() {
		sharedDatabases.mu.Lock()
		delete(sharedDatabases.entries, key)
		sharedDatabases.mu.Unlock()
	})

	got, err := OpenShared(cfg)
	if err != nil {
		t.Fatalf("OpenShared() error = %v", err)
	}
	if got != db {
		t.Fatalf("OpenShared() database = %p, want %p", got, db)
	}

	if err := CloseShared(cfg, db); err != nil {
		t.Fatalf("first CloseShared() error = %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("shared database closed with one reference remaining: %v", err)
	}

	mock.ExpectClose()
	if err := CloseShared(cfg, db); err != nil {
		t.Fatalf("last CloseShared() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations: %v", err)
	}
}
