package listingsubscription

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

func TestIsRetryableUsageLedgerErrorUsesSQLiteDriverCode(t *testing.T) {
	for _, code := range []int{
		sqlite3.SQLITE_LOCKED,
		sqlite3.SQLITE_LOCKED | (1 << 8),
		sqlite3.SQLITE_BUSY,
		sqlite3.SQLITE_BUSY_RECOVERY,
		sqlite3.SQLITE_BUSY_SNAPSHOT,
		sqlite3.SQLITE_BUSY_TIMEOUT,
	} {
		if !isRetryableSQLiteCode(code) {
			t.Errorf("SQLite code %d was not classified as retryable", code)
		}
	}
	if isRetryableSQLiteCode(sqlite3.SQLITE_CONSTRAINT) {
		t.Fatal("SQLite constraint code was classified as retryable")
	}
	if isRetryableUsageLedgerError(errors.New("database is locked but not a driver error")) {
		t.Fatal("text-only lock error was classified as retryable")
	}
}

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

func TestGormUsageLedgerRejectsIdempotencyKeyForDifferentUsageFact(t *testing.T) {
	ctx := context.Background()
	db := openUsageLedgerTestDB(t)
	repo := NewGormRepository(db)
	seedUsageLedgerEntitlement(t, repo, "tenant-17", "studio", map[string]int{"studio_design_jobs_succeeded": 10})
	ledger := NewGormUsageLedger(repo)
	input := usageLedgerReserveInput("tenant-17", "request-fact", 1)
	if _, err := ledger.Reserve(ctx, input); err != nil {
		t.Fatalf("first Reserve() error = %v", err)
	}
	changed := input
	changed.SourceID = "different-job"
	if _, err := ledger.Reserve(ctx, changed); !errors.Is(err, ErrUsageDuplicateIdentity) {
		t.Fatalf("Reserve() changed fact error = %v, want ErrUsageDuplicateIdentity", err)
	}
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
		for _, key := range []string{"request-one", "request-one-b"} {
			if _, err := ledger.Reserve(ctx, usageLedgerReserveInput("tenant-17", key, 1)); err != nil {
				t.Fatalf("Reserve() up to limit error = %v", err)
			}
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

	t.Run("storage signed reservations and period rollover", func(t *testing.T) {
		db := openUsageLedgerTestDB(t)
		repo := NewGormRepository(db)
		seedUsageLedgerEntitlement(t, repo, "tenant-17", "oss_storage", map[string]int{"storage_bytes_current": 100})
		ledger := NewGormUsageLedger(repo)

		add, err := ledger.Reserve(ctx, usageLedgerStorageInput("tenant-17", "storage-add-base", 10))
		if err != nil {
			t.Fatalf("base storage Reserve() error = %v", err)
		}
		if _, err := ledger.Commit(ctx, add.Event.EventID); err != nil {
			t.Fatalf("base storage Commit() error = %v", err)
		}
		remove := usageLedgerStorageInput("tenant-17", "storage-remove-old", -8)
		remove.PeriodKey = "2026-09"
		if _, err := ledger.Reserve(ctx, remove); err != nil {
			t.Fatalf("old-period delete Reserve() error = %v", err)
		}
		addDuringDelete := usageLedgerStorageInput("tenant-17", "storage-add-during-delete", 2)
		addDuringDelete.PeriodKey = "2026-09"
		addEvent, err := ledger.Reserve(ctx, addDuringDelete)
		if err != nil {
			t.Fatalf("overlapping upload Reserve() error = %v", err)
		}
		committedUpload, err := ledger.Commit(ctx, addEvent.Event.EventID)
		if err != nil {
			t.Fatalf("overlapping upload Commit() error = %v", err)
		}
		if committedUpload.StorageSnapshot == nil || *committedUpload.StorageSnapshot != 12 {
			t.Fatalf("storage upload snapshot = %v, want 12 committed bytes", committedUpload.StorageSnapshot)
		}
		if _, err := ledger.Commit(ctx, mustUsageEventID(t, db, "storage-remove-old")); err != nil {
			t.Fatalf("old-period delete Commit() error = %v", err)
		}
		var buckets []usageBucketRow
		if err := db.Where("tenant_id = ? AND module_code = ? AND metric = ?", "tenant-17", ModuleOSSStorage, usageMetricStorageBytesCurrent).Find(&buckets).Error; err != nil {
			t.Fatalf("load storage buckets: %v", err)
		}
		if len(buckets) != 1 || buckets[0].PeriodKey != usageStorageBucketPeriodKey || buckets[0].Committed != 4 || buckets[0].Reserved != 0 {
			t.Fatalf("storage buckets = %+v, want one current bucket committed=4 reserved=0", buckets)
		}
		newPeriodDelete := usageLedgerStorageInput("tenant-17", "storage-remove-new-period", -4)
		newPeriodDelete.PeriodKey = "2026-10"
		newEvent, err := ledger.Reserve(ctx, newPeriodDelete)
		if err != nil {
			t.Fatalf("new-period delete Reserve() error = %v", err)
		}
		finalEvent, err := ledger.Commit(ctx, newEvent.Event.EventID)
		if err != nil {
			t.Fatalf("new-period delete Commit() error = %v", err)
		}
		if finalEvent.StorageSnapshot == nil || *finalEvent.StorageSnapshot != 0 {
			t.Fatalf("final storage snapshot = %v, want 0", finalEvent.StorageSnapshot)
		}
	})

	t.Run("zero limit is unlimited", func(t *testing.T) {
		db := openUsageLedgerTestDB(t)
		repo := NewGormRepository(db)
		seedUsageLedgerEntitlement(t, repo, "tenant-17", "studio", map[string]int{"studio_design_jobs_succeeded": 0})
		if _, err := NewGormUsageLedger(repo).Reserve(ctx, usageLedgerReserveInput("tenant-17", "request-unlimited", 1)); err != nil {
			t.Fatalf("zero-limit Reserve() error = %v, want unlimited semantics", err)
		}
	})
}

func TestGormUsageLedgerUsesStudioFallbackForStorage(t *testing.T) {
	db := openUsageLedgerTestDB(t)
	repo := NewGormRepository(db)
	seedUsageLedgerEntitlement(t, repo, "tenant-17", ModuleStudio, nil)
	if _, err := NewGormUsageLedger(repo).Reserve(context.Background(), usageLedgerStorageInput("tenant-17", "storage-studio-fallback", 1)); err != nil {
		t.Fatalf("storage fallback Reserve() error = %v, want studio entitlement fallback", err)
	}
}

func TestGormUsageLedgerConcurrentQuotaReservations(t *testing.T) {
	ctx := context.Background()
	db := openConcurrentUsageLedgerTestDB(t)
	repo := NewGormRepository(db)
	seedUsageLedgerEntitlement(t, repo, "tenant-concurrent", "studio", map[string]int{"studio_design_jobs_succeeded": 10})
	if err := db.Create(&usageBucketRow{TenantID: "tenant-concurrent", ModuleCode: "studio", PeriodKey: "2026-08", Metric: "studio_design_jobs_succeeded"}).Error; err != nil {
		t.Fatalf("seed concurrent usage bucket: %v", err)
	}
	ledger := NewGormUsageLedger(repo)
	var activeCreates atomic.Int32
	var overlapped atomic.Bool
	if err := db.Callback().Create().Before("gorm:create").Register("test_usage_event_overlap", func(tx *gorm.DB) {
		if tx.Statement.Table != "saas_usage_events" {
			return
		}
		if active := activeCreates.Add(1); active > 1 {
			overlapped.Store(true)
		}
		time.Sleep(5 * time.Millisecond)
		activeCreates.Add(-1)
	}); err != nil {
		t.Fatalf("register overlap callback: %v", err)
	}

	start := make(chan struct{})
	results := make(chan error, 20)
	var workers sync.WaitGroup
	for i := 0; i < 20; i++ {
		workers.Add(1)
		go func(worker int) {
			defer workers.Done()
			<-start
			input := usageLedgerReserveInput("tenant-concurrent", fmt.Sprintf("concurrent-%02d", worker), 1)
			_, err := ledger.Reserve(ctx, input)
			results <- err
		}(i)
	}
	close(start)
	workers.Wait()
	close(results)

	successes := 0
	quotaFailures := 0
	for err := range results {
		if err == nil {
			successes++
			continue
		}
		var quotaErr *UsageQuotaError
		if errors.As(err, &quotaErr) {
			quotaFailures++
			continue
		}
		t.Fatalf("concurrent Reserve() error = %v, want only quota rejection after capacity is exhausted", err)
	}
	if successes != 10 || quotaFailures != 10 {
		t.Fatalf("concurrent reservations = successes:%d quota_failures:%d, want 10/10", successes, quotaFailures)
	}
	if !overlapped.Load() {
		t.Fatal("concurrent reservations did not overlap event transactions")
	}

	var bucket usageBucketRow
	if err := db.Where("tenant_id = ? AND module_code = ? AND period_key = ? AND metric = ?", "tenant-concurrent", "studio", "2026-08", "studio_design_jobs_succeeded").Take(&bucket).Error; err != nil {
		t.Fatalf("load concurrent usage bucket: %v", err)
	}
	var events, outbox int64
	if err := db.Model(&usageEventRow{}).Where("tenant_id = ?", "tenant-concurrent").Count(&events).Error; err != nil {
		t.Fatalf("count concurrent usage events: %v", err)
	}
	if err := db.Model(&usageEventOutboxRow{}).Joins("JOIN saas_usage_events ON saas_usage_events.event_id = saas_usage_event_outbox.event_id").Where("saas_usage_events.tenant_id = ?", "tenant-concurrent").Count(&outbox).Error; err != nil {
		t.Fatalf("count concurrent outbox items: %v", err)
	}
	if bucket.Committed != 0 || bucket.Reserved != 10 || events != 10 || outbox != 10 {
		t.Fatalf("concurrent durable totals = committed:%d reserved:%d events:%d outbox:%d, want 0/10/10/10", bucket.Committed, bucket.Reserved, events, outbox)
	}
	var distinctEvents, distinctOutbox int64
	if err := db.Model(&usageEventRow{}).Where("tenant_id = ?", "tenant-concurrent").Distinct("event_id").Count(&distinctEvents).Error; err != nil {
		t.Fatalf("count distinct concurrent events: %v", err)
	}
	if err := db.Model(&usageEventOutboxRow{}).Joins("JOIN saas_usage_events ON saas_usage_events.event_id = saas_usage_event_outbox.event_id").Where("saas_usage_events.tenant_id = ?", "tenant-concurrent").Distinct("saas_usage_event_outbox.event_id").Count(&distinctOutbox).Error; err != nil {
		t.Fatalf("count distinct concurrent outbox items: %v", err)
	}
	if distinctEvents != 10 || distinctOutbox != 10 {
		t.Fatalf("distinct concurrent IDs = events:%d outbox:%d, want 10/10", distinctEvents, distinctOutbox)
	}
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
	if _, err := ledger.Commit(ctx, reservation.Event.EventID); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	items, err := ledger.ListPendingOutbox(ctx, 1)
	if err != nil {
		t.Fatalf("ListPendingOutbox() error = %v", err)
	}
	if len(items) != 1 || items[0].EventID != reservation.Event.EventID || items[0].Destination != "openmeter" || items[0].Status != "pending" {
		t.Fatalf("ListPendingOutbox() = %+v, want pending OpenMeter item after commit", items)
	}
}

func TestGormUsageLedgerHonorsOutboxRetrySchedule(t *testing.T) {
	ctx := context.Background()
	db := openUsageLedgerTestDB(t)
	repo := NewGormRepository(db)
	seedUsageLedgerEntitlement(t, repo, "tenant-17", "studio", map[string]int{"design_jobs": 2})
	ledger := NewGormUsageLedger(repo)
	reservation, err := ledger.Reserve(ctx, usageLedgerReserveInput("tenant-17", "request-retry-schedule", 1))
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	if _, err := ledger.Commit(ctx, reservation.Event.EventID); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	future := time.Now().UTC().Add(time.Hour)
	if err := db.Model(&usageEventOutboxRow{}).Where("event_id = ?", reservation.Event.EventID).Update("next_attempt_at", future).Error; err != nil {
		t.Fatalf("set future retry: %v", err)
	}
	items, err := ledger.ListPendingOutbox(ctx, 10)
	if err != nil {
		t.Fatalf("ListPendingOutbox() future error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("future retry items = %+v, want none", items)
	}
	past := time.Now().UTC().Add(-time.Minute)
	if err := db.Model(&usageEventOutboxRow{}).Where("event_id = ?", reservation.Event.EventID).Update("next_attempt_at", past).Error; err != nil {
		t.Fatalf("set elapsed retry: %v", err)
	}
	items, err = ledger.ListPendingOutbox(ctx, 10)
	if err != nil || len(items) != 1 {
		t.Fatalf("elapsed retry items = %+v, error=%v, want one", items, err)
	}
}

func TestGormUsageLedgerPendingOutboxExcludesReleasedAndReversalRows(t *testing.T) {
	ctx := context.Background()
	t.Run("released", func(t *testing.T) {
		db := openUsageLedgerTestDB(t)
		repo := NewGormRepository(db)
		seedUsageLedgerEntitlement(t, repo, "tenant-17", "studio", map[string]int{"studio_design_jobs_succeeded": 2})
		ledger := NewGormUsageLedger(repo)
		reservation, err := ledger.Reserve(ctx, usageLedgerReserveInput("tenant-17", "request-release-queue", 1))
		if err != nil {
			t.Fatalf("Reserve() error = %v", err)
		}
		if _, err := ledger.Release(ctx, reservation.Event.EventID, "cancelled"); err != nil {
			t.Fatalf("Release() error = %v", err)
		}
		items, err := ledger.ListPendingOutbox(ctx, 10)
		if err != nil {
			t.Fatalf("ListPendingOutbox() error = %v", err)
		}
		if len(items) != 0 {
			t.Fatalf("released pending outbox = %+v, want empty", items)
		}
	})
	t.Run("reversal", func(t *testing.T) {
		db := openUsageLedgerTestDB(t)
		repo := NewGormRepository(db)
		seedUsageLedgerEntitlement(t, repo, "tenant-17", "studio", map[string]int{"studio_design_jobs_succeeded": 2})
		ledger := NewGormUsageLedger(repo)
		reservation, err := ledger.Reserve(ctx, usageLedgerReserveInput("tenant-17", "request-reversal-queue", 1))
		if err != nil {
			t.Fatalf("Reserve() error = %v", err)
		}
		if _, err := ledger.Commit(ctx, reservation.Event.EventID); err != nil {
			t.Fatalf("Commit() error = %v", err)
		}
		if _, err := ledger.Reverse(ctx, reservation.Event.EventID, "request-reversal-queue-reversal", "duplicate"); err != nil {
			t.Fatalf("Reverse() error = %v", err)
		}
		items, err := ledger.ListPendingOutbox(ctx, 10)
		if err != nil {
			t.Fatalf("ListPendingOutbox() error = %v", err)
		}
		if len(items) != 0 {
			t.Fatalf("reversal pending outbox = %+v, want source cancelled before delivery", items)
		}
	})
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
	second, err := ledger.Reverse(ctx, reservation.Event.EventID, "request-reversal-retry", "retry with another idempotency key")
	if err != nil {
		t.Fatalf("second Reverse() error = %v", err)
	}
	if second.EventID != reversal.EventID || second.ReversalOf != reservation.Event.EventID || second.Quantity != -1 {
		t.Fatalf("second Reverse() = %+v, want the original reversal without another bucket update", second)
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

func TestGormUsageLedgerProjectsReversalAfterDeliveryAndRejectsCountCorrection(t *testing.T) {
	ctx := context.Background()
	t.Run("storage", func(t *testing.T) {
		db := openUsageLedgerTestDB(t)
		repo := NewGormRepository(db)
		seedUsageLedgerEntitlement(t, repo, "tenant-17", ModuleOSSStorage, map[string]int{"storage_bytes": 100})
		ledger := NewGormUsageLedger(repo)
		reservation, err := ledger.Reserve(ctx, usageLedgerStorageInput("tenant-17", "storage-delivered", 10))
		if err != nil {
			t.Fatalf("Reserve() error = %v", err)
		}
		if _, err := ledger.Commit(ctx, reservation.Event.EventID); err != nil {
			t.Fatalf("Commit() error = %v", err)
		}
		if err := db.Model(&usageEventOutboxRow{}).Where("event_id = ?", reservation.Event.EventID).Update("status", "sent").Error; err != nil {
			t.Fatalf("mark delivered: %v", err)
		}
		reversal, err := ledger.Reverse(ctx, reservation.Event.EventID, "storage-delivered-reversal", "correction")
		if err != nil {
			t.Fatalf("Reverse() error = %v", err)
		}
		items, err := ledger.ListPendingOutbox(ctx, 10)
		if err != nil || len(items) != 1 || items[0].EventID != reversal.EventID {
			t.Fatalf("pending reversal = %+v, error=%v", items, err)
		}
		payload, err := BuildOpenMeterUsageOutboxPayload(reversal)
		if err != nil || payload.Quantity != 0 {
			t.Fatalf("reversal payload = %+v, error=%v, want zero snapshot", payload, err)
		}
	})
	t.Run("count", func(t *testing.T) {
		db := openUsageLedgerTestDB(t)
		repo := NewGormRepository(db)
		seedUsageLedgerEntitlement(t, repo, "tenant-17", ModuleStudio, map[string]int{"design_jobs": 2})
		ledger := NewGormUsageLedger(repo)
		reservation, err := ledger.Reserve(ctx, usageLedgerReserveInput("tenant-17", "count-delivered", 1))
		if err != nil {
			t.Fatalf("Reserve() error = %v", err)
		}
		if _, err := ledger.Commit(ctx, reservation.Event.EventID); err != nil {
			t.Fatalf("Commit() error = %v", err)
		}
		if err := db.Model(&usageEventOutboxRow{}).Where("event_id = ?", reservation.Event.EventID).Update("status", "sent").Error; err != nil {
			t.Fatalf("mark delivered: %v", err)
		}
		if _, err := ledger.Reverse(ctx, reservation.Event.EventID, "count-delivered-reversal", "correction"); !errors.Is(err, ErrUsageReversalProjectionUnsupported) {
			t.Fatalf("Reverse() error = %v, want ErrUsageReversalProjectionUnsupported", err)
		}
	})
}

func TestGormUsageLedgerReverseRejectsIdempotencyKeyOwnedByAnotherEvent(t *testing.T) {
	ctx := context.Background()
	db := openUsageLedgerTestDB(t)
	repo := NewGormRepository(db)
	seedUsageLedgerEntitlement(t, repo, "tenant-17", "studio", map[string]int{"studio_design_jobs_succeeded": 2})
	ledger := NewGormUsageLedger(repo)
	source, err := ledger.Reserve(ctx, usageLedgerReserveInput("tenant-17", "request-source", 1))
	if err != nil {
		t.Fatalf("Reserve() source error = %v", err)
	}
	if _, err := ledger.Commit(ctx, source.Event.EventID); err != nil {
		t.Fatalf("Commit() source error = %v", err)
	}
	if _, err := ledger.Reserve(ctx, usageLedgerReserveInput("tenant-17", "request-owned-by-reservation", 1)); err != nil {
		t.Fatalf("Reserve() other event error = %v", err)
	}
	_, err = ledger.Reverse(ctx, source.Event.EventID, "request-owned-by-reservation", "conflicting key")
	if !errors.Is(err, ErrUsageDuplicateIdentity) {
		t.Fatalf("Reverse() conflicting idempotency key error = %v, want ErrUsageDuplicateIdentity", err)
	}
	assertUsageLedgerCounts(t, db, source.Event.EventID, 2, 1, 1, 2)
}

func TestGormUsageLedgerReverseRejectsExistingReversalForNonCommittedSource(t *testing.T) {
	ctx := context.Background()
	db := openUsageLedgerTestDB(t)
	repo := NewGormRepository(db)
	seedUsageLedgerEntitlement(t, repo, "tenant-17", "studio", map[string]int{"studio_design_jobs_succeeded": 2})
	ledger := NewGormUsageLedger(repo)
	reservation, err := ledger.Reserve(ctx, usageLedgerReserveInput("tenant-17", "request-reserved-source", 1))
	if err != nil {
		t.Fatalf("Reserve() source error = %v", err)
	}
	if err := insertUsageReversal(db, "event-invalid-existing-reversal", "request-invalid-existing-reversal", reservation.Event.EventID); err != nil {
		t.Fatalf("insert invalid existing reversal fixture: %v", err)
	}
	_, err = ledger.Reverse(ctx, reservation.Event.EventID, "request-retry", "retry")
	if !errors.Is(err, ErrUsageInvalidTransition) {
		t.Fatalf("Reverse() reserved source with existing reversal error = %v, want ErrUsageInvalidTransition", err)
	}
}

func TestGormUsageLedgerReserveRejectsInactiveEntitlementWindow(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name      string
		startsAt  *time.Time
		expiresAt *time.Time
	}{
		{name: "not started", startsAt: timePtr(time.Now().Add(time.Hour))},
		{name: "expired", expiresAt: timePtr(time.Now().Add(-time.Hour))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := openUsageLedgerTestDB(t)
			repo := NewGormRepository(db)
			seedUsageLedgerEntitlementWithWindow(t, repo, "tenant-17", "studio", map[string]int{"studio_design_jobs_succeeded": 2}, tt.startsAt, tt.expiresAt)
			_, err := NewGormUsageLedger(repo).Reserve(ctx, usageLedgerReserveInput("tenant-17", "request-window-"+tt.name, 1))
			if !errors.Is(err, ErrSubscriptionRequired) {
				t.Fatalf("Reserve() entitlement %s error = %v, want ErrSubscriptionRequired", tt.name, err)
			}
		})
	}
}

func usageLedgerReserveInput(tenantID, idempotencyKey string, quantity int64) ReserveUsageInput {
	return ReserveUsageInput{TenantID: tenantID, ModuleCode: "studio", Metric: "studio_design_jobs_succeeded", Quantity: quantity, PeriodKey: "2026-08", SourceType: "design_job", SourceID: "job-42", IdempotencyKey: idempotencyKey, OccurredAt: time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC)}
}

func usageLedgerStorageInput(tenantID, idempotencyKey string, quantity int64) ReserveUsageInput {
	return ReserveUsageInput{TenantID: tenantID, ModuleCode: "oss_storage", Metric: usageMetricStorageBytesCurrent, Quantity: quantity, PeriodKey: "2026-08", SourceType: "storage_snapshot", SourceID: "bucket-42", IdempotencyKey: idempotencyKey, OccurredAt: time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC)}
}

func mustUsageEventID(t *testing.T, db *gorm.DB, idempotencyKey string) string {
	t.Helper()
	var row usageEventRow
	if err := db.Where("idempotency_key = ?", idempotencyKey).Take(&row).Error; err != nil {
		t.Fatalf("load usage event %q: %v", idempotencyKey, err)
	}
	return row.EventID
}

func seedUsageLedgerEntitlement(t *testing.T, repo *GormRepository, tenantID, moduleCode string, limits map[string]int) {
	seedUsageLedgerEntitlementWithWindow(t, repo, tenantID, moduleCode, limits, nil, nil)
}

func seedUsageLedgerEntitlementWithWindow(t *testing.T, repo *GormRepository, tenantID, moduleCode string, limits map[string]int, startsAt, expiresAt *time.Time) {
	t.Helper()
	if _, err := repo.UpsertEntitlement(context.Background(), &Entitlement{TenantID: tenantID, ModuleCode: moduleCode, Status: StatusActive, Limits: limits, StartsAt: startsAt, ExpiresAt: expiresAt}); err != nil {
		t.Fatalf("UpsertEntitlement() error = %v", err)
	}
}

func timePtr(value time.Time) *time.Time { return &value }

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
		"idx_saas_usage_event_reversal_of",
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

	t.Run("single reversal per source event", func(t *testing.T) {
		db := openUsageLedgerTestDB(t)
		if err := insertUsageEvent(db, "event-source", "tenant-17", "request-source", "committed"); err != nil {
			t.Fatalf("insert source usage event: %v", err)
		}
		if err := insertUsageReversal(db, "event-reversal-first", "request-reversal-first", "event-source"); err != nil {
			t.Fatalf("insert first reversal: %v", err)
		}
		if err := insertUsageReversal(db, "event-reversal-duplicate", "request-reversal-duplicate", "event-source"); err == nil {
			t.Fatal("insert second reversal for source event succeeded, want unique constraint failure")
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

func openConcurrentUsageLedgerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + filepath.ToSlash(filepath.Join(t.TempDir(), "usage-ledger.db")) + "?mode=rwc&_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)"
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: dsn}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open concurrent db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("open concurrent sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(20)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := AutoMigrateRepository(db); err != nil {
		t.Fatalf("AutoMigrateRepository() concurrent error = %v", err)
	}
	return db
}

func insertUsageEvent(db *gorm.DB, eventID, tenantID, idempotencyKey, status string) error {
	return db.Exec(`INSERT INTO saas_usage_events (
		event_id, tenant_id, module_code, metric, quantity, period_key,
		source_type, source_id, idempotency_key, status, occurred_at
	) VALUES (?, ?, 'studio', 'studio_design_jobs_succeeded', 1, '2026-08', 'design_job', 'job-42', ?, ?, CURRENT_TIMESTAMP)`, eventID, tenantID, idempotencyKey, status).Error
}

func insertUsageReversal(db *gorm.DB, eventID, idempotencyKey, reversalOf string) error {
	return db.Exec(`INSERT INTO saas_usage_events (
		event_id, tenant_id, module_code, metric, quantity, period_key,
		source_type, source_id, idempotency_key, status, occurred_at, reversal_of
	) VALUES (?, 'tenant-17', 'studio', 'studio_design_jobs_succeeded', -1, '2026-08', 'design_job', 'job-42', ?, 'reversed', CURRENT_TIMESTAMP, ?)`, eventID, idempotencyKey, reversalOf).Error
}
