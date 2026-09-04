package storehistorymigrate

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/storecenter"

	"gorm.io/gorm"
)

func TestLoadManifestUsesStrictJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "manifest.json")
	valid := `{"schema_version":"store-service-history/no-authoritative-source/v1","decision_reference":"product-decision:store-service-history:phase1","approved_by":"repository-owner","approved_at":"2026-09-04T14:00:00Z"}`
	if err := os.WriteFile(path, []byte(valid), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest, err := LoadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.DecisionReference != "product-decision:store-service-history:phase1" {
		t.Fatalf("manifest = %+v", manifest)
	}

	if err := os.WriteFile(path, []byte(strings.TrimSuffix(valid, "}")+`,"unexpected":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadManifest(path); err == nil {
		t.Fatal("LoadManifest() accepted an unknown field")
	}
	if err := os.WriteFile(path, []byte(valid+` {}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadManifest(path); err == nil {
		t.Fatal("LoadManifest() accepted trailing JSON")
	}
}

func TestRunDefaultsToReadOnlyVerifyAndWritesMachineReadableReport(t *testing.T) {
	fake := &fakeHistoryMigrator{verifyReport: storecenter.StoreHistoryMigrationReport{ScannedCount: 4, HistoryConfirmedAbsentCount: 3, UnknownHistoryPendingActivationCount: 2, ReadyForConstraints: true}}
	var output bytes.Buffer
	closed := false
	readOnlyOpened := false
	writableOpened := false
	err := runWithDependencies(context.Background(), Options{Config: "config/test.yaml", Manifest: "manifest.json", LogLevel: "error"}, &output, runtimeDependencies{
		LoadConfig: func(string) (*config.Config, error) {
			return &config.Config{Database: &config.DatabaseConfig{}}, nil
		},
		OpenDB: func(*config.DatabaseConfig) (*gorm.DB, error) {
			readOnlyOpened = true
			return &gorm.DB{}, nil
		},
		OpenWritableDB: func(*config.DatabaseConfig) (*gorm.DB, error) {
			writableOpened = true
			return &gorm.DB{}, nil
		},
		CloseDB: func(*gorm.DB) error { closed = true; return nil },
		LoadManifest: func(string) (storecenter.NoAuthoritativeHistorySourceManifest, error) {
			return validManifest(), nil
		},
		NewMigrator: func(*gorm.DB, storecenter.NoAuthoritativeHistorySourceManifest, string, func() time.Time) (historyMigrator, error) {
			return fake, nil
		},
	})
	if err != nil {
		t.Fatalf("runWithDependencies() error = %v", err)
	}
	if fake.verifyCalls != 1 || fake.backfillCalls != 0 || !closed || !readOnlyOpened || writableOpened {
		t.Fatalf("verify/backfill/closed/read-only/writable = %d/%d/%v/%v/%v", fake.verifyCalls, fake.backfillCalls, closed, readOnlyOpened, writableOpened)
	}
	if !strings.Contains(output.String(), `"unknown_history_pending_activation_count":2`) || !strings.Contains(output.String(), `"ready_for_constraints":true`) {
		t.Fatalf("report output = %q", output.String())
	}
}

func TestRunBackfillRequiresExplicitActionAndUsesBoundedBatch(t *testing.T) {
	fake := &fakeHistoryMigrator{backfillReport: storecenter.StoreHistoryMigrationReport{ScannedCount: 25, UpdatedCount: 25}}
	var output bytes.Buffer
	readOnlyOpened := false
	writableOpened := false
	err := runWithDependencies(context.Background(), Options{Config: "config/test.yaml", Manifest: "manifest.json", Action: "backfill", BatchSize: 25, LogLevel: "error"}, &output, runtimeDependencies{
		LoadConfig: func(string) (*config.Config, error) { return &config.Config{Database: &config.DatabaseConfig{}}, nil },
		OpenDB: func(*config.DatabaseConfig) (*gorm.DB, error) {
			readOnlyOpened = true
			return &gorm.DB{}, nil
		},
		OpenWritableDB: func(*config.DatabaseConfig) (*gorm.DB, error) {
			writableOpened = true
			return &gorm.DB{}, nil
		},
		CloseDB: func(*gorm.DB) error { return nil },
		LoadManifest: func(string) (storecenter.NoAuthoritativeHistorySourceManifest, error) {
			return validManifest(), nil
		},
		NewMigrator: func(*gorm.DB, storecenter.NoAuthoritativeHistorySourceManifest, string, func() time.Time) (historyMigrator, error) {
			return fake, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if fake.backfillCalls != 1 || fake.batchSize != 25 || fake.verifyCalls != 0 || readOnlyOpened || !writableOpened {
		t.Fatalf("backfill calls/batch/verify/read-only/writable = %d/%d/%d/%v/%v", fake.backfillCalls, fake.batchSize, fake.verifyCalls, readOnlyOpened, writableOpened)
	}
}

func TestRunRejectsUnknownActionBeforeRuntimeSideEffects(t *testing.T) {
	called := false
	err := runWithDependencies(context.Background(), Options{Action: "enable", Manifest: "manifest.json"}, &bytes.Buffer{}, runtimeDependencies{
		LoadConfig: func(string) (*config.Config, error) {
			called = true
			return nil, nil
		},
	})
	if !errors.Is(err, flag.ErrHelp) || called {
		t.Fatalf("runWithDependencies() = %v, called=%v; want help before side effects", err, called)
	}
}

func TestRunRejectsNilDatabaseConfigWithoutPanicking(t *testing.T) {
	err := runWithDependencies(context.Background(), Options{Manifest: "manifest.json"}, &bytes.Buffer{}, runtimeDependencies{
		LoadManifest: func(string) (storecenter.NoAuthoritativeHistorySourceManifest, error) {
			return validManifest(), nil
		},
		LoadConfig: func(string) (*config.Config, error) { return nil, nil },
	})
	if err == nil || !strings.Contains(err.Error(), "database config is required") {
		t.Fatalf("runWithDependencies() error = %v, want missing database config", err)
	}
}

func TestParseFlagsAdvertisesSafeDefaults(t *testing.T) {
	fs := flag.NewFlagSet("store-service-history-migrate", flag.ContinueOnError)
	opts := ParseFlagsFrom(fs, "--manifest", "decision.json")
	if opts.Action != "verify" || opts.BatchSize != 100 || opts.Manifest != "decision.json" {
		t.Fatalf("options = %+v", opts)
	}
}

func validManifest() storecenter.NoAuthoritativeHistorySourceManifest {
	return storecenter.NoAuthoritativeHistorySourceManifest{
		SchemaVersion: storecenter.NoAuthoritativeHistorySourceManifestV1, DecisionReference: "product-decision:store-service-history:phase1",
		ApprovedBy: "repository-owner", ApprovedAt: time.Date(2026, 9, 4, 14, 0, 0, 0, time.UTC),
	}
}

type fakeHistoryMigrator struct {
	verifyCalls, backfillCalls int
	batchSize                  int
	verifyReport               storecenter.StoreHistoryMigrationReport
	backfillReport             storecenter.StoreHistoryMigrationReport
}

func (fake *fakeHistoryMigrator) Verify(context.Context) (storecenter.StoreHistoryMigrationReport, error) {
	fake.verifyCalls++
	return fake.verifyReport, nil
}

func (fake *fakeHistoryMigrator) BackfillBatch(_ context.Context, batchSize int) (storecenter.StoreHistoryMigrationReport, error) {
	fake.backfillCalls++
	fake.batchSize = batchSize
	return fake.backfillReport, nil
}
