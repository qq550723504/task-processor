package listingsubscription

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
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
	outbox, err := ledger.ListPendingOutbox(ctx, 20)
	if err != nil {
		t.Fatalf("ListPendingOutbox() error = %v", err)
	}
	if len(outbox) != 10 {
		t.Fatalf("pending outbox items = %d, want 10", len(outbox))
	}
	for _, item := range outbox {
		if _, ok := eventIDs[item.EventID]; !ok {
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
	outbox, err := ledger.ListPendingOutbox(ctx, 20)
	if err != nil {
		t.Fatalf("ListPendingOutbox() error = %v", err)
	}
	if len(outbox) != 1 || outbox[0].EventID != eventID {
		t.Fatalf("pending outbox items = %+v, want one item for %q", outbox, eventID)
	}
}

func seedMemUsageLedgerEntitlement(t *testing.T, repo *MemRepository, tenantID, moduleCode string, limits map[string]int) {
	t.Helper()
	if _, err := repo.UpsertEntitlement(context.Background(), &Entitlement{TenantID: tenantID, ModuleCode: moduleCode, Status: StatusActive, Limits: limits}); err != nil {
		t.Fatalf("UpsertEntitlement() error = %v", err)
	}
}
