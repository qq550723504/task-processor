package productlistingschemamigrate

import (
	"context"
	"os"
	"path/filepath"
	"testing"

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
