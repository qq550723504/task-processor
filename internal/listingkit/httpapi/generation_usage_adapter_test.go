package httpapi

import (
	"context"
	"testing"
	"time"

	"task-processor/internal/listingsubscription"
)

func TestSubscriptionGenerationUsageAdapterMapsCanonicalFact(t *testing.T) {
	t.Parallel()

	repo := listingsubscription.NewMemRepository()
	ledger := listingsubscription.NewMemUsageLedger(repo)
	svc, err := listingsubscription.NewServiceWithLedger(repo, ledger)
	if err != nil {
		t.Fatalf("NewServiceWithLedger() error = %v", err)
	}
	if _, err := svc.UpsertEntitlement(context.Background(), "tenant-17", listingsubscription.ModuleStudio, listingsubscription.EntitlementInput{
		Status: listingsubscription.StatusActive,
		Limits: map[string]int{"studio_design_jobs_succeeded": 2},
	}); err != nil {
		t.Fatalf("UpsertEntitlement() error = %v", err)
	}

	adapter := newSubscriptionGenerationUsage(svc)
	reservation, err := adapter.ReserveGeneration(context.Background(), "tenant-17", "task-42", time.Date(2026, 8, 14, 2, 3, 4, 0, time.UTC))
	if err != nil {
		t.Fatalf("ReserveGeneration() error = %v", err)
	}
	if reservation.EventID == "" || reservation.AlreadyCommitted {
		t.Fatalf("reservation = %#v, want new reserved event", reservation)
	}

	event, err := svc.GetUsage(context.Background(), "tenant-17", "listingkit:generation:task-42")
	if err != nil {
		t.Fatalf("GetUsage() error = %v", err)
	}
	if event.ModuleCode != listingsubscription.ModuleStudio || event.Metric != "studio_design_jobs_succeeded" || event.Quantity != 1 || event.SourceType != "listingkit_generation" || event.SourceID != "task-42" {
		t.Fatalf("event = %#v, want canonical generation fact", event)
	}
}
