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

func TestRunConstraintsRequiresExplicitActionAndUsesWritableDatabase(t *testing.T) {
	fake := &fakeHistoryMigrator{constraintReport: storecenter.StoreServiceConstraintReport{
		PhaseD:             storecenter.StoreHistoryMigrationReport{ReadyForConstraints: true},
		ConstraintsApplied: true,
	}}
	var output bytes.Buffer
	readOnlyOpened := false
	writableOpened := false
	err := runWithDependencies(context.Background(), Options{
		Config: "config/test.yaml", Manifest: "manifest.json", Action: "constraints", LogLevel: "error",
		ConstraintLockTimeout: 750 * time.Millisecond, ConstraintStatementTimeout: 45 * time.Second,
	}, &output, runtimeDependencies{
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
	if fake.constraintCalls != 1 || fake.verifyCalls != 0 || fake.backfillCalls != 0 || readOnlyOpened || !writableOpened {
		t.Fatalf("constraints/verify/backfill/read-only/writable = %d/%d/%d/%v/%v", fake.constraintCalls, fake.verifyCalls, fake.backfillCalls, readOnlyOpened, writableOpened)
	}
	if fake.constraintOptions.LockTimeout != 750*time.Millisecond || fake.constraintOptions.StatementTimeout != 45*time.Second {
		t.Fatalf("constraint options = %+v", fake.constraintOptions)
	}
	if !strings.Contains(output.String(), `"constraints_applied":true`) {
		t.Fatalf("constraint report output = %q", output.String())
	}
}

func TestRunConstraintsRejectsUnboundedTimeoutsBeforeRuntimeSideEffects(t *testing.T) {
	for _, options := range []Options{
		{Action: "constraints", Manifest: "manifest.json", ConstraintLockTimeout: 0, ConstraintStatementTimeout: time.Second},
		{Action: "constraints", Manifest: "manifest.json", ConstraintLockTimeout: time.Second, ConstraintStatementTimeout: 0},
		{Action: "constraints", Manifest: "manifest.json", ConstraintLockTimeout: 31 * time.Second, ConstraintStatementTimeout: time.Second},
		{Action: "constraints", Manifest: "manifest.json", ConstraintLockTimeout: time.Second, ConstraintStatementTimeout: 31 * time.Minute},
	} {
		called := false
		err := runWithDependencies(context.Background(), options, &bytes.Buffer{}, runtimeDependencies{
			LoadConfig: func(string) (*config.Config, error) {
				called = true
				return nil, nil
			},
		})
		if err == nil || called {
			t.Fatalf("runWithDependencies() = %v, called=%v; want timeout rejection before side effects", err, called)
		}
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
	if opts.Action != "verify" || opts.BatchSize != 100 || opts.Manifest != "decision.json" || opts.ConstraintLockTimeout <= 0 || opts.ConstraintStatementTimeout <= 0 {
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
	verifyCalls, backfillCalls, constraintCalls int
	batchSize                                   int
	constraintOptions                           storecenter.StoreServiceConstraintOptions
	verifyReport                                storecenter.StoreHistoryMigrationReport
	backfillReport                              storecenter.StoreHistoryMigrationReport
	constraintReport                            storecenter.StoreServiceConstraintReport
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

func (fake *fakeHistoryMigrator) ApplyConstraints(_ context.Context, options storecenter.StoreServiceConstraintOptions) (storecenter.StoreServiceConstraintReport, error) {
	fake.constraintCalls++
	fake.constraintOptions = options
	return fake.constraintReport, nil
}
