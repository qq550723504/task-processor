package sourceaccountregistry

import (
	"context"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSchemaHistoryIsDedicatedAndImmutable(t *testing.T) {
	if VersionTableName != "goose_source_account_registry_version" {
		t.Fatalf("VersionTableName = %q", VersionTableName)
	}
	migrations := Migrations()
	if len(migrations) != 1 || migrations[0].Version != 2026090901 || migrations[0].DownFnContext != nil {
		t.Fatalf("Migrations() = %#v", migrations)
	}
}

func TestMigrateRejectsNonPostgresWithoutCreatingTables(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "source-account.sqlite")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := Migrate(context.Background(), db); err == nil {
		t.Fatal("Migrate(sqlite) error = nil")
	}
	for _, table := range []string{"source_account_resources", "source_account_operations", VersionTableName} {
		if db.Migrator().HasTable(table) {
			t.Fatalf("Migrate(sqlite) created %s", table)
		}
	}
}
