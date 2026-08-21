package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingsubscription"
	"task-processor/internal/tenantbridge"
)

func newStudioProductImageAdmissionContext(tenantID string) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest("POST", "/studio/product-images", nil)
	req.Header.Set("X-Tenant-ID", tenantID)
	c.Request = req
	return c
}

func newStudioProductImageAdmissionService(t *testing.T, tenantID string, limit int) *listingsubscription.Service {
	t.Helper()
	repo := listingsubscription.NewMemRepository()
	svc, err := listingsubscription.NewServiceWithLedger(repo, listingsubscription.NewMemUsageLedger(repo))
	if err != nil {
		t.Fatalf("NewServiceWithLedger() error = %v", err)
	}
	if _, err := svc.UpsertEntitlement(context.Background(), tenantID, listingsubscription.ModuleStudio, listingsubscription.EntitlementInput{
		Status: listingsubscription.StatusActive,
		Limits: map[string]int{"product_image_jobs": limit},
	}); err != nil {
		t.Fatalf("UpsertEntitlement() error = %v", err)
	}
	return svc
}

func TestReserveStudioProductImageUsageIncludesLegacyAggregateUsage(t *testing.T) {
	svc := newStudioProductImageAdmissionService(t, "tenant-pre-ledger", 2)
	if _, err := svc.RecordUsage(context.Background(), "tenant-pre-ledger", listingsubscription.ModuleStudio, "product_image_jobs", 2); err != nil {
		t.Fatalf("RecordUsage() error = %v", err)
	}
	h := &handler{subscriptionDependencies: subscriptionDependencies{subscriptionService: svc}}
	_, err := h.reserveStudioProductImageUsage(newStudioProductImageAdmissionContext("tenant-pre-ledger"), "request-1")
	if !errors.Is(err, listingsubscription.ErrSubscriptionQuotaExceed) {
		t.Fatalf("reserve error = %v, want legacy quota exceeded", err)
	}
}

func TestWriteStudioProductImageUsageAdmissionErrorMapsLegacyQuotaToPaymentRequired(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	writeStudioProductImageUsageAdmissionError(c, listingsubscription.ErrSubscriptionQuotaExceed)
	if recorder.Code != http.StatusPaymentRequired {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusPaymentRequired)
	}
	if !strings.Contains(recorder.Body.String(), `"error":"quota_exceeded"`) {
		t.Fatalf("body = %s, want quota_exceeded", recorder.Body.String())
	}
}

func TestReserveStudioProductImageUsagePropagatesLegacyBridgeFailure(t *testing.T) {
	repo := listingsubscription.NewMemRepository()
	svc, err := listingsubscription.NewServiceWithLedger(repo, listingsubscription.NewMemUsageLedger(repo))
	if err != nil {
		t.Fatalf("NewServiceWithLedger() error = %v", err)
	}
	restore := tenantbridge.ConfigureLegacyTenantResolver(failingStudioProductImageUsageLegacyTenantResolverForAdmission{})
	t.Cleanup(restore)
	h := &handler{subscriptionDependencies: subscriptionDependencies{subscriptionService: svc}}
	_, err = h.reserveStudioProductImageUsage(newStudioProductImageAdmissionContext("tenant-bridge-error"), "request-1")
	if err == nil || !strings.Contains(err.Error(), "metadata database unavailable") {
		t.Fatalf("reserve error = %v, want bridge failure", err)
	}
}

func TestReconcileStudioProductImageUsageReleasesPendingEvent(t *testing.T) {
	svc := newStudioProductImageAdmissionService(t, "tenant-release-retry", 2)
	ctx := context.Background()
	reserved, err := svc.ReserveUsage(ctx, listingsubscription.ReserveUsageInput{
		TenantID:       "tenant-release-retry",
		ModuleCode:     listingsubscription.ModuleStudio,
		Metric:         studioProductImageLedgerMetric,
		Quantity:       1,
		PeriodKey:      time.Now().UTC().Format("2006-01"),
		SourceType:     "listingkit_product_image",
		SourceID:       "request-1",
		IdempotencyKey: "listingkit:api:studio_product_image:request-1",
		OccurredAt:     time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("ReserveUsage() error = %v", err)
	}
	if _, err := svc.UpdateUsageMetadata(ctx, reserved.Event.EventID, map[string]string{studioProductImageReleasePendingMetadataKey: "1"}); err != nil {
		t.Fatalf("UpdateUsageMetadata() error = %v", err)
	}
	h := &handler{subscriptionDependencies: subscriptionDependencies{subscriptionService: svc}}
	if err := h.reconcileStudioProductImageUsageReleases(ctx, "tenant-release-retry"); err != nil {
		t.Fatalf("reconcile error = %v", err)
	}
	event, err := svc.GetUsageEventByID(ctx, reserved.Event.EventID)
	if err != nil {
		t.Fatalf("GetUsageEventByID() error = %v", err)
	}
	if event.Status != listingsubscription.UsageEventReleased {
		t.Fatalf("event status = %q, want released", event.Status)
	}
}

func TestStudioProductImageUsageReleaseContextIsDetached(t *testing.T) {
	c := newStudioProductImageAdmissionContext("tenant-detached-release")
	requestCtx, cancel := context.WithCancel(c.Request.Context())
	c.Request = c.Request.WithContext(requestCtx)
	detached := studioProductImageUsageReleaseContext(c)
	cancel()
	select {
	case <-detached.Done():
		t.Fatal("release context inherited request cancellation")
	default:
	}
}

func TestReconcileStudioProductImageUsagePagesPendingEvents(t *testing.T) {
	svc := newStudioProductImageAdmissionService(t, "tenant-release-pages", 200)
	ctx := context.Background()
	for i := 0; i < 101; i++ {
		id := fmt.Sprintf("request-%03d", i)
		reserved, err := svc.ReserveUsage(ctx, listingsubscription.ReserveUsageInput{
			TenantID:       "tenant-release-pages",
			ModuleCode:     listingsubscription.ModuleStudio,
			Metric:         studioProductImageLedgerMetric,
			Quantity:       1,
			PeriodKey:      time.Now().UTC().Format("2006-01"),
			SourceType:     "listingkit_product_image",
			SourceID:       id,
			IdempotencyKey: "listingkit:api:studio_product_image:" + id,
			OccurredAt:     time.Now().UTC(),
		})
		if err != nil {
			t.Fatalf("ReserveUsage(%s) error = %v", id, err)
		}
		if _, err := svc.UpdateUsageMetadata(ctx, reserved.Event.EventID, map[string]string{studioProductImageReleasePendingMetadataKey: "1"}); err != nil {
			t.Fatalf("UpdateUsageMetadata(%s) error = %v", id, err)
		}
	}
	h := &handler{subscriptionDependencies: subscriptionDependencies{subscriptionService: svc}}
	if err := h.reconcileStudioProductImageUsageReleases(ctx, "tenant-release-pages"); err != nil {
		t.Fatalf("reconcile error = %v", err)
	}
	events, err := svc.ListUsageEvents(ctx, 200)
	if err != nil {
		t.Fatalf("ListUsageEvents() error = %v", err)
	}
	for _, event := range events {
		if event.SourceType == "listingkit_product_image" && event.Metric == studioProductImageLedgerMetric && event.Status != listingsubscription.UsageEventReleased {
			t.Fatalf("event %s status = %q, want released", event.EventID, event.Status)
		}
	}
}

func TestReconcileStudioProductImageUsageIsTenantScoped(t *testing.T) {
	ctx := context.Background()
	svc := newStudioProductImageAdmissionService(t, "tenant-release-a", 2)
	if _, err := svc.UpsertEntitlement(ctx, "tenant-release-b", listingsubscription.ModuleStudio, listingsubscription.EntitlementInput{
		Status: listingsubscription.StatusActive,
		Limits: map[string]int{"product_image_jobs": 2},
	}); err != nil {
		t.Fatalf("UpsertEntitlement() tenant B error = %v", err)
	}
	for _, tenantID := range []string{"tenant-release-a", "tenant-release-b"} {
		reserved, err := svc.ReserveUsage(ctx, listingsubscription.ReserveUsageInput{
			TenantID:       tenantID,
			ModuleCode:     listingsubscription.ModuleStudio,
			Metric:         studioProductImageLedgerMetric,
			Quantity:       1,
			PeriodKey:      time.Now().UTC().Format("2006-01"),
			SourceType:     "listingkit_product_image",
			SourceID:       tenantID,
			IdempotencyKey: "listingkit:api:studio_product_image:" + tenantID,
			OccurredAt:     time.Now().UTC(),
		})
		if err != nil {
			t.Fatalf("ReserveUsage(%s) error = %v", tenantID, err)
		}
		if _, err := svc.UpdateUsageMetadata(ctx, reserved.Event.EventID, map[string]string{studioProductImageReleasePendingMetadataKey: "1"}); err != nil {
			t.Fatalf("UpdateUsageMetadata(%s) error = %v", tenantID, err)
		}
	}
	h := &handler{subscriptionDependencies: subscriptionDependencies{subscriptionService: svc}}
	if err := h.reconcileStudioProductImageUsageReleases(ctx, "tenant-release-b"); err != nil {
		t.Fatalf("tenant-scoped reconcile error = %v", err)
	}
	for _, tenantID := range []string{"tenant-release-a", "tenant-release-b"} {
		event, err := svc.GetUsage(ctx, tenantID, "listingkit:api:studio_product_image:"+tenantID)
		if err != nil {
			t.Fatalf("GetUsage(%s) error = %v", tenantID, err)
		}
		want := listingsubscription.UsageEventReserved
		if tenantID == "tenant-release-b" {
			want = listingsubscription.UsageEventReleased
		}
		if event.Status != want {
			t.Fatalf("tenant %s event status = %q, want %q", tenantID, event.Status, want)
		}
	}
}

func TestCommitStudioProductImageUsageMirrorsLegacyCounter(t *testing.T) {
	svc := newStudioProductImageAdmissionService(t, "tenant-legacy-mirror", 2)
	ctx := context.Background()
	reserved, err := svc.ReserveUsage(ctx, listingsubscription.ReserveUsageInput{
		TenantID:       "tenant-legacy-mirror",
		ModuleCode:     listingsubscription.ModuleStudio,
		Metric:         studioProductImageLedgerMetric,
		Quantity:       1,
		PeriodKey:      "2026-08",
		SourceType:     "listingkit_product_image",
		SourceID:       "request-1",
		IdempotencyKey: "listingkit:api:studio_product_image:request-1",
		OccurredAt:     time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("ReserveUsage() error = %v", err)
	}
	if err := commitStudioProductImageUsage(ctx, svc, reserved.Event.EventID); err != nil {
		t.Fatalf("commitStudioProductImageUsage() error = %v", err)
	}
	summary, err := svc.GetSummary(ctx, "tenant-legacy-mirror")
	if err != nil {
		t.Fatalf("GetSummary() error = %v", err)
	}
	for _, entitlement := range summary.Entitlements {
		if entitlement.Module.Code == listingsubscription.ModuleStudio {
			if got := entitlement.Used["product_image_jobs"]; got != 1 {
				t.Fatalf("legacy product_image_jobs usage = %d, want 1", got)
			}
			return
		}
	}
	t.Fatal("studio entitlement missing from summary")
}

type failingPositiveStudioProductImageUsageRepository struct {
	listingsubscription.Repository
}

func (r *failingPositiveStudioProductImageUsageRepository) IncrementUsageOnce(ctx context.Context, tenantID, moduleCode, periodKey, metric string, amount int, operationKey string) (*listingsubscription.UsageCounter, bool, error) {
	if amount > 0 {
		return nil, false, errors.New("legacy mirror temporarily unavailable")
	}
	repo, ok := r.Repository.(listingsubscription.UsageCounterIdempotencyRepository)
	if !ok {
		return nil, false, listingsubscription.ErrUsageCounterIdempotencyUnsupported
	}
	return repo.IncrementUsageOnce(ctx, tenantID, moduleCode, periodKey, metric, amount, operationKey)
}

func TestCommitStudioProductImageUsageDoesNotFailAfterLedgerCommitWhenMirrorFails(t *testing.T) {
	baseRepo := listingsubscription.NewMemRepository()
	repo := &failingPositiveStudioProductImageUsageRepository{Repository: baseRepo}
	svc, err := listingsubscription.NewServiceWithLedger(repo, listingsubscription.NewMemUsageLedger(baseRepo))
	if err != nil {
		t.Fatalf("NewServiceWithLedger() error = %v", err)
	}
	ctx := context.Background()
	if _, err := svc.UpsertEntitlement(ctx, "tenant-mirror-failure", listingsubscription.ModuleStudio, listingsubscription.EntitlementInput{
		Status: listingsubscription.StatusActive,
		Limits: map[string]int{"product_image_jobs": 2},
	}); err != nil {
		t.Fatalf("UpsertEntitlement() error = %v", err)
	}
	reserved, err := svc.ReserveUsage(ctx, listingsubscription.ReserveUsageInput{
		TenantID:       "tenant-mirror-failure",
		ModuleCode:     listingsubscription.ModuleStudio,
		Metric:         studioProductImageLedgerMetric,
		Quantity:       1,
		PeriodKey:      "2026-08",
		SourceType:     "listingkit_product_image",
		SourceID:       "request-1",
		IdempotencyKey: "listingkit:api:studio_product_image:request-1",
		OccurredAt:     time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("ReserveUsage() error = %v", err)
	}
	if err := commitStudioProductImageUsage(ctx, svc, reserved.Event.EventID); err != nil {
		t.Fatalf("commitStudioProductImageUsage() error = %v, want nil after durable commit", err)
	}
	event, err := svc.GetUsageEventByID(ctx, reserved.Event.EventID)
	if err != nil {
		t.Fatalf("GetUsageEventByID() error = %v", err)
	}
	if event.Status != listingsubscription.UsageEventCommitted {
		t.Fatalf("event status = %q, want committed", event.Status)
	}
}

type failingStudioProductImageUsageLegacyTenantResolverForAdmission struct{}

func (failingStudioProductImageUsageLegacyTenantResolverForAdmission) ResolveLegacyTenantID(context.Context, string) (int64, bool, error) {
	return 0, false, errors.New("metadata database unavailable")
}
