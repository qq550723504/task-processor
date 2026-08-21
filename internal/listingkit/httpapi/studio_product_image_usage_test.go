package httpapi

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"task-processor/internal/listingsubscription"
	"task-processor/internal/tenantbridge"
)

type studioProductImageUsageLegacyTenantResolver struct {
	legacyTenantID int64
}

func (r studioProductImageUsageLegacyTenantResolver) ResolveLegacyTenantID(context.Context, string) (int64, bool, error) {
	return r.legacyTenantID, true, nil
}

type failingStudioProductImageUsageLegacyTenantResolver struct{}

func (failingStudioProductImageUsageLegacyTenantResolver) ResolveLegacyTenantID(context.Context, string) (int64, bool, error) {
	return 0, false, errors.New("legacy tenant bridge temporarily unavailable")
}

type failingNegativeProductImageUsageRepository struct {
	listingsubscription.Repository
	failNegative bool
}

type failingSettledUsageLedger struct {
	listingsubscription.UsageLedger
	failSettled bool
}

func (l *failingSettledUsageLedger) UpdateMetadata(ctx context.Context, eventID string, metadata map[string]string) (listingsubscription.UsageEvent, error) {
	if l.failSettled && metadata[legacyMirrorMetadataKey] == legacyMirrorSettled {
		l.failSettled = false
		return listingsubscription.UsageEvent{}, errors.New("metadata persistence temporarily unavailable")
	}
	updater, ok := l.UsageLedger.(listingsubscription.UsageLedgerMetadataUpdater)
	if !ok {
		return listingsubscription.UsageEvent{}, listingsubscription.ErrUsageLedgerMetadataUnsupported
	}
	return updater.UpdateMetadata(ctx, eventID, metadata)
}

func (r *failingNegativeProductImageUsageRepository) IncrementUsage(ctx context.Context, tenantID, moduleCode, periodKey, metric string, amount int) (*listingsubscription.UsageCounter, error) {
	if r.failNegative && amount < 0 {
		return nil, errors.New("legacy usage mirror temporarily unavailable")
	}
	return r.Repository.IncrementUsage(ctx, tenantID, moduleCode, periodKey, metric, amount)
}

func (r *failingNegativeProductImageUsageRepository) IncrementUsageOnce(ctx context.Context, tenantID, moduleCode, periodKey, metric string, amount int, operationKey string) (*listingsubscription.UsageCounter, bool, error) {
	if r.failNegative && amount < 0 {
		return nil, false, errors.New("legacy usage mirror temporarily unavailable")
	}
	repo, ok := r.Repository.(listingsubscription.UsageCounterIdempotencyRepository)
	if !ok {
		return nil, false, listingsubscription.ErrUsageCounterIdempotencyUnsupported
	}
	return repo.IncrementUsageOnce(ctx, tenantID, moduleCode, periodKey, metric, amount, operationKey)
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

func TestSubscriptionStudioProductImageUsageAllowsSecondActiveMirroredReservation(t *testing.T) {
	t.Parallel()

	repo := listingsubscription.NewMemRepository()
	svc, err := listingsubscription.NewServiceWithLedger(repo, listingsubscription.NewMemUsageLedger(repo))
	if err != nil {
		t.Fatalf("NewServiceWithLedger() error = %v", err)
	}
	if _, err := svc.UpsertEntitlement(context.Background(), "tenant-active-mirror", listingsubscription.ModuleStudio, listingsubscription.EntitlementInput{
		Status: listingsubscription.StatusActive,
		Limits: map[string]int{"product_image_jobs": 2},
	}); err != nil {
		t.Fatalf("UpsertEntitlement() error = %v", err)
	}
	adapter := studioProductImageUsageDependency(svc)
	if err := adapter.ReserveProductImageUsage(context.Background(), "tenant-active-mirror", "candidate-1", 1); err != nil {
		t.Fatalf("first ReserveProductImageUsage() error = %v", err)
	}
	if err := adapter.ReserveProductImageUsage(context.Background(), "tenant-active-mirror", "candidate-2", 1); err != nil {
		t.Fatalf("second ReserveProductImageUsage() error = %v, want active mirrored reservation counted once", err)
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

func TestSubscriptionStudioProductImageUsagePropagatesLegacyTenantResolutionFailure(t *testing.T) {
	repo := listingsubscription.NewMemRepository()
	svc, err := listingsubscription.NewServiceWithLedger(repo, listingsubscription.NewMemUsageLedger(repo))
	if err != nil {
		t.Fatalf("NewServiceWithLedger() error = %v", err)
	}
	restore := tenantbridge.ConfigureLegacyTenantResolver(failingStudioProductImageUsageLegacyTenantResolver{})
	t.Cleanup(restore)
	adapter := studioProductImageUsageDependency(svc)
	err = adapter.CommitProductImageUsage(context.Background(), "org-tenant", "missing-reservation")
	if err == nil || !strings.Contains(err.Error(), "legacy tenant bridge temporarily unavailable") {
		t.Fatalf("CommitProductImageUsage() error = %v, want tenant bridge failure", err)
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
	if event.Status != listingsubscription.UsageEventReleased {
		t.Fatalf("event status = %q after failed mirror, want released with retryable mirror", event.Status)
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

func TestSubscriptionStudioProductImageUsageAuthorizationPropagatesLegacyTenantResolutionFailure(t *testing.T) {
	repo := listingsubscription.NewMemRepository()
	svc, err := listingsubscription.NewServiceWithLedger(repo, listingsubscription.NewMemUsageLedger(repo))
	if err != nil {
		t.Fatalf("NewServiceWithLedger() error = %v", err)
	}
	restore := tenantbridge.ConfigureLegacyTenantResolver(failingStudioProductImageUsageLegacyTenantResolver{})
	t.Cleanup(restore)
	adapter := studioProductImageUsageDependency(svc)
	err = adapter.AuthorizeProductImageUsage(context.Background(), "org-tenant", 1)
	if err == nil || !strings.Contains(err.Error(), "legacy tenant bridge temporarily unavailable") {
		t.Fatalf("AuthorizeProductImageUsage() error = %v, want tenant bridge failure", err)
	}
}

func TestSubscriptionStudioProductImageUsageReplaysExistingIdempotentSettlementWithoutAuthorization(t *testing.T) {
	repo := listingsubscription.NewMemRepository()
	svc, err := listingsubscription.NewService(repo)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	ctx := context.Background()
	if _, err := svc.UpsertEntitlement(ctx, "246", listingsubscription.ModuleStudio, listingsubscription.EntitlementInput{
		Status: listingsubscription.StatusActive,
		Limits: map[string]int{"product_image_jobs": 1},
	}); err != nil {
		t.Fatalf("UpsertEntitlement() error = %v", err)
	}
	restore := tenantbridge.ConfigureLegacyTenantResolver(studioProductImageUsageLegacyTenantResolver{legacyTenantID: 246})
	t.Cleanup(restore)
	adapter := studioProductImageUsageDependency(svc)
	if err := adapter.RecordProductImageUsageOnce(ctx, "org-tenant", 1, "operation-replay"); err != nil {
		t.Fatalf("initial RecordProductImageUsageOnce() error = %v", err)
	}
	restore()
	failureRestore := tenantbridge.ConfigureLegacyTenantResolver(failingStudioProductImageUsageLegacyTenantResolver{})
	t.Cleanup(failureRestore)
	if err := adapter.RecordProductImageUsageOnce(ctx, "org-tenant", 1, "operation-replay"); err != nil {
		t.Fatalf("replayed RecordProductImageUsageOnce() error = %v, want existing operation replay", err)
	}
}

func TestSubscriptionStudioProductImageUsageReconcilesPendingMirrorBeforeRelease(t *testing.T) {
	repo := listingsubscription.NewMemRepository()
	baseLedger := listingsubscription.NewMemUsageLedger(repo)
	ledger := &failingSettledUsageLedger{UsageLedger: baseLedger, failSettled: true}
	svc, err := listingsubscription.NewServiceWithLedger(repo, ledger)
	if err != nil {
		t.Fatalf("NewServiceWithLedger() error = %v", err)
	}
	if _, err := svc.UpsertEntitlement(context.Background(), "tenant-pending-release", listingsubscription.ModuleStudio, listingsubscription.EntitlementInput{
		Status: listingsubscription.StatusActive,
		Limits: map[string]int{"product_image_jobs": 2},
	}); err != nil {
		t.Fatalf("UpsertEntitlement() error = %v", err)
	}
	adapter := studioProductImageUsageDependency(svc)
	ctx := context.Background()
	if err := adapter.ReserveProductImageUsage(ctx, "tenant-pending-release", "candidate-1", 1); err == nil {
		t.Fatal("ReserveProductImageUsage() unexpectedly succeeded while metadata persistence failed")
	}
	if err := adapter.ReleaseProductImageUsage(ctx, "tenant-pending-release", "candidate-1", "generation_failed"); err != nil {
		t.Fatalf("ReleaseProductImageUsage() error = %v", err)
	}
	event, err := svc.GetUsage(ctx, "tenant-pending-release", studioProductImageUsageIdempotencyKey("candidate-1"))
	if err != nil {
		t.Fatalf("GetUsage() error = %v", err)
	}
	if event.Status != listingsubscription.UsageEventReleased {
		t.Fatalf("event status = %q, want released", event.Status)
	}
	usage, err := repo.ListUsage(ctx, "tenant-pending-release")
	if err != nil {
		t.Fatalf("ListUsage() error = %v", err)
	}
	for _, counter := range usage {
		if counter.Metric == studioProductImageMetric && counter.Used != 0 {
			t.Fatalf("legacy mirror usage = %d, want zero after reconciled release", counter.Used)
		}
	}
}

func TestSubscriptionStudioProductImageUsageRetriesLegacyMirrorWhenMetadataIsAbsent(t *testing.T) {
	repo := listingsubscription.NewMemRepository()
	svc, err := listingsubscription.NewServiceWithLedger(repo, listingsubscription.NewMemUsageLedger(repo))
	if err != nil {
		t.Fatalf("NewServiceWithLedger() error = %v", err)
	}
	if _, err := svc.UpsertEntitlement(context.Background(), "tenant-absent-mirror", listingsubscription.ModuleStudio, listingsubscription.EntitlementInput{
		Status: listingsubscription.StatusActive,
		Limits: map[string]int{"product_image_jobs": 2},
	}); err != nil {
		t.Fatalf("UpsertEntitlement() error = %v", err)
	}
	ctx := context.Background()
	if _, err := svc.ReserveUsage(ctx, listingsubscription.ReserveUsageInput{
		TenantID:       "tenant-absent-mirror",
		ModuleCode:     listingsubscription.ModuleStudio,
		Metric:         studioProductImageLedgerMetric,
		Quantity:       1,
		PeriodKey:      time.Now().UTC().Format("2006-01"),
		SourceType:     "listingkit_product_image",
		SourceID:       "candidate-absent",
		IdempotencyKey: studioProductImageUsageIdempotencyKey("candidate-absent"),
		OccurredAt:     time.Now().UTC(),
	}); err != nil {
		t.Fatalf("ReserveUsage() error = %v", err)
	}
	adapter := studioProductImageUsageDependency(svc)
	if err := adapter.ReserveProductImageUsage(ctx, "tenant-absent-mirror", "candidate-absent", 1); err != nil {
		t.Fatalf("ReserveProductImageUsage() error = %v", err)
	}
	usage, err := repo.ListUsage(ctx, "tenant-absent-mirror")
	if err != nil {
		t.Fatalf("ListUsage() error = %v", err)
	}
	found := false
	for _, counter := range usage {
		if counter.Metric == studioProductImageMetric && counter.Used != 1 {
			t.Fatalf("legacy mirror usage = %d, want one after absent metadata retry", counter.Used)
		}
		if counter.Metric == studioProductImageMetric {
			found = true
		}
	}
	if !found {
		t.Fatal("legacy mirror counter missing after absent metadata retry")
	}
}

func TestSubscriptionStudioProductImageUsageDoesNotReleaseAbsentLegacyMirror(t *testing.T) {
	repo := listingsubscription.NewMemRepository()
	svc, err := listingsubscription.NewServiceWithLedger(repo, listingsubscription.NewMemUsageLedger(repo))
	if err != nil {
		t.Fatalf("NewServiceWithLedger() error = %v", err)
	}
	if _, err := svc.UpsertEntitlement(context.Background(), "tenant-absent-release", listingsubscription.ModuleStudio, listingsubscription.EntitlementInput{
		Status: listingsubscription.StatusActive,
		Limits: map[string]int{"product_image_jobs": 2},
	}); err != nil {
		t.Fatalf("UpsertEntitlement() error = %v", err)
	}
	ctx := context.Background()
	if _, err := svc.ReserveUsage(ctx, listingsubscription.ReserveUsageInput{
		TenantID:       "tenant-absent-release",
		ModuleCode:     listingsubscription.ModuleStudio,
		Metric:         studioProductImageLedgerMetric,
		Quantity:       1,
		PeriodKey:      time.Now().UTC().Format("2006-01"),
		SourceType:     "listingkit_product_image",
		SourceID:       "candidate-absent-release",
		IdempotencyKey: studioProductImageUsageIdempotencyKey("candidate-absent-release"),
		OccurredAt:     time.Now().UTC(),
	}); err != nil {
		t.Fatalf("ReserveUsage() error = %v", err)
	}
	adapter := studioProductImageUsageDependency(svc)
	if err := adapter.ReleaseProductImageUsage(ctx, "tenant-absent-release", "candidate-absent-release", "test"); err != nil {
		t.Fatalf("ReleaseProductImageUsage() error = %v", err)
	}
	usage, err := repo.ListUsage(ctx, "tenant-absent-release")
	if err != nil {
		t.Fatalf("ListUsage() error = %v", err)
	}
	for _, counter := range usage {
		if counter.Metric == studioProductImageMetric && counter.Used != 0 {
			t.Fatalf("legacy mirror usage = %d, want zero when mirror metadata is absent", counter.Used)
		}
	}
}
