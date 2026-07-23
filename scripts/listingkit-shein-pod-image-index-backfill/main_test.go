package main

import (
	"bytes"
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"

	"task-processor/internal/core/config"
	corelogger "task-processor/internal/core/logger"
	"task-processor/internal/listingkit"
	listingkitstore "task-processor/internal/listingkit/store"
)

func TestParseRequiresExplicitConfig(t *testing.T) {
	_, err := parseArgs(nil)
	if err == nil || !strings.Contains(err.Error(), "-config is required") {
		t.Fatalf("parse error = %v, want explicit config requirement", err)
	}
}

func TestParseAcceptsExplicitConfigAndBatchSize(t *testing.T) {
	opts, err := parseArgs([]string{"-config", "config/custom.yaml", "-batch-size", "25"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.configPath != "config/custom.yaml" || opts.batchSize != 25 {
		t.Fatalf("unexpected options: %+v", opts)
	}
}

func TestDefaultConfigLoaderDoesNotWriteGlobalLogsToStdout(t *testing.T) {
	globalLogger := corelogger.GetGlobalLogManager().GetRawLogger()
	originalOutput := globalLogger.Out
	var stdout bytes.Buffer
	globalLogger.SetOutput(&stdout)
	t.Cleanup(func() {
		globalLogger.SetOutput(originalOutput)
	})

	_, err := defaultDependencies().loadConfig(filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil {
		t.Fatal("expected missing config error")
	}
	if stdout.Len() != 0 {
		t.Fatalf("default config loader wrote to stdout: %q", stdout.String())
	}
}

func TestRunMainWritesErrorsOnlyToStderr(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runMain(
		context.Background(),
		[]string{"-unknown"},
		&stdout,
		&stderr,
		dependencies{},
	)

	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "flag provided but not defined") {
		t.Fatalf("stderr = %q, want parse error", stderr.String())
	}
}

func TestRunMainPreservesStdoutContractAndWritesDiagnosticsToStderr(t *testing.T) {
	db := &gorm.DB{}
	var calls []string
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	times := []time.Time{
		time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 23, 10, 0, 2, 0, time.UTC),
	}
	nowIndex := 0

	deps := dependencies{
		loadConfig: func(path string) (*config.Config, error) {
			calls = append(calls, "load")
			if path != "config/production.yaml" {
				t.Fatalf("config path = %q", path)
			}
			return &config.Config{Database: &config.DatabaseConfig{}}, nil
		},
		openDB: func(*config.DatabaseConfig) (*gorm.DB, error) {
			calls = append(calls, "open")
			return db, nil
		},
		migrate: func(got *gorm.DB) error {
			calls = append(calls, "migrate")
			if got != db {
				t.Fatal("migrate received unexpected database")
			}
			return nil
		},
		backfill: func(_ context.Context, got *gorm.DB, batchSize int) (listingkitstore.SheinPODImageLookupBackfillSummary, error) {
			calls = append(calls, "backfill")
			if got != db || batchSize != 200 {
				t.Fatalf("backfill database/batch = %p/%d", got, batchSize)
			}
			return listingkitstore.SheinPODImageLookupBackfillSummary{
				Processed:        17,
				SkippedMalformed: 1,
				MalformedRows: []listingkitstore.SheinPODImageLookupBackfillMalformedRow{{
					TaskID: "malformed-task",
					Field:  "result",
					Reason: "invalid_json",
				}},
			}, nil
		},
		closeDB: func(got *gorm.DB) error {
			calls = append(calls, "close")
			if got != db {
				t.Fatal("close received unexpected database")
			}
			return nil
		},
		now: func() time.Time {
			value := times[nowIndex]
			nowIndex++
			return value
		},
	}
	exitCode := runMain(
		context.Background(),
		[]string{"-config", "config/production.yaml"},
		&stdout,
		&stderr,
		deps,
	)
	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
	}
	if want := []string{"load", "open", "migrate", "backfill", "close"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
	if got, want := stdout.String(), "processed=17 duration=2s\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if got, want := stderr.String(), "skipped_malformed=1\n"+
		"skipped_malformed task_id=\"malformed-task\" field=result reason=invalid_json\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func TestDefaultMigrationCreatesOnlyPODImageLookupIndexSchema(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	if err := defaultDependencies().migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if !db.Migrator().HasTable(&listingkit.SheinPODImageLookupIndex{}) {
		t.Fatal("POD image lookup index table was not created")
	}
	if db.Migrator().HasTable(&listingkit.Task{}) {
		t.Fatal("backfill migration unexpectedly ran unrelated ListingKit task migrations")
	}
}
