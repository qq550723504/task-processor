package migration

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func TestPostgresSessionLockerSerializesIndependentPoolsAndProviders(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TASK_PROCESSOR_TEST_POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("set TASK_PROCESSOR_TEST_POSTGRES_DSN to run the independent PostgreSQL pool/provider lock integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	adminConfig, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse TASK_PROCESSOR_TEST_POSTGRES_DSN: %v", err)
	}
	adminDB := stdlib.OpenDB(*adminConfig)
	t.Cleanup(func() { _ = adminDB.Close() })
	if err := adminDB.PingContext(ctx); err != nil {
		t.Fatalf("ping PostgreSQL integration database: %v", err)
	}

	schemaName := fmt.Sprintf("goose_runner_%d", time.Now().UnixNano())
	if _, err := adminDB.ExecContext(ctx, "CREATE SCHEMA "+schemaName); err != nil {
		t.Fatalf("create isolated PostgreSQL schema: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = adminDB.ExecContext(cleanupCtx, "DROP SCHEMA "+schemaName+" CASCADE")
	})

	firstDB := openPostgresRunnerPool(t, dsn, schemaName)
	secondDB := openPostgresRunnerPool(t, dsn, schemaName)
	var runs atomic.Int32
	firstEntered := make(chan struct{})
	secondEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	newMigration := func() *goose.Migration {
		return goose.NewGoMigration(2026083199, &goose.GoFunc{
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
	firstRunner, err := New(goose.DialectPostgres, firstDB, newMigration())
	if err != nil {
		t.Fatalf("New() first runner error = %v", err)
	}
	secondRunner, err := New(goose.DialectPostgres, secondDB, newMigration())
	if err != nil {
		t.Fatalf("New() second runner error = %v", err)
	}

	firstResult := make(chan error, 1)
	go func() {
		_, err := firstRunner.Up(ctx)
		firstResult <- err
	}()
	select {
	case <-firstEntered:
	case <-ctx.Done():
		t.Fatalf("first runner did not enter migration: %v", ctx.Err())
	}

	secondStarted := make(chan struct{})
	secondResult := make(chan error, 1)
	go func() {
		close(secondStarted)
		_, err := secondRunner.Up(ctx)
		secondResult <- err
	}()
	<-secondStarted
	select {
	case <-secondEntered:
		close(releaseFirst)
		t.Fatal("second PostgreSQL provider entered migration while the first provider held the session lock")
	case <-time.After(150 * time.Millisecond):
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

func openPostgresRunnerPool(t *testing.T, dsn, schemaName string) *sql.DB {
	t.Helper()
	poolConfig, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse PostgreSQL DSN: %v", err)
	}
	if poolConfig.RuntimeParams == nil {
		poolConfig.RuntimeParams = make(map[string]string)
	}
	poolConfig.RuntimeParams["search_path"] = schemaName
	db := stdlib.OpenDB(*poolConfig)
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	t.Cleanup(func() { _ = db.Close() })
	return db
}
