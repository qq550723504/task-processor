package storecenter

import (
	"context"
	"sync"
	"task-processor/internal/listingsubscription"
	"time"
)

const reservationLeaseStopTimeout = 5 * time.Second

type reservationLease struct {
	mu        sync.Mutex
	input     listingsubscription.StoreQuotaTransitionInput
	updatedAt *time.Time
	cancel    context.CancelFunc
	done      chan struct{}
	stopOnce  sync.Once
	stopErr   error
}

func (s *Service) keepReservationLeaseAlive(ctx context.Context, input listingsubscription.StoreQuotaTransitionInput, renewInterval time.Duration) *reservationLease {
	leaseCtx, cancel := context.WithCancel(ctx)
	lease := &reservationLease{input: input, cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(lease.done)
		ticker := time.NewTicker(renewInterval)
		defer ticker.Stop()
		for {
			select {
			case <-leaseCtx.Done():
				return
			case <-ticker.C:
				result, err := s.quota.RenewReservation(leaseCtx, input)
				if err != nil || result.Allocation.UpdatedAt.IsZero() {
					continue
				}
				lease.mu.Lock()
				updatedAt := result.Allocation.UpdatedAt.UTC()
				lease.updatedAt = &updatedAt
				lease.mu.Unlock()
			}
		}
	}()
	return lease
}

func (lease *reservationLease) stop() error {
	lease.stopOnce.Do(func() {
		lease.cancel()
		timer := time.NewTimer(reservationLeaseStopTimeout)
		defer timer.Stop()
		select {
		case <-lease.done:
		case <-timer.C:
			lease.stopErr = context.DeadlineExceeded
		}
	})
	return lease.stopErr
}

func (lease *reservationLease) transition() listingsubscription.StoreQuotaTransitionInput {
	lease.mu.Lock()
	defer lease.mu.Unlock()
	input := lease.input
	if lease.updatedAt != nil {
		updatedAt := *lease.updatedAt
		input.ExpectedUpdatedAt = &updatedAt
	}
	return input
}
