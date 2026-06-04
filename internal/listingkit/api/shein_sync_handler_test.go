package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
)

type stubSheinSyncHandlerService struct {
	job           *listingkit.SheinSyncJobRecord
	products      []listingkit.SheinSyncedProductRecord
	total         int64
	syncErr       error
	listErr       error
	updateCostErr error
	syncCtx       context.Context
	syncTenantID  int64
	syncStoreID   int64
	syncTrigger   listingkit.SheinSyncTriggerMode
	listCtx       context.Context
	listQuery     *listingkit.SheinSyncedProductQuery
	updateCostCtx context.Context
	updateCostID  int64
	updateCost    *float64
}

func (s *stubSheinSyncHandlerService) SyncSheinOnShelfProducts(ctx context.Context, tenantID, storeID int64, triggerMode listingkit.SheinSyncTriggerMode) (*listingkit.SheinSyncJobRecord, error) {
	s.syncCtx = ctx
	s.syncTenantID = tenantID
	s.syncStoreID = storeID
	s.syncTrigger = triggerMode
	return s.job, s.syncErr
}

func (s *stubSheinSyncHandlerService) ListSyncedProducts(ctx context.Context, query *listingkit.SheinSyncedProductQuery) ([]listingkit.SheinSyncedProductRecord, int64, error) {
	s.listCtx = ctx
	s.listQuery = query
	return s.products, s.total, s.listErr
}

func (s *stubSheinSyncHandlerService) UpdateManualCostPrice(ctx context.Context, productID int64, manualCostPrice *float64) error {
	s.updateCostCtx = ctx
	s.updateCostID = productID
	s.updateCost = manualCostPrice
	return s.updateCostErr
}

type stubSheinCandidateHandlerService struct{}

func (stubSheinCandidateHandlerService) RefreshCandidates(context.Context, int64, int64, string) (*listingkit.SheinCandidateRefreshResult, error) {
	return nil, nil
}

func (stubSheinCandidateHandlerService) ListCandidates(context.Context, *listingkit.SheinActivityCandidateQuery) ([]listingkit.SheinActivityCandidateRecord, int64, error) {
	return nil, 0, nil
}

func (stubSheinCandidateHandlerService) ReviewCandidate(context.Context, int64, int64, int64, listingkit.SheinCandidateReviewStatus, *bool, *bool) (*listingkit.SheinActivityCandidateRecord, error) {
	return nil, nil
}

type stubSheinEnrollmentHandlerService struct{}

func (stubSheinEnrollmentHandlerService) ExecuteSheinActivityEnrollment(context.Context, int64, int64, string, string, listingkit.SheinEnrollmentRunTriggerMode, ...int64) (*listingkit.SheinActivityEnrollmentRunRecord, error) {
	return nil, nil
}

func (stubSheinEnrollmentHandlerService) ExecuteAutoSheinActivityEnrollment(context.Context, int64, int64, string, string) (*listingkit.SheinActivityEnrollmentRunRecord, error) {
	return nil, nil
}

func TestTriggerSheinStoreSyncReturnsAcceptedJobPayload(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	syncSvc := &stubSheinSyncHandlerService{
		job: &listingkit.SheinSyncJobRecord{
			ID:           12,
			TenantID:     18,
			StoreID:      2001,
			TriggerMode:  listingkit.SheinSyncTriggerModeManual,
			Status:       listingkit.SheinSyncJobStatusSucceeded,
			FetchedCount: 3,
		},
	}
	h, err := NewHandler(
		&stubGenerationTaskService{},
		WithSheinSyncServices(syncSvc, stubSheinCandidateHandlerService{}, stubSheinEnrollmentHandlerService{}),
	)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/shein-sync/stores/:store_id/sync", h.TriggerSheinStoreSync)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/shein-sync/stores/2001/sync", strings.NewReader(`{"trigger_mode":"manual"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "18")
	req.Header.Set("X-User-ID", "shein-ops")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 body=%s", resp.Code, resp.Body.String())
	}
	if syncSvc.syncTenantID != 18 || syncSvc.syncStoreID != 2001 {
		t.Fatalf("sync target = tenant:%d store:%d, want tenant:18 store:2001", syncSvc.syncTenantID, syncSvc.syncStoreID)
	}
	if syncSvc.syncTrigger != listingkit.SheinSyncTriggerModeManual {
		t.Fatalf("trigger mode = %q, want %q", syncSvc.syncTrigger, listingkit.SheinSyncTriggerModeManual)
	}
	if got := listingkit.TenantIDFromContext(syncSvc.syncCtx); got != "18" {
		t.Fatalf("tenant in ctx = %q, want 18", got)
	}
	if got := listingkit.RequestUserIDFromContext(syncSvc.syncCtx); got != "shein-ops" {
		t.Fatalf("user in ctx = %q, want shein-ops", got)
	}

	var body struct {
		Job *listingkit.SheinSyncJobRecord `json:"job"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.Job == nil || body.Job.ID != 12 {
		t.Fatalf("body.job = %+v, want id 12", body.Job)
	}
}

func TestTriggerSheinStoreSyncRejectsNonNumericTenantID(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	h, err := NewHandler(
		&stubGenerationTaskService{},
		WithSheinSyncServices(&stubSheinSyncHandlerService{}, stubSheinCandidateHandlerService{}, stubSheinEnrollmentHandlerService{}),
	)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/shein-sync/stores/:store_id/sync", h.TriggerSheinStoreSync)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/shein-sync/stores/2001/sync", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-red")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 body=%s", resp.Code, resp.Body.String())
	}
}
