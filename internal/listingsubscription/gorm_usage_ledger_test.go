package listingsubscription

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestAutoMigrateRepositoryCreatesUsageLedgerSchema(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	if err := AutoMigrateRepository(db); err != nil {
		t.Fatalf("AutoMigrateRepository() error = %v", err)
	}

	for _, table := range []string{
		"saas_usage_events",
		"saas_usage_buckets",
		"saas_usage_event_outbox",
	} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("AutoMigrateRepository() did not create %s", table)
		}
	}
	for _, index := range []string{
		"idx_saas_usage_event_tenant_idempotency_key",
		"idx_saas_usage_event_tenant_metric_status",
		"idx_saas_usage_event_outbox_event_id",
		"idx_saas_usage_event_outbox_status_next_attempt",
	} {
		if !db.Migrator().HasIndex(indexTableForUsageLedgerIndex(index), index) {
			t.Fatalf("AutoMigrateRepository() did not create %s", index)
		}
	}
}

func indexTableForUsageLedgerIndex(index string) string {
	if index == "idx_saas_usage_event_outbox_event_id" || index == "idx_saas_usage_event_outbox_status_next_attempt" {
		return "saas_usage_event_outbox"
	}
	return "saas_usage_events"
}
