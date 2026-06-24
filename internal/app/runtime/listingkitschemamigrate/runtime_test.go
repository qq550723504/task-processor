package listingkitschemamigrate

import (
	"context"
	"flag"
	"testing"

	"task-processor/internal/core/config"

	"gorm.io/gorm"
)

func TestResolveConfigPathAndParseFlags(t *testing.T) {
	if got := ResolveConfigPath("", ""); got != "config/config-dev.yaml" {
		t.Fatalf("default config path = %q", got)
	}
	if got := ResolveConfigPath("config/custom.yaml", "legacy.yaml"); got != "config/custom.yaml" {
		t.Fatalf("config path precedence = %q", got)
	}
	if got := ResolveConfigPath("", "legacy.yaml"); got != "legacy.yaml" {
		t.Fatalf("legacy config path = %q", got)
	}

	fs := flag.NewFlagSet("listingkit-schema-migrate", flag.ContinueOnError)
	opts := ParseFlagsFrom(fs,
		"--config", "config/runtime.yaml",
		"--app-config", "legacy.yaml",
		"--log-level", "debug",
		"--scope", "shein-sync",
	)
	if opts.Config != "config/runtime.yaml" || opts.AppConfig != "legacy.yaml" || opts.LogLevel != "debug" || opts.Scope != "shein-sync" {
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
