package productlistingschemamigrate

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"task-processor/internal/app/runtime/currentapplication"
	"task-processor/internal/core/config"

	"gorm.io/gorm"
)

func TestRunDefaultLoaderAcceptsDatabaseOnlyConfig(t *testing.T) {
	configPath := writeDatabaseOnlyConfig(t)
	db := &gorm.DB{}
	ctx := context.WithValue(context.Background(), migrationContextKey{}, "migration-request")

	err := runWithDependencies(ctx, configPath, Dependencies{
		OpenDB: func(cfg *config.DatabaseConfig) (*gorm.DB, error) {
			if cfg == nil || cfg.Host != "database.internal" {
				t.Fatalf("database config = %#v", cfg)
			}
			return db, nil
		},
		CloseDB: func(*gorm.DB) error { return nil },
		MigrateAll: func(got context.Context, migrationDB *gorm.DB) error {
			if got.Value(migrationContextKey{}) != "migration-request" {
				t.Fatal("migration did not receive caller context")
			}
			if migrationDB != db {
				t.Fatalf("migration db = %p, want %p", migrationDB, db)
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Run() with database-only config error = %v", err)
	}
}

func TestImageAgentGrantRequiresExplicitMatchingOwnerDatabase(t *testing.T) {
	if err := GrantImageAgentRuntime(context.Background(), "config/config-dev.yaml", ""); err == nil {
		t.Fatal("grant mode accepted an implicit legacy database config")
	}
	owner := &config.Config{Database: &config.DatabaseConfig{Host: "127.0.0.1", Port: 5432, User: "owner", Database: "image_owner"}}
	current := &currentapplication.Config{ImageAgent: &currentapplication.ImageAgentConfig{Database: currentapplication.DatabaseConfig{Host: "127.0.0.1", Port: 5432, User: "image_agent_runtime", Database: "image_owner"}}}
	if err := validateImageAgentGrantTarget(owner, current); err != nil {
		t.Fatalf("matching owner rejected: %v", err)
	}
	for _, change := range []func(){
		func() { current.ImageAgent.Database.Database = "other" },
		func() { current.ImageAgent.Database.Host = "other" },
		func() { current.ImageAgent.Database.Port = 5433 },
	} {
		original := current.ImageAgent.Database
		change()
		if err := validateImageAgentGrantTarget(owner, current); err == nil {
			t.Fatal("grant accepted a mismatched owner database")
		}
		current.ImageAgent.Database = original
	}
	owner.Database.User = "image_agent_runtime"
	if err := validateImageAgentGrantTarget(owner, current); err == nil {
		t.Fatal("runtime credential used as owner installer")
	}
	owner.Database.User = "owner"
	current.ImageAgent = nil
	if err := validateImageAgentGrantTarget(owner, current); err == nil {
		t.Fatal("grant accepted manifest without ImageAgent owner")
	}
}

type migrationContextKey struct{}

func writeDatabaseOnlyConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "database-only.yaml")
	contents := []byte("database:\n  host: database.internal\n  port: 5432\n  user: listingkit\n  password: test-only\n  database: listingkit\n")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
