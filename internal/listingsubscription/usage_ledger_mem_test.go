package listingsubscription

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestUsageLedgerConcurrentReservationsRespectLimit(t *testing.T) {
	ctx := context.Background()
	repo := NewMemRepository()
	seedMemUsageLedgerEntitlement(t, repo, "tenant-17", "studio", map[string]int{"studio_design_jobs_succeeded": 10})
	ledger := NewMemUsageLedger(repo)

	start := make(chan struct{})
	type reservationOutcome struct {
		result ReserveUsageResult
		err    error
	}
	outcomes := make(chan reservationOutcome, 20)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			result, err := ledger.Reserve(ctx, usageLedgerReserveInput("tenant-17", fmt.Sprintf("concurrent-%d", i), 1))
			outcomes <- reservationOutcome{result: result, err: err}
		}(i)
	}
	close(start)
	wg.Wait()
	close(outcomes)

	succeeded := 0
	eventIDs := map[string]struct{}{}
	var oneSuccessfulKey string
	for outcome := range outcomes {
		if outcome.err == nil {
			succeeded++
			eventIDs[outcome.result.Event.EventID] = struct{}{}
			oneSuccessfulKey = outcome.result.Event.IdempotencyKey
			continue
		}
		if !errors.Is(outcome.err, ErrUsageQuotaExceeded) {
			t.Fatalf("Reserve() error = %v, want ErrUsageQuotaExceeded", outcome.err)
		}
	}
	if succeeded != 10 {
		t.Fatalf("successful reservations = %d, want 10", succeeded)
	}
	if len(eventIDs) != 10 {
		t.Fatalf("unique event IDs = %d, want 10", len(eventIDs))
	}

	replay, err := ledger.Reserve(ctx, usageLedgerReserveInput("tenant-17", oneSuccessfulKey, 1))
	if err != nil {
		t.Fatalf("replay Reserve() error = %v", err)
	}
	if !replay.Existing || replay.CommittedUsage+replay.ReservedUsage != 10 {
		t.Fatalf("replay Reserve() = %+v, want existing event and total usage 10", replay)
	}
	for eventID := range eventIDs {
		if _, err := ledger.Commit(ctx, eventID); err != nil {
			t.Fatalf("Commit(%q) error = %v", eventID, err)
		}
	}
	outbox, err := ledger.ListPendingOutbox(ctx, 20)
	if err != nil {
		t.Fatalf("ListPendingOutbox() error = %v", err)
	}
	if len(outbox) != 10 {
		t.Fatalf("pending outbox items = %d, want 10", len(outbox))
	}
	for _, item := range outbox {
		if _, ok := eventIDs[item.EventID]; !ok || item.Status != "in_flight" {
			t.Fatalf("outbox event ID %q was not returned by a successful reservation", item.EventID)
		}
	}
}

func TestMemUsageLedgerConcurrentReplayCreatesOneReservation(t *testing.T) {
	ctx := context.Background()
	repo := NewMemRepository()
	seedMemUsageLedgerEntitlement(t, repo, "tenant-17", "studio", map[string]int{"studio_design_jobs_succeeded": 10})
	ledger := NewMemUsageLedger(repo)

	start := make(chan struct{})
	results := make(chan ReserveUsageResult, 20)
	errors := make(chan error, 20)
	var wg sync.WaitGroup
	input := usageLedgerReserveInput("tenant-17", "concurrent-replay", 1)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			result, err := ledger.Reserve(ctx, input)
			if err != nil {
				errors <- err
				return
			}
			results <- result
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errors)

	for err := range errors {
		t.Fatalf("Reserve() error = %v", err)
	}
	var eventID string
	for result := range results {
		if eventID == "" {
			eventID = result.Event.EventID
		}
		if result.Event.EventID != eventID {
			t.Fatalf("Reserve() event ID = %q, want %q", result.Event.EventID, eventID)
		}
	}
	if eventID == "" {
		t.Fatal("Reserve() did not return an event ID")
	}
	replay, err := ledger.Reserve(ctx, input)
	if err != nil {
		t.Fatalf("replay Reserve() error = %v", err)
	}
	if !replay.Existing || replay.Event.EventID != eventID || replay.CommittedUsage != 0 || replay.ReservedUsage != 1 {
		t.Fatalf("replay Reserve() = %+v, want existing event and one reserved unit", replay)
	}
	if _, err := ledger.Commit(ctx, eventID); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	outbox, err := ledger.ListPendingOutbox(ctx, 20)
	if err != nil {
		t.Fatalf("ListPendingOutbox() error = %v", err)
	}
	if len(outbox) != 1 || outbox[0].EventID != eventID || outbox[0].Status != "in_flight" {
		t.Fatalf("pending outbox items = %+v, want one item for %q", outbox, eventID)
	}
}

func TestMemUsageLedgerClaimedOutboxBlocksReverseUntilResolved(t *testing.T) {
	ctx := context.Background()
	repo := NewMemRepository()
	seedMemUsageLedgerEntitlement(t, repo, "tenant-17", ModuleStudio, map[string]int{"studio_design_jobs_succeeded": 2})
	ledger := NewMemUsageLedger(repo)
	reservation, err := ledger.Reserve(ctx, usageLedgerReserveInput("tenant-17", "request-claim-reverse", 1))
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	if _, err := ledger.Commit(ctx, reservation.Event.EventID); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	items, err := ledger.ListPendingOutbox(ctx, 1)
	if err != nil || len(items) != 1 || items[0].Status != "in_flight" {
		t.Fatalf("ListPendingOutbox() = %+v, error=%v, want one in-flight item", items, err)
	}
	if _, err := ledger.Reverse(ctx, reservation.Event.EventID, "request-claim-reverse-comp", "unknown delivery"); !errors.Is(err, ErrUsageReversalDeliveryUnresolved) {
		t.Fatalf("Reverse() error = %v, want ErrUsageReversalDeliveryUnresolved", err)
	}
}

func TestMemUsageLedgerClaimOutboxHonorsLimit(t *testing.T) {
	ctx := context.Background()
	repo := NewMemRepository()
	seedMemUsageLedgerEntitlement(t, repo, "tenant-17", ModuleStudio, map[string]int{"studio_design_jobs_succeeded": 10})
	ledger := NewMemUsageLedger(repo)
	for _, key := range []string{"request-limit-one", "request-limit-two"} {
		reservation, err := ledger.Reserve(ctx, usageLedgerReserveInput("tenant-17", key, 1))
		if err != nil {
			t.Fatalf("Reserve(%s) error = %v", key, err)
		}
		if _, err := ledger.Commit(ctx, reservation.Event.EventID); err != nil {
			t.Fatalf("Commit(%s) error = %v", key, err)
		}
	}
	first, err := ledger.ListPendingOutbox(ctx, 1)
	if err != nil || len(first) != 1 {
		t.Fatalf("first ListPendingOutbox() = %+v, error=%v, want one item", first, err)
	}
	second, err := ledger.ListPendingOutbox(ctx, 1)
	if err != nil || len(second) != 1 || second[0].EventID == first[0].EventID {
		t.Fatalf("second ListPendingOutbox() = %+v, error=%v, want the remaining item", second, err)
	}
}

func TestMemUsageLedgerRejectsIdempotencyKeyForDifferentUsageFact(t *testing.T) {
	repo := NewMemRepository()
	seedMemUsageLedgerEntitlement(t, repo, "tenant-17", "studio", map[string]int{"studio_design_jobs_succeeded": 10})
	ledger := NewMemUsageLedger(repo)
	input := usageLedgerReserveInput("tenant-17", "request-fact", 1)
	if _, err := ledger.Reserve(context.Background(), input); err != nil {
		t.Fatalf("first Reserve() error = %v", err)
	}
	input.SourceID = "different-job"
	if _, err := ledger.Reserve(context.Background(), input); !errors.Is(err, ErrUsageDuplicateIdentity) {
		t.Fatalf("Reserve() changed fact error = %v, want ErrUsageDuplicateIdentity", err)
	}
}

func TestMemUsageLedgerRejectsNonCanonicalPeriodKey(t *testing.T) {
	repo := NewMemRepository()
	seedMemUsageLedgerEntitlement(t, repo, "tenant-17", "studio", map[string]int{"design_jobs": 10})
	input := usageLedgerReserveInput("tenant-17", "period-mismatch", 1)
	input.PeriodKey = "2026-08-retry"
	if _, err := NewMemUsageLedger(repo).Reserve(context.Background(), input); !errors.Is(err, ErrUsageInvalidInput) {
		t.Fatalf("Reserve() error = %v, want ErrUsageInvalidInput", err)
	}
}

func TestMemUsageLedgerUsesStorageFallbackAndUnlimitedZeroLimit(t *testing.T) {
	t.Run("legacy entitlement metric mapping", func(t *testing.T) {
		repo := NewMemRepository()
		seedMemUsageLedgerEntitlement(t, repo, "tenant-17", ModuleStudio, map[string]int{"design_jobs": 1})
		ledger := NewMemUsageLedger(repo)
		if _, err := ledger.Reserve(context.Background(), usageLedgerReserveInput("tenant-17", "legacy-limit-1", 1)); err != nil {
			t.Fatalf("first mapped Reserve() error = %v", err)
		}
		if _, err := ledger.Reserve(context.Background(), usageLedgerReserveInput("tenant-17", "legacy-limit-2", 1)); !errors.Is(err, ErrUsageQuotaExceeded) {
			t.Fatalf("second mapped Reserve() error = %v, want ErrUsageQuotaExceeded", err)
		}
	})
	t.Run("storage fallback", func(t *testing.T) {
		repo := NewMemRepository()
		seedMemUsageLedgerEntitlement(t, repo, "tenant-17", ModuleStudio, nil)
		if _, err := NewMemUsageLedger(repo).Reserve(context.Background(), usageLedgerStorageInput("tenant-17", "storage-fallback", 1)); err != nil {
			t.Fatalf("storage fallback Reserve() error = %v", err)
		}
	})
	t.Run("zero limit", func(t *testing.T) {
		repo := NewMemRepository()
		seedMemUsageLedgerEntitlement(t, repo, "tenant-17", ModuleStudio, map[string]int{"studio_design_jobs_succeeded": 0})
		if _, err := NewMemUsageLedger(repo).Reserve(context.Background(), usageLedgerReserveInput("tenant-17", "unlimited", 1)); err != nil {
			t.Fatalf("zero-limit Reserve() error = %v, want unlimited semantics", err)
		}
	})
}

func TestMemUsageLedgerStorageSignedTransitionsAndOutboxFiltering(t *testing.T) {
	repo := NewMemRepository()
	seedMemUsageLedgerEntitlement(t, repo, "tenant-17", ModuleOSSStorage, map[string]int{"storage_bytes_current": 100})
	ledger := NewMemUsageLedger(repo)
	ctx := context.Background()
	base, err := ledger.Reserve(ctx, usageLedgerStorageInput("tenant-17", "storage-base", 10))
	if err != nil {
		t.Fatalf("base Reserve() error = %v", err)
	}
	baseCommitted, err := ledger.Commit(ctx, base.Event.EventID)
	if err != nil {
		t.Fatalf("base Commit() error = %v", err)
	}
	if baseCommitted.StorageSnapshot == nil || *baseCommitted.StorageSnapshot != 10 {
		t.Fatalf("base storage snapshot = %v, want 10", baseCommitted.StorageSnapshot)
	}
	remove := usageLedgerStorageInput("tenant-17", "storage-delete", -8)
	remove.PeriodKey = "2026-09"
	if _, err := ledger.Reserve(ctx, remove); err != nil {
		t.Fatalf("delete Reserve() error = %v", err)
	}
	add := usageLedgerStorageInput("tenant-17", "storage-add", 2)
	add.PeriodKey = "2026-09"
	addEvent, err := ledger.Reserve(ctx, add)
	if err != nil {
		t.Fatalf("add Reserve() error = %v", err)
	}
	committedAdd, err := ledger.Commit(ctx, addEvent.Event.EventID)
	if err != nil {
		t.Fatalf("add Commit() error = %v", err)
	}
	if committedAdd.StorageSnapshot == nil || *committedAdd.StorageSnapshot != 12 {
		t.Fatalf("add storage snapshot = %v, want committed 12", committedAdd.StorageSnapshot)
	}
	items, err := ledger.ListPendingOutbox(ctx, 20)
	if err != nil || len(items) != 2 {
		t.Fatalf("pending outbox after signed reservations = %d, %v; want 2 committed events", len(items), err)
	}
	deleteEvent, err := ledger.Get(ctx, "tenant-17", "storage-delete")
	if err != nil {
		t.Fatalf("Get() delete event: %v", err)
	}
	if _, err := ledger.Commit(ctx, deleteEvent.EventID); err != nil {
		t.Fatalf("delete Commit() error = %v", err)
	}
	final, err := ledger.Reserve(ctx, ReserveUsageInput{TenantID: "tenant-17", ModuleCode: ModuleOSSStorage, Metric: usageMetricStorageBytesCurrent, Quantity: -4, PeriodKey: "2026-10", SourceType: "storage_snapshot", SourceID: "bucket-42", IdempotencyKey: "storage-final", OccurredAt: time.Now().UTC()})
	if err != nil {
		t.Fatalf("period rollover Reserve() error = %v", err)
	}
	finalCommitted, err := ledger.Commit(ctx, final.Event.EventID)
	if err != nil {
		t.Fatalf("period rollover Commit() error = %v", err)
	}
	if finalCommitted.StorageSnapshot == nil || *finalCommitted.StorageSnapshot != 0 {
		t.Fatalf("period rollover snapshot = %v, want 0", finalCommitted.StorageSnapshot)
	}
}

func TestMemUsageLedgerRejectsDeletionCommitBelowZeroAgainstUncommittedUpload(t *testing.T) {
	repo := NewMemRepository()
	seedMemUsageLedgerEntitlement(t, repo, "tenant-17", ModuleOSSStorage, map[string]int{"storage_bytes_current": 100})
	ledger := NewMemUsageLedger(repo)
	ctx := context.Background()
	upload, err := ledger.Reserve(ctx, usageLedgerStorageInput("tenant-17", "storage-upload-first", 10))
	if err != nil {
		t.Fatalf("upload Reserve() error = %v", err)
	}
	deletion, err := ledger.Reserve(ctx, usageLedgerStorageInput("tenant-17", "storage-delete-first", -10))
	if err != nil {
		t.Fatalf("deletion Reserve() error = %v", err)
	}
	if _, err := ledger.Commit(ctx, deletion.Event.EventID); !errors.Is(err, ErrUsageQuotaExceeded) {
		t.Fatalf("deletion Commit() error = %v, want ErrUsageQuotaExceeded", err)
	}
	if _, err := ledger.Commit(ctx, upload.Event.EventID); err != nil {
		t.Fatalf("upload Commit() error = %v", err)
	}
}

func seedMemUsageLedgerEntitlement(t *testing.T, repo *MemRepository, tenantID, moduleCode string, limits map[string]int) {
	t.Helper()
	if _, err := repo.UpsertEntitlement(context.Background(), &Entitlement{TenantID: tenantID, ModuleCode: moduleCode, Status: StatusActive, Limits: limits}); err != nil {
		t.Fatalf("UpsertEntitlement() error = %v", err)
	}
}
