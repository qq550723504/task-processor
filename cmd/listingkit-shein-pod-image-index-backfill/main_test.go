package main

import (
	"bytes"
	"context"
	"reflect"
	"testing"
	"time"

	"gorm.io/gorm"

	"task-processor/internal/core/config"
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
