package migration

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

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

func TestRunnerUpIsSafeForConcurrentUse(t *testing.T) {
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

	const callers = 8
	start := make(chan struct{})
	errors := make(chan error, callers)
	var wait sync.WaitGroup
	for range callers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, err := runner.Up(context.Background())
			errors <- err
		}()
	}
	close(start)
	wait.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatalf("concurrent Up() error = %v", err)
		}
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

func openRunnerTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "runner.sqlite"))
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
