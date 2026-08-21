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

	"task-processor/internal/listingkit"
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

func TestReserveStudioProductImageUsageRepairsBatchReleaseMarkerBeforeAuthorization(t *testing.T) {
	ctx := context.Background()
	svc := newStudioProductImageAdmissionService(t, "tenant-batch-release-repair", 1)
	if _, err := svc.RecordUsage(ctx, "tenant-batch-release-repair", listingsubscription.ModuleStudio, "product_image_jobs", 1); err != nil {
		t.Fatalf("RecordUsage() error = %v", err)
	}
	reserved, err := svc.ReserveUsage(ctx, listingsubscription.ReserveUsageInput{
		TenantID: "tenant-batch-release-repair", ModuleCode: listingsubscription.ModuleStudio,
		Metric: studioProductImageLedgerMetric, Quantity: 1, PeriodKey: time.Now().UTC().Format("2006-01"),
		SourceType: studioProductImageSourceType, SourceID: "batch-release", IdempotencyKey: "listingkit:studio_product_image:batch-release",
		OccurredAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("ReserveUsage() error = %v", err)
	}
	if _, err := svc.UpdateUsageMetadata(ctx, reserved.Event.EventID, map[string]string{
		studioProductImageLegacyMirrorMetadataKey:               "settled",
		studioProductImageLegacyMirrorReleasePendingMetadataKey: "1",
	}); err != nil {
		t.Fatalf("UpdateUsageMetadata() error = %v", err)
	}
	if _, err := svc.ReleaseUsage(ctx, reserved.Event.EventID, "batch_release"); err != nil {
		t.Fatalf("ReleaseUsage() error = %v", err)
	}
	h := &handler{subscriptionDependencies: subscriptionDependencies{subscriptionService: svc}}
	if _, err := h.reserveStudioProductImageUsage(newStudioProductImageAdmissionContext("tenant-batch-release-repair"), "direct-request"); err != nil {
		t.Fatalf("reserve error = %v, want batch release repaired before quota authorization", err)
	}
	event, err := svc.GetUsageEventByID(ctx, reserved.Event.EventID)
	if err != nil {
		t.Fatalf("GetUsageEventByID() error = %v", err)
	}
	if event.Metadata[studioProductImageLegacyMirrorReleasePendingMetadataKey] != "" {
		t.Fatalf("release repair marker = %q, want cleared after reconciliation", event.Metadata[studioProductImageLegacyMirrorReleasePendingMetadataKey])
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

func TestReconcileStudioProductImageUsageRecoversAbandonedAsyncJob(t *testing.T) {
	ctx := listingkit.WithTenantID(context.Background(), "tenant-async-recovery")
	svc := newStudioProductImageAdmissionService(t, "tenant-async-recovery", 2)
	repo := listingkit.NewMemStudioAsyncJobRepository()
	jobID := "async-recovery-job"
	old := time.Now().UTC().Add(-2 * time.Hour)
	if err := repo.CreateStudioAsyncJob(ctx, &listingkit.StudioAsyncJobRecord{
		ID: jobID, TenantID: "tenant-async-recovery", Path: "/studio/product-images",
		Status: listingkit.StudioAsyncJobStatusRunning, CreatedAt: old, UpdatedAt: old,
	}); err != nil {
		t.Fatalf("CreateStudioAsyncJob() error = %v", err)
	}
	reserved, err := svc.ReserveUsage(ctx, listingsubscription.ReserveUsageInput{
		TenantID: "tenant-async-recovery", ModuleCode: listingsubscription.ModuleStudio,
		Metric: studioProductImageLedgerMetric, Quantity: 1, PeriodKey: time.Now().UTC().Format("2006-01"),
		SourceType: studioProductImageAsyncSourceType, SourceID: jobID, IdempotencyKey: "listingkit:api:studio_product_image:" + jobID,
		OccurredAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("ReserveUsage() error = %v", err)
	}
	h := &handler{
		subscriptionDependencies: subscriptionDependencies{subscriptionService: svc},
		studioAsyncJobs:          &studioAsyncJobStore{repo: repo},
	}
	if err := h.reconcileStudioProductImageUsageReleases(ctx, "tenant-async-recovery"); err != nil {
		t.Fatalf("reconcile error = %v", err)
	}
	event, err := svc.GetUsageEventByID(ctx, reserved.Event.EventID)
	if err != nil {
		t.Fatalf("GetUsageEventByID() error = %v", err)
	}
	if event.Status != listingsubscription.UsageEventReleased {
		t.Fatalf("event status = %q, want released", event.Status)
	}
	job, ok := h.studioAsyncJobs.get(ctx, jobID)
	if !ok || job.Status != listingkit.StudioAsyncJobStatusFailed {
		t.Fatalf("job = %+v, ok=%v, want failed abandoned job", job, ok)
	}
}

func TestReconcileStudioProductImageUsagePagesPastFullPageOfActiveAsyncReservations(t *testing.T) {
	repo := listingsubscription.NewMemRepository()
	baseLedger := listingsubscription.NewMemUsageLedger(repo)
	pageLedger := &activeAsyncReconciliationPageLedger{
		UsageLedger: baseLedger,
		events:      make([]listingsubscription.UsageEvent, 100),
	}
	for i := range pageLedger.events {
		pageLedger.events[i] = listingsubscription.UsageEvent{
			EventID: fmt.Sprintf("active-event-%03d", i), TenantID: "tenant-active-page",
			ModuleCode: listingsubscription.ModuleStudio, Metric: studioProductImageLedgerMetric,
			Quantity: 1, PeriodKey: time.Now().UTC().Format("2006-01"), SourceType: studioProductImageAsyncSourceType,
			SourceID: "active-job", Status: listingsubscription.UsageEventReserved,
			OccurredAt: time.Now().UTC(), CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
		}
	}
	svc, err := listingsubscription.NewServiceWithLedger(repo, pageLedger)
	if err != nil {
		t.Fatalf("NewServiceWithLedger() error = %v", err)
	}
	jobRepo := listingkit.NewMemStudioAsyncJobRepository()
	now := time.Now().UTC()
	if err := jobRepo.CreateStudioAsyncJob(listingkit.WithTenantID(context.Background(), "tenant-active-page"), &listingkit.StudioAsyncJobRecord{
		ID: "active-job", TenantID: "tenant-active-page", Path: "/studio/product-images",
		Status: listingkit.StudioAsyncJobStatusRunning, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("CreateStudioAsyncJob() error = %v", err)
	}
	if _, err := svc.UpsertEntitlement(context.Background(), "tenant-active-page", listingsubscription.ModuleStudio, listingsubscription.EntitlementInput{
		Status: listingsubscription.StatusActive, Limits: map[string]int{"product_image_jobs": 2},
	}); err != nil {
		t.Fatalf("UpsertEntitlement() error = %v", err)
	}
	reserved, err := svc.ReserveUsage(context.Background(), listingsubscription.ReserveUsageInput{
		TenantID: "tenant-active-page", ModuleCode: listingsubscription.ModuleStudio,
		Metric: studioProductImageLedgerMetric, Quantity: 1, PeriodKey: now.Format("2006-01"),
		SourceType: studioProductImageSourceType, SourceID: "pending-after-active-page",
		IdempotencyKey: "pending-after-active-page", OccurredAt: now,
	})
	if err != nil {
		t.Fatalf("ReserveUsage() error = %v", err)
	}
	if _, err := svc.UpdateUsageMetadata(context.Background(), reserved.Event.EventID, map[string]string{studioProductImageReleasePendingMetadataKey: "1"}); err != nil {
		t.Fatalf("UpdateUsageMetadata() error = %v", err)
	}
	actionable, err := svc.GetUsageEventByID(context.Background(), reserved.Event.EventID)
	if err != nil {
		t.Fatalf("GetUsageEventByID() error = %v", err)
	}
	pageLedger.actionable = *actionable
	h := &handler{
		subscriptionDependencies: subscriptionDependencies{subscriptionService: svc},
		studioAsyncJobs:          &studioAsyncJobStore{repo: jobRepo},
	}
	if err := h.reconcileStudioProductImageUsageReleases(listingkit.WithTenantID(context.Background(), "tenant-active-page"), "tenant-active-page"); err != nil {
		t.Fatalf("reconcile error = %v", err)
	}
	event, err := svc.GetUsageEventByID(context.Background(), reserved.Event.EventID)
	if err != nil {
		t.Fatalf("GetUsageEventByID() error = %v", err)
	}
	if event.Status != listingsubscription.UsageEventReleased {
		t.Fatalf("pending event status = %q, want released after the active page", event.Status)
	}
	if len(pageLedger.offsets) < 2 || pageLedger.offsets[0] != 0 || pageLedger.offsets[1] != 100 {
		t.Fatalf("reconciliation offsets = %v, want to advance past the unchanged full page", pageLedger.offsets)
	}
}

func TestReconcileStudioProductImageUsageLooksUpAsyncJobsByTenant(t *testing.T) {
	ctxOwner := listingkit.WithRequestIdentity(listingkit.WithTenantID(context.Background(), "tenant-cross-user"), listingkit.RequestIdentity{TenantID: "tenant-cross-user", UserID: "user-a"})
	ctxOtherUser := listingkit.WithRequestIdentity(listingkit.WithTenantID(context.Background(), "tenant-cross-user"), listingkit.RequestIdentity{TenantID: "tenant-cross-user", UserID: "user-b"})
	svc := newStudioProductImageAdmissionService(t, "tenant-cross-user", 2)
	jobRepo := listingkit.NewMemStudioAsyncJobRepository()
	now := time.Now().UTC()
	if err := jobRepo.CreateStudioAsyncJob(ctxOwner, &listingkit.StudioAsyncJobRecord{
		ID: "cross-user-job", TenantID: "tenant-cross-user", UserID: "user-a", Path: "/studio/product-images",
		Status: listingkit.StudioAsyncJobStatusRunning, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("CreateStudioAsyncJob() error = %v", err)
	}
	reserved, err := svc.ReserveUsage(ctxOwner, listingsubscription.ReserveUsageInput{
		TenantID: "tenant-cross-user", ModuleCode: listingsubscription.ModuleStudio,
		Metric: studioProductImageLedgerMetric, Quantity: 1, PeriodKey: now.Format("2006-01"),
		SourceType: studioProductImageAsyncSourceType, SourceID: "cross-user-job", IdempotencyKey: "cross-user-job",
		OccurredAt: now,
	})
	if err != nil {
		t.Fatalf("ReserveUsage() error = %v", err)
	}
	h := &handler{
		subscriptionDependencies: subscriptionDependencies{subscriptionService: svc},
		studioAsyncJobs:          &studioAsyncJobStore{repo: jobRepo},
	}
	if err := h.reconcileStudioProductImageUsageReleases(ctxOtherUser, "tenant-cross-user"); err != nil {
		t.Fatalf("reconcile error = %v", err)
	}
	event, err := svc.GetUsageEventByID(ctxOtherUser, reserved.Event.EventID)
	if err != nil {
		t.Fatalf("GetUsageEventByID() error = %v", err)
	}
	if event.Status != listingsubscription.UsageEventReserved {
		t.Fatalf("event status = %q, want active cross-user job to remain reserved", event.Status)
	}
}

func TestReconcileStudioProductImageUsageFailsStaleAsyncJobAcrossUsers(t *testing.T) {
	ctxOwner := listingkit.WithRequestIdentity(listingkit.WithTenantID(context.Background(), "tenant-stale-cross-user"), listingkit.RequestIdentity{TenantID: "tenant-stale-cross-user", UserID: "user-a"})
	ctxOtherUser := listingkit.WithRequestIdentity(listingkit.WithTenantID(context.Background(), "tenant-stale-cross-user"), listingkit.RequestIdentity{TenantID: "tenant-stale-cross-user", UserID: "user-b"})
	svc := newStudioProductImageAdmissionService(t, "tenant-stale-cross-user", 2)
	jobRepo := listingkit.NewMemStudioAsyncJobRepository()
	old := time.Now().UTC().Add(-2 * time.Hour)
	if err := jobRepo.CreateStudioAsyncJob(ctxOwner, &listingkit.StudioAsyncJobRecord{
		ID: "stale-cross-user-job", TenantID: "tenant-stale-cross-user", UserID: "user-a", Path: "/studio/product-images",
		Status: listingkit.StudioAsyncJobStatusRunning, CreatedAt: old, UpdatedAt: old,
	}); err != nil {
		t.Fatalf("CreateStudioAsyncJob() error = %v", err)
	}
	reserved, err := svc.ReserveUsage(ctxOwner, listingsubscription.ReserveUsageInput{
		TenantID: "tenant-stale-cross-user", ModuleCode: listingsubscription.ModuleStudio,
		Metric: studioProductImageLedgerMetric, Quantity: 1, PeriodKey: old.Format("2006-01"),
		SourceType: studioProductImageAsyncSourceType, SourceID: "stale-cross-user-job", IdempotencyKey: "stale-cross-user-job",
		OccurredAt: old,
	})
	if err != nil {
		t.Fatalf("ReserveUsage() error = %v", err)
	}
	h := &handler{
		subscriptionDependencies: subscriptionDependencies{subscriptionService: svc},
		studioAsyncJobs:          &studioAsyncJobStore{repo: jobRepo},
	}
	if err := h.reconcileStudioProductImageUsageReleases(ctxOtherUser, "tenant-stale-cross-user"); err != nil {
		t.Fatalf("reconcile error = %v", err)
	}
	event, err := svc.GetUsageEventByID(ctxOtherUser, reserved.Event.EventID)
	if err != nil {
		t.Fatalf("GetUsageEventByID() error = %v", err)
	}
	if event.Status != listingsubscription.UsageEventReleased {
		t.Fatalf("event status = %q, want released stale reservation", event.Status)
	}
	job, err := jobRepo.GetStudioAsyncJobForTenant(ctxOtherUser, "tenant-stale-cross-user", "stale-cross-user-job")
	if err != nil {
		t.Fatalf("GetStudioAsyncJobForTenant() error = %v", err)
	}
	if job.Status != listingkit.StudioAsyncJobStatusFailed {
		t.Fatalf("job status = %q, want failed", job.Status)
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

type activeAsyncReconciliationPageLedger struct {
	listingsubscription.UsageLedger
	events     []listingsubscription.UsageEvent
	actionable listingsubscription.UsageEvent
	returned   bool
	offsets    []int
}

func (l *activeAsyncReconciliationPageLedger) GetByID(ctx context.Context, eventID string) (listingsubscription.UsageEvent, error) {
	return l.UsageLedger.(listingsubscription.UsageLedgerEventLookup).GetByID(ctx, eventID)
}

func (l *activeAsyncReconciliationPageLedger) UpdateMetadata(ctx context.Context, eventID string, metadata map[string]string) (listingsubscription.UsageEvent, error) {
	return l.UsageLedger.(listingsubscription.UsageLedgerMetadataUpdater).UpdateMetadata(ctx, eventID, metadata)
}

func (l *activeAsyncReconciliationPageLedger) ListEventsPageForReconciliationWithFilter(_ context.Context, _ listingsubscription.UsageLedgerReconciliationFilter, _, offset int) ([]listingsubscription.UsageEvent, error) {
	l.offsets = append(l.offsets, offset)
	switch offset {
	case 0:
		return append([]listingsubscription.UsageEvent(nil), l.events...), nil
	case len(l.events):
		if l.returned {
			return nil, nil
		}
		l.returned = true
		return []listingsubscription.UsageEvent{l.actionable}, nil
	default:
		return nil, fmt.Errorf("unexpected reconciliation offset %d", offset)
	}
}

type canonicalBillingTenantResolverForAdmission struct{}

func (canonicalBillingTenantResolverForAdmission) ResolveLegacyTenantID(_ context.Context, tenantID string) (int64, bool, error) {
	if tenantID == "tenant-canonical" {
		return 246, true, nil
	}
	return 0, false, nil
}

func (canonicalBillingTenantResolverForAdmission) ResolveOrganizationID(_ context.Context, legacyTenantID int64) (string, bool, error) {
	if legacyTenantID == 246 {
		return "tenant-canonical", true, nil
	}
	return "", false, nil
}

func TestReconcileStudioProductImageUsageRecoversCanonicalAsyncJobTenant(t *testing.T) {
	restore := tenantbridge.ConfigureLegacyTenantResolver(canonicalBillingTenantResolverForAdmission{})
	t.Cleanup(restore)
	ctx := context.Background()
	svc := newStudioProductImageAdmissionService(t, "246", 2)
	jobRepo := listingkit.NewMemStudioAsyncJobRepository()
	now := time.Now().UTC()
	if err := jobRepo.CreateStudioAsyncJob(listingkit.WithTenantID(ctx, "tenant-canonical"), &listingkit.StudioAsyncJobRecord{
		ID: "canonical-tenant-job", TenantID: "tenant-canonical", Path: "/studio/product-images",
		Status: listingkit.StudioAsyncJobStatusRunning, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("CreateStudioAsyncJob() error = %v", err)
	}
	reserved, err := svc.ReserveUsage(ctx, listingsubscription.ReserveUsageInput{
		TenantID: "246", ModuleCode: listingsubscription.ModuleStudio,
		Metric: studioProductImageLedgerMetric, Quantity: 1, PeriodKey: now.Format("2006-01"),
		SourceType: studioProductImageAsyncSourceType, SourceID: "canonical-tenant-job",
		IdempotencyKey: "canonical-tenant-job", OccurredAt: now,
	})
	if err != nil {
		t.Fatalf("ReserveUsage() error = %v", err)
	}
	h := &handler{
		subscriptionDependencies: subscriptionDependencies{subscriptionService: svc},
		studioAsyncJobs:          &studioAsyncJobStore{repo: jobRepo},
	}
	if err := h.reconcileStudioProductImageUsageReleases(ctx, "246"); err != nil {
		t.Fatalf("reconcile error = %v", err)
	}
	event, err := svc.GetUsageEventByID(ctx, reserved.Event.EventID)
	if err != nil {
		t.Fatalf("GetUsageEventByID() error = %v", err)
	}
	if event.Status != listingsubscription.UsageEventReserved {
		t.Fatalf("event status = %q, want reserved for fresh canonical-tenant job", event.Status)
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
