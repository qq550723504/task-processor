package listingsubscription

import (
	"context"
	"errors"
	"testing"
)

func TestServiceGetUsageDelegatesToConfiguredLedger(t *testing.T) {
	t.Parallel()

	repo := NewMemRepository()
	ledger := NewMemUsageLedger(repo)
	svc, err := NewServiceWithLedger(repo, ledger)
	if err != nil {
		t.Fatalf("NewServiceWithLedger() error = %v", err)
	}
	seedServiceUsageLedgerEntitlement(t, svc)
	reservation, err := svc.ReserveUsage(context.Background(), serviceUsageLedgerInput())
	if err != nil {
		t.Fatalf("ReserveUsage() error = %v", err)
	}

	got, err := svc.GetUsage(context.Background(), "tenant-17", "request-42")
	if err != nil {
		t.Fatalf("GetUsage() error = %v", err)
	}
	if got == nil || got.EventID != reservation.Event.EventID {
		t.Fatalf("GetUsage() = %#v, want event %q", got, reservation.Event.EventID)
	}
}

func TestServiceGetUsageRequiresConfiguredLedger(t *testing.T) {
	t.Parallel()

	svc, err := NewService(NewMemRepository())
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if _, err := svc.GetUsage(context.Background(), "tenant-17", "request-42"); !errors.Is(err, ErrUsageLedgerNotConfigured) {
		t.Fatalf("GetUsage() error = %v, want ErrUsageLedgerNotConfigured", err)
	}
}

func TestServiceGetUsageClassifiesMissingEvent(t *testing.T) {
	t.Parallel()

	repo := NewMemRepository()
	svc, err := NewServiceWithLedger(repo, NewMemUsageLedger(repo))
	if err != nil {
		t.Fatalf("NewServiceWithLedger() error = %v", err)
	}
	if _, err := svc.GetUsage(context.Background(), "tenant-17", "missing-event"); !errors.Is(err, ErrUsageEventNotFound) {
		t.Fatalf("GetUsage() error = %v, want ErrUsageEventNotFound", err)
	}
}
