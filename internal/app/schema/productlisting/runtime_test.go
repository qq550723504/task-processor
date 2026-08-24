package productlisting

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAutoMigrateRuntimeCreatesExecutionEnvelopeColumns(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := AutoMigrateRuntime(db); err != nil {
		t.Fatalf("AutoMigrateRuntime: %v", err)
	}

	for _, table := range []string{"product_enrich_tasks", "product_image_tasks", "amazon_listing_tasks"} {
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
