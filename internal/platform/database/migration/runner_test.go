package migration

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
)

func TestRunnerUpIsIdempotentAndStatusReflectsAppliedMigration(t *testing.T) {
	db := openRunnerTestDB(t)
	var runs atomic.Int32
	migration := goose.NewGoMigration(2026083001, &goose.GoFunc{
		RunDB: func(context.Context, *sql.DB) error {
			runs.Add(1)
			return nil
		},
	}, nil)
	runner, err := New(goose.DialectSQLite3, db, migration)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	before, err := runner.Status(context.Background())
	if err != nil {
		t.Fatalf("Status() before Up error = %v", err)
	}
	assertMigrationStatus(t, before, 2026083001, goose.StatePending)

	first, err := runner.Up(context.Background())
	if err != nil {
		t.Fatalf("first Up() error = %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("first Up() results = %d, want 1", len(first))
	}
	second, err := runner.Up(context.Background())
	if err != nil {
		t.Fatalf("second Up() error = %v", err)
	}
	if len(second) != 0 {
		t.Fatalf("second Up() results = %d, want 0", len(second))
	}
	if got := runs.Load(); got != 1 {
		t.Fatalf("migration runs = %d, want 1", got)
	}

	after, err := runner.Status(context.Background())
	if err != nil {
		t.Fatalf("Status() after Up error = %v", err)
	}
	assertMigrationStatus(t, after, 2026083001, goose.StateApplied)
}

func TestRunnerUpSerializesIndependentSQLiteRunnersInOneProcess(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "runner.sqlite")
	firstDB := openRunnerTestDBAtPath(t, databasePath)
	secondDB := openRunnerTestDBAtPath(t, databasePath)
	var runs atomic.Int32
	firstEntered := make(chan struct{})
	secondEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	newMigration := func() *goose.Migration {
		return goose.NewGoMigration(2026083001, &goose.GoFunc{
			RunDB: func(context.Context, *sql.DB) error {
				switch runs.Add(1) {
				case 1:
					close(firstEntered)
					<-releaseFirst
				case 2:
					close(secondEntered)
				}
				return nil
			},
		}, nil)
	}
	firstRunner, err := New(goose.DialectSQLite3, firstDB, newMigration())
	if err != nil {
		t.Fatalf("New() first runner error = %v", err)
	}
	secondRunner, err := New(goose.DialectSQLite3, secondDB, newMigration())
	if err != nil {
		t.Fatalf("New() second runner error = %v", err)
	}

	firstResult := make(chan error, 1)
	go func() {
		_, err := firstRunner.Up(context.Background())
		firstResult <- err
	}()
	<-firstEntered

	secondStarted := make(chan struct{})
	secondResult := make(chan error, 1)
	go func() {
		close(secondStarted)
		_, err := secondRunner.Up(context.Background())
		secondResult <- err
	}()
	<-secondStarted

	select {
	case <-secondEntered:
		close(releaseFirst)
		t.Fatal("second runner entered migration before first runner released")
	case <-time.After(100 * time.Millisecond):
	}
	close(releaseFirst)
	if err := <-firstResult; err != nil {
		t.Fatalf("first Up() error = %v", err)
	}
	if err := <-secondResult; err != nil {
		t.Fatalf("second Up() error = %v", err)
	}
	if got := runs.Load(); got != 1 {
		t.Fatalf("migration runs = %d, want 1", got)
	}
}

func TestNewRejectsUnknownDialect(t *testing.T) {
	db := openRunnerTestDB(t)
	if _, err := New(goose.Dialect("unknown"), db); err == nil {
		t.Fatal("New() error = nil, want unsupported dialect error")
	}
}

func TestNewWithVersionTableKeepsIndependentHistory(t *testing.T) {
	db := openRunnerTestDB(t)
	migration := goose.NewGoMigration(2026090901, &goose.GoFunc{RunDB: func(context.Context, *sql.DB) error { return nil }}, nil)
	runner, err := NewWithVersionTable(goose.DialectSQLite3, db, "goose_source_account_registry_version", migration)
	if err != nil {
		t.Fatalf("NewWithVersionTable() error = %v", err)
	}
	if _, err := runner.Up(context.Background()); err != nil {
		t.Fatalf("Up() error = %v", err)
	}
	var custom, shared int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'goose_source_account_registry_version'`).Scan(&custom); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'goose_db_version'`).Scan(&shared); err != nil {
		t.Fatal(err)
	}
	if custom != 1 || shared != 0 {
		t.Fatalf("version tables custom=%d shared=%d", custom, shared)
	}
	if _, err := NewWithVersionTable(goose.DialectSQLite3, db, "", migration); err == nil {
		t.Fatal("empty version table accepted")
	}
}

func TestVersionTableValidationAllowsOnlySafePostgresQualification(t *testing.T) {
	for _, tt := range []struct {
		name    string
		dialect goose.Dialect
		table   string
		want    bool
	}{
		{name: "unqualified", dialect: goose.DialectSQLite3, table: "goose_source_account_registry_version", want: true},
		{name: "qualified postgres", dialect: goose.DialectPostgres, table: "public.goose_source_account_registry_version", want: true},
		{name: "qualified sqlite", dialect: goose.DialectSQLite3, table: "main.goose_source_account_registry_version", want: false},
		{name: "empty schema", dialect: goose.DialectPostgres, table: ".goose_version", want: false},
		{name: "empty table", dialect: goose.DialectPostgres, table: "public.", want: false},
		{name: "extra qualification", dialect: goose.DialectPostgres, table: "database.public.goose_version", want: false},
		{name: "injection", dialect: goose.DialectPostgres, table: "public.goose_version;drop_table", want: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := validVersionTableName(tt.dialect, tt.table); got != tt.want {
				t.Fatalf("validVersionTableName(%q, %q) = %t, want %t", tt.dialect, tt.table, got, tt.want)
			}
		})
	}
}

func openRunnerTestDB(t *testing.T) *sql.DB {
	t.Helper()
	return openRunnerTestDBAtPath(t, filepath.Join(t.TempDir(), "runner.sqlite"))
}

func openRunnerTestDBAtPath(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func assertMigrationStatus(t *testing.T, statuses []*goose.MigrationStatus, version int64, state goose.State) {
	t.Helper()
	if len(statuses) != 1 {
		t.Fatalf("migration statuses = %d, want 1", len(statuses))
	}
	if statuses[0].Source == nil {
		t.Fatal("migration status source is nil")
	}
	if got := statuses[0].Source.Version; got != version {
		t.Fatalf("migration version = %d, want %d", got, version)
	}
	if got := statuses[0].State; got != state {
		t.Fatalf("migration state = %q, want %q", got, state)
	}
}
