package httpapi

import (
	"context"
	"errors"
	"testing"

	"task-processor/internal/listingsubscription"
)

func TestSubscriptionStudioProductImageUsageReservationUsesDurableLedger(t *testing.T) {
	t.Parallel()

	repo := listingsubscription.NewMemRepository()
	svc, err := listingsubscription.NewServiceWithLedger(repo, listingsubscription.NewMemUsageLedger(repo))
	if err != nil {
		t.Fatalf("NewServiceWithLedger() error = %v", err)
	}
	if _, err := svc.UpsertEntitlement(context.Background(), "tenant-17", listingsubscription.ModuleStudio, listingsubscription.EntitlementInput{
		Status: listingsubscription.StatusActive,
		Limits: map[string]int{"product_image_jobs": 2},
	}); err != nil {
		t.Fatalf("UpsertEntitlement() error = %v", err)
	}

	adapter := studioProductImageUsageDependency(svc)
	if err := adapter.ReserveProductImageUsage(context.Background(), "tenant-17", "candidate-42", 2); err != nil {
		t.Fatalf("ReserveProductImageUsage() error = %v", err)
	}
	event, err := svc.GetUsage(context.Background(), "tenant-17", "listingkit:studio_product_image:candidate-42")
	if err != nil {
		t.Fatalf("GetUsage() error = %v", err)
	}
	if event.Status != listingsubscription.UsageEventReserved || event.Quantity != 2 || event.Metric != "product_image_jobs_succeeded" {
		t.Fatalf("event = %#v, want reserved product-image event", event)
	}
	if err := adapter.ReserveProductImageUsage(context.Background(), "tenant-17", "candidate-over-limit", 1); err == nil {
		t.Fatal("ReserveProductImageUsage() unexpectedly allowed a concurrent reservation beyond the limit")
	}
	if err := adapter.CommitProductImageUsage(context.Background(), "tenant-17", "candidate-42"); err != nil {
		t.Fatalf("CommitProductImageUsage() error = %v", err)
	}
	event, err = svc.GetUsage(context.Background(), "tenant-17", "listingkit:studio_product_image:candidate-42")
	if err != nil {
		t.Fatalf("GetUsage() after commit error = %v", err)
	}
	if event.Status != listingsubscription.UsageEventCommitted {
		t.Fatalf("event status = %q, want committed", event.Status)
	}
	if err := adapter.ReserveProductImageUsage(context.Background(), "tenant-17", "candidate-42", 2); err != nil {
		t.Fatalf("idempotent ReserveProductImageUsage() error = %v", err)
	}
}

func TestSubscriptionStudioProductImageUsageDisablesReservationWithoutLedger(t *testing.T) {
	t.Parallel()

	svc, err := listingsubscription.NewService(listingsubscription.NewMemRepository())
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	adapter := studioProductImageUsageDependency(svc)
	if adapter.StudioProductImageUsageReservationEnabled() {
		t.Fatal("StudioProductImageUsageReservationEnabled() = true, want false")
	}
}

func TestSubscriptionStudioProductImageUsageReservationAccountsForLegacyCounter(t *testing.T) {
	t.Parallel()

	repo := listingsubscription.NewMemRepository()
	svc, err := listingsubscription.NewServiceWithLedger(repo, listingsubscription.NewMemUsageLedger(repo))
	if err != nil {
		t.Fatalf("NewServiceWithLedger() error = %v", err)
	}
	if _, err := svc.UpsertEntitlement(context.Background(), "tenant-legacy", listingsubscription.ModuleStudio, listingsubscription.EntitlementInput{
		Status: listingsubscription.StatusActive,
		Limits: map[string]int{"product_image_jobs": 2},
	}); err != nil {
		t.Fatalf("UpsertEntitlement() error = %v", err)
	}
	if _, err := svc.RecordUsage(context.Background(), "tenant-legacy", listingsubscription.ModuleStudio, "product_image_jobs", 1); err != nil {
		t.Fatalf("RecordUsage() error = %v", err)
	}
	adapter := studioProductImageUsageDependency(svc)
	if err := adapter.ReserveProductImageUsage(context.Background(), "tenant-legacy", "candidate-1", 2); err == nil {
		t.Fatal("ReserveProductImageUsage() allowed a reservation beyond legacy usage")
	}
}

func TestSubscriptionStudioProductImageUsageReservationMirrorsLegacyCounter(t *testing.T) {
	t.Parallel()

	repo := listingsubscription.NewMemRepository()
	svc, err := listingsubscription.NewServiceWithLedger(repo, listingsubscription.NewMemUsageLedger(repo))
	if err != nil {
		t.Fatalf("NewServiceWithLedger() error = %v", err)
	}
	if _, err := svc.UpsertEntitlement(context.Background(), "tenant-mirror", listingsubscription.ModuleStudio, listingsubscription.EntitlementInput{
		Status: listingsubscription.StatusActive,
		Limits: map[string]int{"product_image_jobs": 2},
	}); err != nil {
		t.Fatalf("UpsertEntitlement() error = %v", err)
	}
	adapter := studioProductImageUsageDependency(svc)
	if err := adapter.ReserveProductImageUsage(context.Background(), "tenant-mirror", "candidate-1", 1); err != nil {
		t.Fatalf("ReserveProductImageUsage() error = %v", err)
	}
	if err := adapter.AuthorizeProductImageUsage(context.Background(), "tenant-mirror", 2); !errors.Is(err, listingsubscription.ErrSubscriptionQuotaExceed) {
		t.Fatalf("AuthorizeProductImageUsage() error = %v, want quota exceeded from mirrored ledger reservation", err)
	}
	if err := adapter.ReleaseProductImageUsage(context.Background(), "tenant-mirror", "candidate-1", "test"); err != nil {
		t.Fatalf("ReleaseProductImageUsage() error = %v", err)
	}
	if err := adapter.AuthorizeProductImageUsage(context.Background(), "tenant-mirror", 2); err != nil {
		t.Fatalf("AuthorizeProductImageUsage() after release error = %v, want released mirror", err)
	}
}
