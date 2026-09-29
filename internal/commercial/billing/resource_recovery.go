package billing

import (
	"context"
	"errors"
	"time"
)

type ResourceRecoveryPosition struct {
	CreatedAt time.Time
	OrderID   string
}

type ResourceRecoveryPage struct {
	Orders []Order
	Next   *ResourceRecoveryPosition
}

type ResourceRecoveryStore interface {
	ListRecoverableResourceOrders(context.Context, *ResourceRecoveryPosition, int) (ResourceRecoveryPage, error)
}

// The cursor is scheduling state only. A restart can revisit the same original
// orders; durable money decisions and source-bound grants make that safe.
func (s *Service) ReconcileRecoverableResourceOrders(ctx context.Context, limit int) error {
	if s == nil || limit < 1 || limit > 50 {
		return ErrInvalid
	}
	store, ok := s.orders.(ResourceRecoveryStore)
	if !ok {
		return ErrFeatureUnavailable
	}
	s.resourceRecoveryLock.Lock()
	defer s.resourceRecoveryLock.Unlock()
	page, err := store.ListRecoverableResourceOrders(ctx, s.resourceRecoveryAfter, limit)
	if err != nil {
		return err
	}
	if len(page.Orders) > limit {
		return ErrFeatureUnavailable
	}
	s.resourceRecoveryAfter = page.Next
	var failures []error
	for _, order := range page.Orders {
		if ctx.Err() != nil {
			return errors.Join(append(failures, ctx.Err())...)
		}
		_, err := s.ReconcileResourceOrder(ctx, order.OrganizationID, order.OrderID)
		if err != nil && !errors.Is(err, ErrReconciliationRequired) && !errors.Is(err, ErrInsufficientFunds) && !errors.Is(err, ErrResourceGrantRejected) {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
