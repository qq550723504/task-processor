package listingkitschemamigrate

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"task-processor/internal/core/config"
	"task-processor/internal/listingkit"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestDefaultLoaderAcceptsDatabaseOnlyConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "database-only.yaml")
	contents := []byte("database:\n  host: database.internal\n  port: 5432\n  user: listingkit\n  password: test-only\n  database: listingkit\n")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := defaultRuntimeDependencies().LoadConfig(path)
	if err != nil {
		t.Fatalf("load database-only config: %v", err)
	}
	if cfg.Database == nil || cfg.Database.Host != "database.internal" {
		t.Fatalf("database config = %#v", cfg.Database)
	}
}

func TestResolveConfigPathAndParseFlags(t *testing.T) {
	if got := ResolveConfigPath(""); got != "config/config-dev.yaml" {
		t.Fatalf("default config path = %q", got)
	}
	if got := ResolveConfigPath("config/custom.yaml"); got != "config/custom.yaml" {
		t.Fatalf("config path precedence = %q", got)
	}

	fs := flag.NewFlagSet("listingkit-schema-migrate", flag.ContinueOnError)
	opts := ParseFlagsFrom(fs,
		"--config", "config/runtime.yaml",
		"--log-level", "debug",
		"--scope", "shein-sync",
	)
	if opts.Config != "config/runtime.yaml" || opts.LogLevel != "debug" || opts.Scope != "shein-sync" {
		t.Fatalf("unexpected parsed options: %+v", opts)
	}
}

func TestRunDispatchesSheinSyncScopeAndClosesDatabase(t *testing.T) {
	var opened bool
	var migratedSheinSync bool
	var closed bool
	db := &gorm.DB{}

	err := runWithDependencies(context.Background(), Options{Config: "config/test.yaml", LogLevel: "error", Scope: "shein-sync"}, runtimeDependencies{
		LoadConfig: func(configPath string) (*config.Config, error) {
			if configPath != "config/test.yaml" {
				t.Fatalf("unexpected config path %q", configPath)
			}
			return &config.Config{Database: &config.DatabaseConfig{}}, nil
		},
		OpenDB: func(cfg *config.DatabaseConfig) (*gorm.DB, error) {
			opened = true
			return db, nil
		},
		CloseDB: func(got *gorm.DB) error {
			if got != db {
				t.Fatalf("closed unexpected db handle")
			}
			closed = true
			return nil
		},
		MigrateAll: func(db *gorm.DB) error {
			t.Fatal("MigrateAll should not be called for shein-sync scope")
			return nil
		},
		MigrateSheinSync: func(got *gorm.DB) error {
			if got != db {
				t.Fatalf("migrated unexpected db handle")
			}
			migratedSheinSync = true
			return nil
		},
	})
	if err != nil {
		t.Fatalf("runWithDependencies returned error: %v", err)
	}
	if !opened || !migratedSheinSync || !closed {
		t.Fatalf("expected open, shein-sync migration, and close; opened=%v migrated=%v closed=%v", opened, migratedSheinSync, closed)
	}
}

func TestMigrationScopeDispatchesExactMigratorsInOrder(t *testing.T) {
	db := &gorm.DB{}
	for _, test := range []struct {
		name      string
		scope     string
		wantOrder []string
		wantErr   error
	}{
		{name: "empty is all", scope: "", wantOrder: []string{"all", "workbench"}},
		{name: "all", scope: "all", wantOrder: []string{"all", "workbench"}},
		{name: "shein sync", scope: "shein-sync", wantOrder: []string{"shein-sync"}},
		{name: "workbench", scope: "workbench", wantOrder: []string{"workbench"}},
		{name: "unknown", scope: "unknown", wantErr: flag.ErrHelp},
	} {
		t.Run(test.name, func(t *testing.T) {
			var order []string
			err := runMigration(db, test.scope, runtimeDependencies{
				MigrateAll: func(got *gorm.DB) error {
					if got != db {
						t.Fatal("MigrateAll received a different database")
					}
					order = append(order, "all")
					return nil
				},
				MigrateSheinSync: func(got *gorm.DB) error {
					if got != db {
						t.Fatal("MigrateSheinSync received a different database")
					}
					order = append(order, "shein-sync")
					return nil
				},
				MigrateWorkbench: func(got *gorm.DB) error {
					if got != db {
						t.Fatal("MigrateWorkbench received a different database")
					}
					order = append(order, "workbench")
					return nil
				},
			})
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("runMigration() error = %v, want %v", err, test.wantErr)
			}
			if !reflect.DeepEqual(order, test.wantOrder) {
				t.Fatalf("migration order = %v, want %v", order, test.wantOrder)
			}
		})
	}
}

func TestMigrationUnknownScopeReturnsHelpBeforeAnyRuntimeSideEffect(t *testing.T) {
	calls := make([]string, 0, 6)
	err := runWithDependencies(context.Background(), Options{Scope: "unknown"}, runtimeDependencies{
		LoadConfig: func(string) (*config.Config, error) {
			calls = append(calls, "load-config")
			return &config.Config{Database: &config.DatabaseConfig{}}, nil
		},
		OpenDB: func(*config.DatabaseConfig) (*gorm.DB, error) {
			calls = append(calls, "open-db")
			return &gorm.DB{}, nil
		},
		CloseDB: func(*gorm.DB) error {
			calls = append(calls, "close-db")
			return nil
		},
		MigrateAll: func(*gorm.DB) error {
			calls = append(calls, "migrate-all")
			return nil
		},
		MigrateSheinSync: func(*gorm.DB) error {
			calls = append(calls, "migrate-shein-sync")
			return nil
		},
		MigrateWorkbench: func(*gorm.DB) error {
			calls = append(calls, "migrate-workbench")
			return nil
		},
	})
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("runWithDependencies() error = %v, want flag.ErrHelp", err)
	}
	if len(calls) != 0 {
		t.Fatalf("unknown scope caused runtime side effects: %v", calls)
	}
}

func TestMigrationScopeAllStopsBeforeWorkbenchAfterListingKitFailure(t *testing.T) {
	db := &gorm.DB{}
	wantErr := errors.New("listingkit migration failed")
	workbenchCalled := false
	err := runMigration(db, "all", runtimeDependencies{
		MigrateAll: func(*gorm.DB) error { return wantErr },
		MigrateWorkbench: func(*gorm.DB) error {
			workbenchCalled = true
			return nil
		},
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("runMigration() error = %v, want %v", err, wantErr)
	}
	if workbenchCalled {
		t.Fatal("Workbench migration ran after the broad migration failed")
	}
}

func TestMigrationScopeWorkbenchClosesDatabaseOnMigrationFailure(t *testing.T) {
	db := &gorm.DB{}
	wantErr := errors.New("workbench migration failed")
	closed := false
	err := runWithDependencies(context.Background(), Options{Config: "config/test.yaml", LogLevel: "error", Scope: "workbench"}, runtimeDependencies{
		LoadConfig: func(string) (*config.Config, error) {
			return &config.Config{Database: &config.DatabaseConfig{}}, nil
		},
		OpenDB: func(*config.DatabaseConfig) (*gorm.DB, error) { return db, nil },
		CloseDB: func(got *gorm.DB) error {
			if got != db {
				t.Fatal("closed a different database")
			}
			closed = true
			return nil
		},
		MigrateAll: func(*gorm.DB) error {
			t.Fatal("broad migration ran for workbench-only scope")
			return nil
		},
		MigrateSheinSync: func(*gorm.DB) error {
			t.Fatal("Shein sync migration ran for workbench-only scope")
			return nil
		},
		MigrateWorkbench: func(*gorm.DB) error { return wantErr },
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("runWithDependencies() error = %v, want wrapped %v", err, wantErr)
	}
	if !closed {
		t.Fatal("database was not closed after Workbench migration failure")
	}
}

func TestMigrationScopeWorkbenchDefaultsToOwnedMigrator(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	err = runWithDependencies(context.Background(), Options{Config: "config/test.yaml", LogLevel: "error", Scope: "workbench"}, runtimeDependencies{
		LoadConfig: func(string) (*config.Config, error) {
			return &config.Config{Database: &config.DatabaseConfig{}}, nil
		},
		OpenDB:  func(*config.DatabaseConfig) (*gorm.DB, error) { return db, nil },
		CloseDB: func(*gorm.DB) error { return nil },
	})
	if err != nil {
		t.Fatalf("runWithDependencies() error = %v", err)
	}
	if !db.Migrator().HasTable("workbench_stores") {
		t.Fatal("default Workbench migrator did not create the owned schema")
	}
}

func TestMigrationScopeFlagHelpAdvertisesAllSupportedScopes(t *testing.T) {
	var output bytes.Buffer
	fs := flag.NewFlagSet("listingkit-schema-migrate", flag.ContinueOnError)
	fs.SetOutput(&output)
	ParseFlagsFrom(fs, "--help")
	fs.PrintDefaults()
	for _, scope := range []string{"all", "shein-sync", "workbench"} {
		if !strings.Contains(output.String(), scope) {
			t.Fatalf("scope help does not advertise %q: %s", scope, output.String())
		}
	}
}

func TestAutoMigrateListingKitRuntimeSchemaCreatesSheinPODImageLookupIndexTable(t *testing.T) {
	db := openRuntimeSchemaTestDB(t)

	if err := autoMigrateListingKitRuntimeSchema(db); err != nil {
		t.Fatalf("autoMigrateListingKitRuntimeSchema() error = %v", err)
	}

	if !db.Migrator().HasTable(&listingkit.SheinPODImageLookupIndex{}) {
		t.Fatal("expected POD image lookup index table to be created")
	}
}

func TestAutoMigrateListingKitRuntimeSchemaCreatesAIInvocationsTable(t *testing.T) {
	db := openRuntimeSchemaTestDB(t)

	if err := autoMigrateListingKitRuntimeSchema(db); err != nil {
		t.Fatalf("autoMigrateListingKitRuntimeSchema() error = %v", err)
	}
	if !db.Migrator().HasTable("ai_invocations") {
		t.Fatal("expected ai_invocations table to be created")
	}
	if !db.Migrator().HasTable("ai_async_jobs") {
		t.Fatal("expected ai_async_jobs table to be created")
	}
}

func TestAutoMigrateListingKitRuntimeSchemaCreatesSDSChildRetryTable(t *testing.T) {
	db := openRuntimeSchemaTestDB(t)

	if err := autoMigrateListingKitRuntimeSchema(db); err != nil {
		t.Fatalf("autoMigrateListingKitRuntimeSchema() error = %v", err)
	}
	if !db.Migrator().HasTable(&listingkit.SDSChildRetryJob{}) {
		t.Fatal("expected SDS child retry table to be created")
	}
}

func openRuntimeSchemaTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	for _, table := range []string{
		"listing_store",
		"listing_product_import_task",
		"listing_filter_rule",
		"listing_profit_rule",
		"listing_pricing_rule",
		"listing_operation_strategy",
		"listing_sensitive_word",
		"listing_product_import_mapping",
		"listing_category",
		"listing_product_data",
	} {
		if err := db.Exec("CREATE TABLE " + table + " (id integer)").Error; err != nil {
			t.Fatalf("create legacy %s table: %v", table, err)
		}
	}
	return db
}
