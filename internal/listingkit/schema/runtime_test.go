package schema

import (
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"

	"task-processor/internal/listingkit"
)

func TestAutoMigrateRuntimeRejectsNilDB(t *testing.T) {
	t.Parallel()

	err := AutoMigrateRuntime(nil)
	if err == nil || !strings.Contains(err.Error(), "database is nil") {
		t.Fatalf("AutoMigrateRuntime(nil) error = %v, want database is nil", err)
	}
}

func TestAutoMigrateRuntimeCreatesRepresentativeTables(t *testing.T) {
	db := openSchemaTestDB(t)

	if err := AutoMigrateRuntime(db); err != nil {
		t.Fatalf("AutoMigrateRuntime() error = %v", err)
	}
	for _, table := range []any{
		"ai_invocations",
		"ai_async_jobs",
		"listing_kit_sds_baseline_cache",
		"listingkit_owner_scope_system_owned_exceptions",
		&memberinviteAuditTable{},
		&listingkit.SDSChildRetryJob{},
		&listingkit.SheinPODImageLookupIndex{},
	} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("expected table for %T(%v) to be created", table, table)
		}
	}
	if !db.Migrator().HasColumn(&listingkit.SheinPODImageLookupIndex{}, "sds_gallery_image_urls") {
		t.Fatal("expected POD image lookup index table to store SDS gallery image URLs")
	}
}

type memberinviteAuditTable struct{}

func (memberinviteAuditTable) TableName() string {
	return "listingkit_member_invitation_audits"
}

func openSchemaTestDB(t *testing.T) *gorm.DB {
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
