package main

import (
	"bytes"
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"task-processor/internal/core/config"
	corelogger "task-processor/internal/core/logger"
)

func TestParseDefaultsToProductionConfigAndBatchSize200(t *testing.T) {
	opts, err := parseArgs(nil)
	if err != nil {
		t.Fatal(err)
	}
	if opts.configPath != "config/config-prod.yaml" {
		t.Fatalf("config path = %q", opts.configPath)
	}
	if opts.batchSize != 200 {
		t.Fatalf("batch size = %d, want 200", opts.batchSize)
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

func TestRunMigratesBeforeBackfillAndOutputsOnlyCountAndDuration(t *testing.T) {
	db := &gorm.DB{}
	var calls []string
	var output bytes.Buffer
	times := []time.Time{
		time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 23, 10, 0, 2, 0, time.UTC),
	}
	nowIndex := 0

	err := run(context.Background(), nil, &output, dependencies{
		loadConfig: func(path string) (*config.Config, error) {
			calls = append(calls, "load")
			if path != "config/config-prod.yaml" {
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
		backfill: func(_ context.Context, got *gorm.DB, batchSize int) (int64, error) {
			calls = append(calls, "backfill")
			if got != db || batchSize != 200 {
				t.Fatalf("backfill database/batch = %p/%d", got, batchSize)
			}
			return 17, nil
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
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"load", "open", "migrate", "backfill", "close"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
	if got, want := output.String(), "processed=17 duration=2s\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}
