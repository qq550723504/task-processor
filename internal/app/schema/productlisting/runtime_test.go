package productlisting

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"task-processor/internal/platform/database/migration"
)

func TestResolveDialectSupportsOnlySQLiteAndPostgres(t *testing.T) {
	for _, test := range []struct {
		name string
		want goose.Dialect
	}{
		{name: "sqlite", want: goose.DialectSQLite3},
		{name: "postgres", want: goose.DialectPostgres},
	} {
		t.Run(test.name, func(t *testing.T) {
			dialect, err := resolveDialect(&gorm.DB{Config: &gorm.Config{Dialector: namedDialector{name: test.name}}})
			if err != nil {
				t.Fatalf("resolveDialect() error = %v", err)
			}
			if dialect != test.want {
				t.Fatalf("resolveDialect() = %q, want %q", dialect, test.want)
			}
		})
	}

	if _, err := resolveDialect(&gorm.DB{Config: &gorm.Config{Dialector: namedDialector{name: "mysql"}}}); err == nil {
		t.Fatal("resolveDialect(mysql) error = nil, want unsupported dialect error")
	}
}

func TestMigrationsRejectDifferentSQLConnection(t *testing.T) {
	db := openProductListingSchemaTestDB(t)
	other := openProductListingSchemaTestDB(t)
	otherSQLDB, err := other.DB()
	if err != nil {
		t.Fatalf("get other sql.DB: %v", err)
	}

	migrations := Migrations(db)
	if len(migrations) != 1 {
		t.Fatalf("Migrations() = %d entries, want 1", len(migrations))
	}
	runner, err := migration.New(goose.DialectSQLite3, otherSQLDB, migrations...)
	if err != nil {
		t.Fatalf("migration.New() error = %v", err)
	}
	if _, err := runner.Up(context.Background()); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("Up() with different sql.DB error = %v, want connection mismatch", err)
	}
}

func TestMigrateRecordsBaselineAndDoesNotRunTwice(t *testing.T) {
	db := openProductListingSchemaTestDB(t)

	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("first Migrate() error = %v", err)
	}
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	var count int
	if err := sqlDB.QueryRow(`SELECT count(*) FROM goose_db_version WHERE version_id = 2026083001 AND is_applied = 1`).Scan(&count); err != nil {
		t.Fatalf("query applied baseline: %v", err)
	}
	if count != 1 {
		t.Fatalf("applied baseline rows = %d, want 1", count)
	}
}

func TestAutoMigrateRuntimeCreatesExecutionEnvelopeColumns(t *testing.T) {
	db := openProductListingSchemaTestDB(t)
	if err := AutoMigrateRuntime(db); err != nil {
		t.Fatalf("AutoMigrateRuntime: %v", err)
	}

	for _, table := range []string{"amazon_listing_tasks"} {
		columns, err := db.Migrator().ColumnTypes(table)
		if err != nil {
			t.Fatalf("ColumnTypes(%s): %v", table, err)
		}
		seen := map[string]bool{}
		for _, column := range columns {
			seen[column.Name()] = true
		}
		for _, column := range []string{"execution_identity_version", "execution_tenant_id", "execution_user_id", "execution_trace_id", "execution_source_platform", "execution_source_task_type"} {
			if !seen[column] {
				t.Fatalf("table %s missing column %s", table, column)
			}
		}
	}
	for _, table := range []string{"image_agent_v2_runs", "image_agent_v2_plans", "image_agent_v2_slots", "image_agent_v2_attempts", "image_agent_v2_events", "image_agent_v2_asset_catalog", "image_agent_v2_asset_catalog_manifests", "image_agent_v2_projection_snapshots", "image_agent_v2_projection_commits", "image_agent_v2_slot_external_effects", "image_agent_v3_slot_external_effects"} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("missing table %s", table)
		}
	}
	for _, table := range []string{"product_approved_assets", "product_approval_receipts", "product_approved_inventory_heads"} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("missing table %s", table)
		}
	}
	for _, table := range []string{"product_snapshot_versions", "product_snapshot_heads"} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("missing table %s", table)
		}
	}
	for _, table := range []string{"image_agent_runs", "image_agent_plans", "image_agent_slots", "image_agent_attempts", "image_agent_events", "image_agent_asset_catalog", "image_agent_asset_catalog_manifests", "image_agent_projection_snapshots", "image_agent_projection_commits", "image_agent_slot_external_effects"} {
		if db.Migrator().HasTable(table) {
			t.Fatalf("clean bootstrap must not manufacture legacy table %s", table)
		}
	}

	columns, err := db.Migrator().ColumnTypes("ai_invocations")
	if err != nil {
		t.Fatalf("ColumnTypes(ai_invocations): %v", err)
	}
	for _, column := range columns {
		if column.Name() == "cache_status" {
			return
		}
	}
	t.Fatal("table ai_invocations missing column cache_status")
}

func TestAutoMigrateRuntimeLeavesLegacyProductTaskTablesRetired(t *testing.T) {
	db := openProductListingSchemaTestDB(t)
	if err := AutoMigrateRuntime(db); err != nil {
		t.Fatalf("AutoMigrateRuntime: %v", err)
	}

	for _, table := range []string{"product_enrich_tasks", "product_image_tasks"} {
		if db.Migrator().HasTable(table) {
			t.Errorf("AutoMigrateRuntime created retired table %s", table)
		}
	}
}

func openProductListingSchemaTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "productlisting.sqlite")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

type namedDialector struct {
	gorm.Dialector
	name string
}

func (d namedDialector) Name() string { return d.name }
