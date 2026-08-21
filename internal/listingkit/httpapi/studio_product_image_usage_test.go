package httpapi

import (
	"context"
	"errors"
	"testing"

	"task-processor/internal/listingsubscription"
	"task-processor/internal/tenantbridge"
)

type studioProductImageUsageLegacyTenantResolver struct {
	legacyTenantID int64
}

func (r studioProductImageUsageLegacyTenantResolver) ResolveLegacyTenantID(context.Context, string) (int64, bool, error) {
	return r.legacyTenantID, true, nil
}

type failingNegativeProductImageUsageRepository struct {
	listingsubscription.Repository
	failNegative bool
}

func (r *failingNegativeProductImageUsageRepository) IncrementUsage(ctx context.Context, tenantID, moduleCode, periodKey, metric string, amount int) (*listingsubscription.UsageCounter, error) {
	if r.failNegative && amount < 0 {
		return nil, errors.New("legacy usage mirror temporarily unavailable")
	}
	return r.Repository.IncrementUsage(ctx, tenantID, moduleCode, periodKey, metric, amount)
}

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

func TestSubscriptionStudioProductImageUsageUsesLegacyBillingTenantConsistently(t *testing.T) {
	repo := listingsubscription.NewMemRepository()
	svc, err := listingsubscription.NewServiceWithLedger(repo, listingsubscription.NewMemUsageLedger(repo))
	if err != nil {
		t.Fatalf("NewServiceWithLedger() error = %v", err)
	}
	if _, err := svc.UpsertEntitlement(context.Background(), "246", listingsubscription.ModuleStudio, listingsubscription.EntitlementInput{
		Status: listingsubscription.StatusActive,
		Limits: map[string]int{"product_image_jobs": 3},
	}); err != nil {
		t.Fatalf("UpsertEntitlement() error = %v", err)
	}
	restore := tenantbridge.ConfigureLegacyTenantResolver(studioProductImageUsageLegacyTenantResolver{legacyTenantID: 246})
	t.Cleanup(restore)

	adapter := studioProductImageUsageDependency(svc)
	ctx := context.Background()
	if err := adapter.AuthorizeProductImageUsage(ctx, "org-tenant", 1); err != nil {
		t.Fatalf("AuthorizeProductImageUsage() error = %v", err)
	}
	if err := adapter.ReserveProductImageUsage(ctx, "org-tenant", "candidate-legacy", 1); err != nil {
		t.Fatalf("ReserveProductImageUsage() error = %v", err)
	}
	event, err := svc.GetUsage(ctx, "246", "listingkit:studio_product_image:candidate-legacy")
	if err != nil {
		t.Fatalf("GetUsage(legacy tenant) error = %v", err)
	}
	if event.TenantID != "246" {
		t.Fatalf("event tenant = %q, want legacy billing tenant 246", event.TenantID)
	}
	if err := adapter.CommitProductImageUsage(ctx, "org-tenant", "candidate-legacy"); err != nil {
		t.Fatalf("CommitProductImageUsage() error = %v", err)
	}
	if err := adapter.RecordProductImageUsage(ctx, "org-tenant", 1); err != nil {
		t.Fatalf("RecordProductImageUsage() error = %v", err)
	}
}

func TestSubscriptionStudioProductImageUsageReleaseRetriesLegacyMirrorBeforeLedgerRelease(t *testing.T) {
	baseRepo := listingsubscription.NewMemRepository()
	repo := &failingNegativeProductImageUsageRepository{Repository: baseRepo}
	svc, err := listingsubscription.NewServiceWithLedger(repo, listingsubscription.NewMemUsageLedger(baseRepo))
	if err != nil {
		t.Fatalf("NewServiceWithLedger() error = %v", err)
	}
	if _, err := svc.UpsertEntitlement(context.Background(), "tenant-mirror-retry", listingsubscription.ModuleStudio, listingsubscription.EntitlementInput{
		Status: listingsubscription.StatusActive,
		Limits: map[string]int{"product_image_jobs": 2},
	}); err != nil {
		t.Fatalf("UpsertEntitlement() error = %v", err)
	}
	adapter := studioProductImageUsageDependency(svc)
	ctx := context.Background()
	if err := adapter.ReserveProductImageUsage(ctx, "tenant-mirror-retry", "candidate-1", 1); err != nil {
		t.Fatalf("ReserveProductImageUsage() error = %v", err)
	}
	repo.failNegative = true
	if err := adapter.ReleaseProductImageUsage(ctx, "tenant-mirror-retry", "candidate-1", "test"); err == nil {
		t.Fatal("ReleaseProductImageUsage() unexpectedly succeeded while mirror failed")
	}
	event, err := svc.GetUsage(ctx, "tenant-mirror-retry", "listingkit:studio_product_image:candidate-1")
	if err != nil {
		t.Fatalf("GetUsage() after failed release error = %v", err)
	}
	if event.Status != listingsubscription.UsageEventReserved {
		t.Fatalf("event status = %q after failed mirror, want reserved for retry", event.Status)
	}
	repo.failNegative = false
	if err := adapter.ReleaseProductImageUsage(ctx, "tenant-mirror-retry", "candidate-1", "retry"); err != nil {
		t.Fatalf("ReleaseProductImageUsage() retry error = %v", err)
	}
	event, err = svc.GetUsage(ctx, "tenant-mirror-retry", "listingkit:studio_product_image:candidate-1")
	if err != nil {
		t.Fatalf("GetUsage() after release retry error = %v", err)
	}
	if event.Status != listingsubscription.UsageEventReleased {
		t.Fatalf("event status = %q after release retry, want released", event.Status)
	}
}

func TestSubscriptionStudioProductImageUsageRetriesPendingLegacyMirrorOnExistingReservation(t *testing.T) {
	repo := listingsubscription.NewMemRepository()
	svc, err := listingsubscription.NewServiceWithLedger(repo, listingsubscription.NewMemUsageLedger(repo))
	if err != nil {
		t.Fatalf("NewServiceWithLedger() error = %v", err)
	}
	if _, err := svc.UpsertEntitlement(context.Background(), "tenant-pending-mirror", listingsubscription.ModuleStudio, listingsubscription.EntitlementInput{
		Status: listingsubscription.StatusActive,
		Limits: map[string]int{"product_image_jobs": 2},
	}); err != nil {
		t.Fatalf("UpsertEntitlement() error = %v", err)
	}
	adapter := studioProductImageUsageDependency(svc)
	ctx := context.Background()
	if err := adapter.ReserveProductImageUsage(ctx, "tenant-pending-mirror", "candidate-1", 1); err != nil {
		t.Fatalf("ReserveProductImageUsage() error = %v", err)
	}
	event, err := svc.GetUsage(ctx, "tenant-pending-mirror", "listingkit:studio_product_image:candidate-1")
	if err != nil {
		t.Fatalf("GetUsage() error = %v", err)
	}
	if _, err := svc.RecordUsageForPeriod(ctx, event.TenantID, studioProductImageModule, studioProductImageMetric, event.PeriodKey, -1); err != nil {
		t.Fatalf("RecordUsageForPeriod() reset error = %v", err)
	}
	if _, err := svc.UpdateUsageMetadata(ctx, event.EventID, map[string]string{legacyMirrorMetadataKey: legacyMirrorPending}); err != nil {
		t.Fatalf("UpdateUsageMetadata() error = %v", err)
	}
	if err := adapter.ReserveProductImageUsage(ctx, "tenant-pending-mirror", "candidate-1", 1); err != nil {
		t.Fatalf("ReserveProductImageUsage() pending retry error = %v", err)
	}
	usage, err := repo.ListUsage(ctx, "tenant-pending-mirror")
	if err != nil {
		t.Fatalf("ListUsage() error = %v", err)
	}
	for _, counter := range usage {
		if counter.Metric == studioProductImageMetric && counter.Used != 1 {
			t.Fatalf("legacy mirror usage = %d, want one after pending retry", counter.Used)
		}
	}
}
