package schema

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

type sqliteColumnInfo struct {
	Name    string `gorm:"column:name"`
	Type    string `gorm:"column:type"`
	NotNull int    `gorm:"column:notnull"`
}

func TestWorkbenchAutoMigrateCreatesOwnedTablesAndIsRepeatable(t *testing.T) {
	db := openWorkbenchSchemaTestDB(t)
	if err := db.Exec(`CREATE TABLE listing_store (id integer primary key, marker text not null)`).Error; err != nil {
		t.Fatalf("create legacy sentinel: %v", err)
	}
	if err := db.Exec(`INSERT INTO listing_store (id, marker) VALUES (7, 'legacy-row')`).Error; err != nil {
		t.Fatalf("seed legacy sentinel: %v", err)
	}
	legacyColumnsBefore := sqliteTableColumns(t, db, "listing_store")

	for attempt := 0; attempt < 2; attempt++ {
		if err := AutoMigrateRuntime(db); err != nil {
			t.Fatalf("AutoMigrateRuntime() attempt %d error = %v", attempt+1, err)
		}
	}

	wantTables := []string{
		"listing_store",
		"saas_organization_resource_audit_logs",
		"saas_organization_resource_buckets",
		"saas_organization_resource_debts",
		"saas_organization_resource_events",
		"saas_organization_resource_operations",
		"saas_organization_resource_reservations",
		"saas_organization_resource_source_claims",
		"saas_plan_modules",
		"saas_store_quota_allocations",
		"saas_store_quota_buckets",
		"saas_tenant_entitlements",
		"saas_tenant_subscriptions",
		"workbench_store_audit_logs",
		"workbench_stores",
	}
	if got := sqliteUserTables(t, db); !reflect.DeepEqual(got, wantTables) {
		t.Fatalf("migrated tables = %v, want %v", got, wantTables)
	}

	var legacyCount int64
	if err := db.Table("listing_store").Where("id = ? AND marker = ?", 7, "legacy-row").Count(&legacyCount).Error; err != nil {
		t.Fatalf("query legacy sentinel: %v", err)
	}
	if legacyCount != 1 {
		t.Fatalf("legacy sentinel row count = %d, want 1", legacyCount)
	}
	if got := sqliteTableColumns(t, db, "listing_store"); !reflect.DeepEqual(got, legacyColumnsBefore) {
		t.Fatalf("legacy sentinel columns changed: before=%v after=%v", legacyColumnsBefore, got)
	}
}

func TestWorkbenchAutoMigrateUsesNonNullStringOrganizationColumns(t *testing.T) {
	db := openWorkbenchSchemaTestDB(t)
	if err := AutoMigrateRuntime(db); err != nil {
		t.Fatalf("AutoMigrateRuntime() error = %v", err)
	}

	for _, table := range []string{"workbench_stores", "workbench_store_audit_logs", "saas_store_quota_allocations", "saas_store_quota_buckets", "saas_organization_resource_buckets", "saas_organization_resource_debts", "saas_organization_resource_reservations"} {
		columns := sqliteTableColumns(t, db, table)
		var organizationColumn *sqliteColumnInfo
		for index := range columns {
			if columns[index].Name == "organization_id" {
				organizationColumn = &columns[index]
				break
			}
		}
		if organizationColumn == nil {
			t.Fatalf("%s has no organization_id column", table)
		}
		columnType := strings.ToLower(organizationColumn.Type)
		if !strings.Contains(columnType, "char") && !strings.Contains(columnType, "text") && !strings.Contains(columnType, "string") {
			t.Fatalf("%s organization_id type = %q, want string storage", table, organizationColumn.Type)
		}
		if organizationColumn.NotNull != 1 {
			t.Fatalf("%s organization_id notnull = %d, want 1", table, organizationColumn.NotNull)
		}
	}
}

func TestWorkbenchAutoMigrateRejectsNullStoreQuotaBucketOrganization(t *testing.T) {
	db := openWorkbenchSchemaTestDB(t)
	if err := AutoMigrateRuntime(db); err != nil {
		t.Fatalf("AutoMigrateRuntime() error = %v", err)
	}

	if err := db.Exec(`INSERT INTO saas_store_quota_buckets (organization_id) VALUES (NULL)`).Error; err == nil {
		t.Fatal("Store quota bucket accepted a NULL organization_id")
	}
}

func TestWorkbenchAutoMigrateRejectsNilDatabase(t *testing.T) {
	if err := AutoMigrateRuntime(nil); err == nil {
		t.Fatal("AutoMigrateRuntime(nil) accepted a nil database")
	}
}

func openWorkbenchSchemaTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open SQLite: %v", err)
	}
	return db
}

func sqliteUserTables(t *testing.T, db *gorm.DB) []string {
	t.Helper()
	var tables []string
	if err := db.Raw(`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`).Scan(&tables).Error; err != nil {
		t.Fatalf("list SQLite tables: %v", err)
	}
	sort.Strings(tables)
	return tables
}

func sqliteTableColumns(t *testing.T, db *gorm.DB, table string) []sqliteColumnInfo {
	t.Helper()
	var columns []sqliteColumnInfo
	if err := db.Raw("PRAGMA table_info(" + table + ")").Scan(&columns).Error; err != nil {
		t.Fatalf("inspect columns for %s: %v", table, err)
	}
	return columns
}
