package listingsubscription

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestGormUsageLedgerReserveIsIdempotent(t *testing.T) {
	ctx := context.Background()
	db := openUsageLedgerTestDB(t)
	repo := NewGormRepository(db)
	seedUsageLedgerEntitlement(t, repo, "tenant-17", "studio", map[string]int{"studio_design_jobs_succeeded": 2})
	ledger := NewGormUsageLedger(repo)
	input := usageLedgerReserveInput("tenant-17", "request-42", 1)

	first, err := ledger.Reserve(ctx, input)
	if err != nil {
		t.Fatalf("first Reserve() error = %v", err)
	}
	if first.Existing || first.Event.Status != UsageEventReserved || first.ReservedUsage != 1 || first.CommittedUsage != 0 {
		t.Fatalf("first Reserve() = %+v, want new reserved event and 0/1 bucket", first)
	}
	second, err := ledger.Reserve(ctx, input)
	if err != nil {
		t.Fatalf("second Reserve() error = %v", err)
	}
	if !second.Existing || second.Event.EventID != first.Event.EventID || second.ReservedUsage != 1 || second.CommittedUsage != 0 {
		t.Fatalf("second Reserve() = %+v, want existing first event and unchanged 0/1 bucket", second)
	}

	assertUsageLedgerCounts(t, db, first.Event.EventID, 1, 0, 1, 1)
}

func TestGormUsageLedgerCommitAndReleaseAreIdempotent(t *testing.T) {
	ctx := context.Background()
	t.Run("commit", func(t *testing.T) {
		db := openUsageLedgerTestDB(t)
		repo := NewGormRepository(db)
		seedUsageLedgerEntitlement(t, repo, "tenant-17", "studio", map[string]int{"studio_design_jobs_succeeded": 2})
		ledger := NewGormUsageLedger(repo)
		reservation, err := ledger.Reserve(ctx, usageLedgerReserveInput("tenant-17", "request-commit", 1))
		if err != nil {
			t.Fatalf("Reserve() error = %v", err)
		}
		first, err := ledger.Commit(ctx, reservation.Event.EventID)
		if err != nil {
			t.Fatalf("first Commit() error = %v", err)
		}
		second, err := ledger.Commit(ctx, reservation.Event.EventID)
		if err != nil {
			t.Fatalf("second Commit() error = %v", err)
		}
		if first.Status != UsageEventCommitted || second.Status != first.Status || second.EventID != first.EventID {
			t.Fatalf("Commit() results = %+v, %+v; want matching committed events", first, second)
		}
		assertUsageLedgerCounts(t, db, reservation.Event.EventID, 1, 1, 0, 1)
	})

	t.Run("release", func(t *testing.T) {
		db := openUsageLedgerTestDB(t)
		repo := NewGormRepository(db)
		seedUsageLedgerEntitlement(t, repo, "tenant-17", "studio", map[string]int{"studio_design_jobs_succeeded": 2})
		ledger := NewGormUsageLedger(repo)
		reservation, err := ledger.Reserve(ctx, usageLedgerReserveInput("tenant-17", "request-release", 1))
		if err != nil {
			t.Fatalf("Reserve() error = %v", err)
		}
		first, err := ledger.Release(ctx, reservation.Event.EventID, "customer@example.com asked")
		if err != nil {
			t.Fatalf("first Release() error = %v", err)
		}
		second, err := ledger.Release(ctx, reservation.Event.EventID, "retry")
		if err != nil {
			t.Fatalf("second Release() error = %v", err)
		}
		if first.Status != UsageEventReleased || second.Status != first.Status || second.EventID != first.EventID {
			t.Fatalf("Release() results = %+v, %+v; want matching released events", first, second)
		}
		assertUsageLedgerCounts(t, db, reservation.Event.EventID, 1, 0, 0, 1)
		var audit auditLogRow
		if err := db.Where("action = ?", "usage_released").Take(&audit).Error; err != nil {
			t.Fatalf("load release audit payload: %v", err)
		}
		if audit.Payload == "" || containsUsageReason(audit.Payload, "customer@example.com") {
			t.Fatalf("release audit payload = %q, want a non-empty redacted payload", audit.Payload)
		}
	})
}

func TestUsageLedgerQuotaReservationAndStorageDeltas(t *testing.T) {
	ctx := context.Background()
	t.Run("positive quantity", func(t *testing.T) {
		db := openUsageLedgerTestDB(t)
		repo := NewGormRepository(db)
		seedUsageLedgerEntitlement(t, repo, "tenant-17", "studio", map[string]int{"studio_design_jobs_succeeded": 2})
		ledger := NewGormUsageLedger(repo)
		if _, err := ledger.Reserve(ctx, usageLedgerReserveInput("tenant-17", "request-one", 2)); err != nil {
			t.Fatalf("Reserve() up to limit error = %v", err)
		}
		_, err := ledger.Reserve(ctx, usageLedgerReserveInput("tenant-17", "request-two", 1))
		var quotaErr *UsageQuotaError
		if !errors.As(err, &quotaErr) {
			t.Fatalf("Reserve() over limit error = %v, want *UsageQuotaError", err)
		}
		if quotaErr.Metric != "studio_design_jobs_succeeded" || quotaErr.Limit == nil || *quotaErr.Limit != 2 || quotaErr.CommittedUsage != 0 || quotaErr.ReservedUsage != 2 || quotaErr.Quantity != 1 {
			t.Fatalf("quota error = %+v, want metric/limit/committed/reserved/quantity", quotaErr)
		}
	})

	t.Run("storage delta", func(t *testing.T) {
		db := openUsageLedgerTestDB(t)
		repo := NewGormRepository(db)
		seedUsageLedgerEntitlement(t, repo, "tenant-17", "oss_storage", map[string]int{"storage_bytes_current": 100})
		ledger := NewGormUsageLedger(repo)
		positive := usageLedgerStorageInput("tenant-17", "storage-add", 10)
		reservation, err := ledger.Reserve(ctx, positive)
		if err != nil {
			t.Fatalf("Reserve() positive storage delta error = %v", err)
		}
		if _, err := ledger.Commit(ctx, reservation.Event.EventID); err != nil {
			t.Fatalf("Commit() positive storage delta error = %v", err)
		}
		if _, err := ledger.Reserve(ctx, usageLedgerStorageInput("tenant-17", "storage-remove", -10)); err != nil {
			t.Fatalf("Reserve() non-negative storage delta error = %v", err)
		}
		_, err = ledger.Reserve(ctx, usageLedgerStorageInput("tenant-17", "storage-below-zero", -1))
		if !errors.Is(err, ErrUsageQuotaExceeded) {
			t.Fatalf("Reserve() negative storage below zero error = %v, want ErrUsageQuotaExceeded", err)
		}
		var quotaErr *UsageQuotaError
		if !errors.As(err, &quotaErr) || quotaErr.Metric != usageMetricStorageBytesCurrent || quotaErr.Limit == nil || *quotaErr.Limit != 100 || quotaErr.CommittedUsage != 10 || quotaErr.ReservedUsage != -10 || quotaErr.Quantity != -1 {
			t.Fatalf("storage quota error = %+v, want metric/limit/committed/reserved/quantity", quotaErr)
		}
	})
}

func TestGormUsageLedgerListsPendingOutbox(t *testing.T) {
	ctx := context.Background()
	db := openUsageLedgerTestDB(t)
	repo := NewGormRepository(db)
	seedUsageLedgerEntitlement(t, repo, "tenant-17", "studio", map[string]int{"studio_design_jobs_succeeded": 2})
	ledger := NewGormUsageLedger(repo)
	reservation, err := ledger.Reserve(ctx, usageLedgerReserveInput("tenant-17", "request-outbox", 1))
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	items, err := ledger.ListPendingOutbox(ctx, 1)
	if err != nil {
		t.Fatalf("ListPendingOutbox() error = %v", err)
	}
	if len(items) != 1 || items[0].EventID != reservation.Event.EventID || items[0].Destination != "openmeter" || items[0].Status != "pending" {
		t.Fatalf("ListPendingOutbox() = %+v, want pending OpenMeter item for reservation", items)
	}
}

func TestGormUsageLedgerReverseCreatesImmutableReversal(t *testing.T) {
	ctx := context.Background()
	db := openUsageLedgerTestDB(t)
	repo := NewGormRepository(db)
	seedUsageLedgerEntitlement(t, repo, "tenant-17", "studio", map[string]int{"studio_design_jobs_succeeded": 2})
	ledger := NewGormUsageLedger(repo)
	reservation, err := ledger.Reserve(ctx, usageLedgerReserveInput("tenant-17", "request-original", 1))
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	if _, err := ledger.Commit(ctx, reservation.Event.EventID); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	reversal, err := ledger.Reverse(ctx, reservation.Event.EventID, "request-reversal", "duplicate request")
	if err != nil {
		t.Fatalf("Reverse() error = %v", err)
	}
	if reversal.Status != UsageEventReversed || reversal.ReversalOf != reservation.Event.EventID || reversal.Quantity != -1 {
		t.Fatalf("Reverse() = %+v, want reversed event linked to original with negative quantity", reversal)
	}
	original, err := ledger.Get(ctx, "tenant-17", "request-original")
	if err != nil {
		t.Fatalf("Get() original error = %v", err)
	}
	if original.Status != UsageEventCommitted || original.Quantity != 1 {
		t.Fatalf("original event after reversal = %+v, want immutable committed original", original)
	}
	assertUsageLedgerCounts(t, db, reversal.EventID, 2, 0, 0, 2)
}

func usageLedgerReserveInput(tenantID, idempotencyKey string, quantity int64) ReserveUsageInput {
	return ReserveUsageInput{TenantID: tenantID, ModuleCode: "studio", Metric: "studio_design_jobs_succeeded", Quantity: quantity, PeriodKey: "2026-08", SourceType: "design_job", SourceID: "job-42", IdempotencyKey: idempotencyKey, OccurredAt: time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC)}
}

func usageLedgerStorageInput(tenantID, idempotencyKey string, quantity int64) ReserveUsageInput {
	return ReserveUsageInput{TenantID: tenantID, ModuleCode: "oss_storage", Metric: usageMetricStorageBytesCurrent, Quantity: quantity, PeriodKey: "2026-08", SourceType: "storage_snapshot", SourceID: "bucket-42", IdempotencyKey: idempotencyKey, OccurredAt: time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC)}
}

func seedUsageLedgerEntitlement(t *testing.T, repo *GormRepository, tenantID, moduleCode string, limits map[string]int) {
	t.Helper()
	if _, err := repo.UpsertEntitlement(context.Background(), &Entitlement{TenantID: tenantID, ModuleCode: moduleCode, Status: StatusActive, Limits: limits}); err != nil {
		t.Fatalf("UpsertEntitlement() error = %v", err)
	}
}

func assertUsageLedgerCounts(t *testing.T, db *gorm.DB, eventID string, events int64, committed, reserved int64, outbox int64) {
	t.Helper()
	var gotEvents, gotOutbox int64
	if err := db.Model(&usageEventRow{}).Count(&gotEvents).Error; err != nil {
		t.Fatalf("count usage events: %v", err)
	}
	if err := db.Model(&usageEventOutboxRow{}).Count(&gotOutbox).Error; err != nil {
		t.Fatalf("count usage outbox: %v", err)
	}
	var bucket usageBucketRow
	if err := db.Where("tenant_id = ? AND module_code = ? AND period_key = ? AND metric = ?", "tenant-17", "studio", "2026-08", "studio_design_jobs_succeeded").Take(&bucket).Error; err != nil {
		t.Fatalf("load usage bucket: %v", err)
	}
	if gotEvents != events || gotOutbox != outbox || bucket.Committed != committed || bucket.Reserved != reserved {
		t.Fatalf("ledger counts = events:%d outbox:%d committed:%d reserved:%d, want events:%d outbox:%d committed:%d reserved:%d", gotEvents, gotOutbox, bucket.Committed, bucket.Reserved, events, outbox, committed, reserved)
	}
}

func containsUsageReason(payload, reason string) bool {
	return strings.Contains(payload, reason)
}

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
