package storecenter

import (
	"context"
	"sync"
	"testing"
	"time"

	"task-processor/internal/listingsubscription"
)

func TestReservationLeaseTracksLatestRenewalBeforeStop(t *testing.T) {
	updatedAt := time.Date(2026, 9, 2, 1, 2, 3, 4, time.UTC)
	ledger := &reservationLeaseQuotaFake{updatedAt: updatedAt, renewed: make(chan struct{}, 1)}
	service := &Service{quota: ledger}
	input := listingsubscription.StoreQuotaTransitionInput{OrganizationID: "org-1", AllocationID: "allocation-1", StoreID: "store-1", RequestKey: "request-1", ActorSubject: "actor-1"}
	lease := service.keepReservationLeaseAlive(context.Background(), input, time.Millisecond)

	select {
	case <-ledger.renewed:
	case <-time.After(time.Second):
		t.Fatal("reservation lease did not renew")
	}

	if err := lease.stop(); err != nil {
		t.Fatalf("stop() error = %v", err)
	}
	release := lease.transition()
	if release.ExpectedUpdatedAt == nil || !release.ExpectedUpdatedAt.Equal(updatedAt) {
		t.Fatalf("release fence = %v, want renewal timestamp %v", release.ExpectedUpdatedAt, updatedAt)
	}

	calls := ledger.renewCalls()
	time.Sleep(5 * time.Millisecond)
	if got := ledger.renewCalls(); got != calls {
		t.Fatalf("renew calls after stop = %d, want %d", got, calls)
	}
}

type reservationLeaseQuotaFake struct {
	mu        sync.Mutex
	updatedAt time.Time
	renewed   chan struct{}
	calls     int
}

func (f *reservationLeaseQuotaFake) Reserve(context.Context, listingsubscription.StoreQuotaReserveInput) (listingsubscription.StoreQuotaReserveResult, error) {
	return listingsubscription.StoreQuotaReserveResult{}, nil
}

func (f *reservationLeaseQuotaFake) RenewReservation(context.Context, listingsubscription.StoreQuotaTransitionInput) (listingsubscription.StoreQuotaTransitionResult, error) {
	f.mu.Lock()
	f.calls++
	updatedAt := f.updatedAt
	f.mu.Unlock()
	select {
	case f.renewed <- struct{}{}:
	default:
	}
	return listingsubscription.StoreQuotaTransitionResult{Allocation: listingsubscription.StoreQuotaAllocation{UpdatedAt: updatedAt}}, nil
}

func (f *reservationLeaseQuotaFake) Commit(context.Context, listingsubscription.StoreQuotaTransitionInput) (listingsubscription.StoreQuotaTransitionResult, error) {
	return listingsubscription.StoreQuotaTransitionResult{}, nil
}

func (f *reservationLeaseQuotaFake) ReleaseReservation(context.Context, listingsubscription.StoreQuotaTransitionInput) (listingsubscription.StoreQuotaTransitionResult, error) {
	return listingsubscription.StoreQuotaTransitionResult{}, nil
}

func (f *reservationLeaseQuotaFake) Deallocate(context.Context, listingsubscription.StoreQuotaTransitionInput) (listingsubscription.StoreQuotaTransitionResult, error) {
	return listingsubscription.StoreQuotaTransitionResult{}, nil
}

func (f *reservationLeaseQuotaFake) GetByRequestKey(context.Context, string, string) (*listingsubscription.StoreQuotaAllocation, error) {
	return nil, nil
}

func (f *reservationLeaseQuotaFake) Summary(context.Context, string) (listingsubscription.StoreQuotaSummary, error) {
	return listingsubscription.StoreQuotaSummary{}, nil
}

func (f *reservationLeaseQuotaFake) renewCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}
