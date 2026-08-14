package listingsubscription

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestAutoMigrateRepositoryCreatesUsageLedgerSchema(t *testing.T) {
	db := openUsageLedgerTestDB(t)

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

func TestAutoMigrateRepositoryEnforcesUsageLedgerConstraints(t *testing.T) {
	t.Run("usage event status", func(t *testing.T) {
		db := openUsageLedgerTestDB(t)
		if err := insertUsageEvent(db, "event-invalid-status", "tenant-17", "request-invalid-status", "invalid"); err == nil {
			t.Fatal("insert usage event with invalid status succeeded, want CHECK constraint failure")
		}
	})

	t.Run("tenant idempotency identity", func(t *testing.T) {
		db := openUsageLedgerTestDB(t)
		if err := insertUsageEvent(db, "event-first", "tenant-17", "request-42", "reserved"); err != nil {
			t.Fatalf("insert first usage event: %v", err)
		}
		if err := insertUsageEvent(db, "event-duplicate", "tenant-17", "request-42", "reserved"); err == nil {
			t.Fatal("insert duplicate tenant/idempotency usage event succeeded, want unique constraint failure")
		}
	})

	t.Run("outbox event identity", func(t *testing.T) {
		db := openUsageLedgerTestDB(t)
		if err := db.Exec("INSERT INTO saas_usage_event_outbox (event_id) VALUES (?)", "event-42").Error; err != nil {
			t.Fatalf("insert first outbox row: %v", err)
		}
		if err := db.Exec("INSERT INTO saas_usage_event_outbox (event_id) VALUES (?)", "event-42").Error; err == nil {
			t.Fatal("insert duplicate outbox event succeeded, want unique constraint failure")
		}
	})

	t.Run("outbox and bucket defaults", func(t *testing.T) {
		db := openUsageLedgerTestDB(t)
		if err := db.Exec("INSERT INTO saas_usage_event_outbox (event_id) VALUES (?)", "event-defaults").Error; err != nil {
			t.Fatalf("insert outbox row: %v", err)
		}
		var outbox struct {
			Destination string
			Status      string
			Attempts    int
		}
		if err := db.Raw("SELECT destination, status, attempts FROM saas_usage_event_outbox WHERE event_id = ?", "event-defaults").Scan(&outbox).Error; err != nil {
			t.Fatalf("load outbox defaults: %v", err)
		}
		if outbox.Destination != "openmeter" || outbox.Status != "pending" || outbox.Attempts != 0 {
			t.Fatalf("outbox defaults = %+v, want openmeter/pending/0", outbox)
		}

		if err := db.Exec("INSERT INTO saas_usage_buckets (tenant_id, module_code, period_key, metric) VALUES (?, ?, ?, ?)", "tenant-17", "studio", "2026-08", "studio_design_jobs_succeeded").Error; err != nil {
			t.Fatalf("insert usage bucket: %v", err)
		}
		var bucket struct {
			Committed int64
			Reserved  int64
		}
		if err := db.Raw("SELECT committed, reserved FROM saas_usage_buckets WHERE tenant_id = ? AND module_code = ? AND period_key = ? AND metric = ?", "tenant-17", "studio", "2026-08", "studio_design_jobs_succeeded").Scan(&bucket).Error; err != nil {
			t.Fatalf("load bucket defaults: %v", err)
		}
		if bucket.Committed != 0 || bucket.Reserved != 0 {
			t.Fatalf("bucket defaults = %+v, want 0/0", bucket)
		}
	})
}

func indexTableForUsageLedgerIndex(index string) string {
	if index == "idx_saas_usage_event_outbox_event_id" || index == "idx_saas_usage_event_outbox_status_next_attempt" {
		return "saas_usage_event_outbox"
	}
	return "saas_usage_events"
}

func openUsageLedgerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := AutoMigrateRepository(db); err != nil {
		t.Fatalf("AutoMigrateRepository() error = %v", err)
	}
	return db
}

func insertUsageEvent(db *gorm.DB, eventID, tenantID, idempotencyKey, status string) error {
	return db.Exec(`INSERT INTO saas_usage_events (
		event_id, tenant_id, module_code, metric, quantity, period_key,
		source_type, source_id, idempotency_key, status, occurred_at
	) VALUES (?, ?, 'studio', 'studio_design_jobs_succeeded', 1, '2026-08', 'design_job', 'job-42', ?, ?, CURRENT_TIMESTAMP)`, eventID, tenantID, idempotencyKey, status).Error
}
