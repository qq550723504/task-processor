package api

import (
	"context"
	"errors"
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
	if err := h.reconcileStudioProductImageUsageReleases(ctx); err != nil {
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

type failingStudioProductImageUsageLegacyTenantResolverForAdmission struct{}

func (failingStudioProductImageUsageLegacyTenantResolverForAdmission) ResolveLegacyTenantID(context.Context, string) (int64, bool, error) {
	return 0, false, errors.New("metadata database unavailable")
}
