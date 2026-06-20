package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/store"
	"task-processor/internal/tenantbridge"
)

type stubLegacyTenantResolver struct {
	mapping map[string]int64
}

func (s stubLegacyTenantResolver) ResolveLegacyTenantID(_ context.Context, tenantID string) (int64, bool, error) {
	value, ok := s.mapping[strings.TrimSpace(tenantID)]
	return value, ok, nil
}

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

type stubSheinSummaryStoreRepository struct {
	stores []listingadmin.Store
}

func (s *stubSheinSummaryStoreRepository) ListStores(_ context.Context, query listingadmin.StoreQuery) (*listingadmin.StorePage, error) {
	items := make([]listingadmin.Store, 0)
	for _, item := range s.stores {
		if query.TenantID > 0 && item.TenantID != query.TenantID {
			continue
		}
		if query.Platform != "" && item.Platform != query.Platform {
			continue
		}
		items = append(items, item)
	}
	return &listingadmin.StorePage{
		Items:    items,
		Total:    int64(len(items)),
		Page:     1,
		PageSize: len(items),
	}, nil
}

func (s *stubSheinSummaryStoreRepository) GetStore(_ context.Context, tenantID, id int64) (*listingadmin.Store, error) {
	for _, item := range s.stores {
		if item.TenantID == tenantID && item.ID == id {
			storeCopy := item
			return &storeCopy, nil
		}
	}
	return nil, listingadmin.ErrStoreNotFound
}

func (*stubSheinSummaryStoreRepository) CreateStore(context.Context, *listingadmin.Store) (*listingadmin.Store, error) {
	panic("unexpected call")
}
func (*stubSheinSummaryStoreRepository) UpdateStore(context.Context, *listingadmin.Store) (*listingadmin.Store, error) {
	panic("unexpected call")
}
func (*stubSheinSummaryStoreRepository) UpdateStoreID(context.Context, int64, string) (*listingadmin.Store, error) {
	panic("unexpected call")
}
func (*stubSheinSummaryStoreRepository) UpdateStoreStatus(context.Context, int64, int64, int16, string) (*listingadmin.Store, error) {
	panic("unexpected call")
}
func (*stubSheinSummaryStoreRepository) DeleteStore(context.Context, int64, int64) error {
	panic("unexpected call")
}
func (*stubSheinSummaryStoreRepository) ListDeletedStores(context.Context, int64) ([]listingadmin.Store, error) {
	panic("unexpected call")
}
func (*stubSheinSummaryStoreRepository) RestoreStore(context.Context, int64, int64) (*listingadmin.Store, error) {
	panic("unexpected call")
}
func (*stubSheinSummaryStoreRepository) PermanentlyDeleteStore(context.Context, int64, int64) error {
	panic("unexpected call")
}
func (*stubSheinSummaryStoreRepository) ExtendStoreValidity(context.Context, int64, int64, int) (*listingadmin.Store, error) {
	panic("unexpected call")
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
		&stubHandlerCoreService{},
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
		&stubHandlerCoreService{},
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

func TestListSheinEnrollmentDashboardReturnsAggregatedStats(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	enableAutoListing := true
	storeRepo := &stubSheinSummaryStoreRepository{
		stores: []listingadmin.Store{
			{
				ID:                2001,
				TenantID:          18,
				Name:              "SHEIN US",
				Username:          "shein-us",
				Platform:          "SHEIN",
				Region:            "US",
				EnableAutoListing: &enableAutoListing,
			},
		},
	}
	syncRepo := store.NewMemSheinSyncRepository()
	now := time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC)
	if err := syncRepo.UpsertSyncedProducts(context.Background(), []*listingkit.SheinSyncedProductRecord{
		{TenantID: 18, StoreID: 2001, SKCName: "skc-a", IsActive: true, ManualCostPrice: float64Ptr(19.9)},
		{TenantID: 18, StoreID: 2001, SKCName: "skc-b", IsActive: true},
	}); err != nil {
		t.Fatalf("seed synced products: %v", err)
	}
	if err := syncRepo.SaveCandidates(context.Background(), []*listingkit.SheinActivityCandidateRecord{
		{
			TenantID:          18,
			StoreID:           2001,
			ActivityType:      "PROMOTION",
			ActivityKey:       "PROMOTION:18:2001",
			SKCName:           "skc-a",
			CandidateVersion:  "v1",
			EligibilityStatus: listingkit.SheinCandidateEligibilityStatusEligible,
			ReviewStatus:      listingkit.SheinCandidateReviewStatusApproved,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		{
			TenantID:          18,
			StoreID:           2001,
			ActivityType:      "PROMOTION",
			ActivityKey:       "PROMOTION:18:2001",
			SKCName:           "skc-b",
			CandidateVersion:  "v1",
			EligibilityStatus: listingkit.SheinCandidateEligibilityStatusEligible,
			ReviewStatus:      listingkit.SheinCandidateReviewStatusPendingReview,
			CreatedAt:         now.Add(time.Minute),
			UpdatedAt:         now.Add(time.Minute),
		},
	}); err != nil {
		t.Fatalf("seed candidates: %v", err)
	}
	if err := syncRepo.SaveSyncJob(context.Background(), &listingkit.SheinSyncJobRecord{
		TenantID:    18,
		StoreID:     2001,
		TriggerMode: listingkit.SheinSyncTriggerModeManual,
		Status:      listingkit.SheinSyncJobStatusSucceeded,
		StartedAt:   sheinTimePtr(now),
		FinishedAt:  sheinTimePtr(now.Add(2 * time.Minute)),
	}); err != nil {
		t.Fatalf("seed sync job: %v", err)
	}
	if err := syncRepo.CreateEnrollmentRun(context.Background(), &listingkit.SheinActivityEnrollmentRunRecord{
		TenantID:     18,
		StoreID:      2001,
		ActivityType: "PROMOTION",
		ActivityKey:  "PROMOTION:18:2001",
		TriggerMode:  listingkit.SheinEnrollmentRunTriggerModeManualConfirmed,
		Status:       listingkit.SheinEnrollmentRunStatusSucceeded,
		StartedAt:    sheinTimePtr(now.Add(3 * time.Minute)),
		FinishedAt:   sheinTimePtr(now.Add(4 * time.Minute)),
	}); err != nil {
		t.Fatalf("seed enrollment run: %v", err)
	}

	h, err := NewHandler(
		&stubHandlerCoreService{},
		WithStoreRepository(storeRepo),
		WithSheinSyncRepository(syncRepo),
		WithSheinSyncServices(&stubSheinSyncHandlerService{}, stubSheinCandidateHandlerService{}, stubSheinEnrollmentHandlerService{}),
	)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.GET("/api/v1/listing-kits/shein-sync/dashboard", h.ListSheinEnrollmentDashboard)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/shein-sync/dashboard?activity_type=PROMOTION", nil)
	req.Header.Set("X-Tenant-ID", "18")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", resp.Code, resp.Body.String())
	}

	var body struct {
		Items []listingkit.SheinEnrollmentStoreSummary `json:"items"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if len(body.Items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(body.Items))
	}
	if body.Items[0].SyncedProductCount != 2 {
		t.Fatalf("synced_product_count = %d, want 2", body.Items[0].SyncedProductCount)
	}
	if body.Items[0].MissingCostCount != 1 {
		t.Fatalf("missing_cost_count = %d, want 1", body.Items[0].MissingCostCount)
	}
	if body.Items[0].PendingReviewCount != 1 {
		t.Fatalf("pending_review_count = %d, want 1", body.Items[0].PendingReviewCount)
	}
	if body.Items[0].ReadyToEnrollCount != 1 {
		t.Fatalf("ready_to_enroll_count = %d, want 1", body.Items[0].ReadyToEnrollCount)
	}
	if body.Items[0].LastEnrollmentRun == nil || body.Items[0].LastEnrollmentRun.Status != listingkit.SheinEnrollmentRunStatusSucceeded {
		t.Fatalf("last enrollment run = %+v, want succeeded", body.Items[0].LastEnrollmentRun)
	}
}

func TestListSheinEnrollmentDashboardUsesLegacyTenantMapping(t *testing.T) {
	t.Parallel()

	restore := tenantbridge.ConfigureLegacyTenantResolver(stubLegacyTenantResolver{
		mapping: map[string]int64{"373211199677923496": 227},
	})
	t.Cleanup(restore)

	gin.SetMode(gin.TestMode)
	storeRepo := &stubSheinSummaryStoreRepository{
		stores: []listingadmin.Store{
			{
				ID:       1024,
				TenantID: 227,
				Name:     "MX Store",
				Username: "zone-mx",
				Platform: "SHEIN",
				Region:   "MX",
			},
		},
	}
	syncRepo := store.NewMemSheinSyncRepository()
	h, err := NewHandler(
		&stubHandlerCoreService{},
		WithStoreRepository(storeRepo),
		WithSheinSyncRepository(syncRepo),
		WithSheinSyncServices(&stubSheinSyncHandlerService{}, stubSheinCandidateHandlerService{}, stubSheinEnrollmentHandlerService{}),
	)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.GET("/api/v1/listing-kits/shein-sync/dashboard", h.ListSheinEnrollmentDashboard)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/shein-sync/dashboard?activity_type=PROMOTION", nil)
	req.Header.Set("X-Tenant-ID", "373211199677923496")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", resp.Code, resp.Body.String())
	}

	var body struct {
		Items []listingkit.SheinEnrollmentStoreSummary `json:"items"`
		Total int                                      `json:"total"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.Total != 1 || len(body.Items) != 1 {
		t.Fatalf("total=%d len(items)=%d, want 1", body.Total, len(body.Items))
	}
	if body.Items[0].StoreID != 1024 {
		t.Fatalf("store id = %d, want 1024", body.Items[0].StoreID)
	}
}

func TestListSheinEnrollmentDashboardPreservesStoreOrder(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	storeRepo := &stubSheinSummaryStoreRepository{
		stores: []listingadmin.Store{
			{
				ID:       2002,
				TenantID: 18,
				Name:     "Second Store",
				Username: "second",
				Platform: "SHEIN",
				Region:   "US",
			},
			{
				ID:       2001,
				TenantID: 18,
				Name:     "First Store",
				Username: "first",
				Platform: "SHEIN",
				Region:   "MX",
			},
		},
	}
	syncRepo := store.NewMemSheinSyncRepository()
	h, err := NewHandler(
		&stubHandlerCoreService{},
		WithStoreRepository(storeRepo),
		WithSheinSyncRepository(syncRepo),
		WithSheinSyncServices(&stubSheinSyncHandlerService{}, stubSheinCandidateHandlerService{}, stubSheinEnrollmentHandlerService{}),
	)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.GET("/api/v1/listing-kits/shein-sync/dashboard", h.ListSheinEnrollmentDashboard)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/shein-sync/dashboard?activity_type=PROMOTION", nil)
	req.Header.Set("X-Tenant-ID", "18")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", resp.Code, resp.Body.String())
	}

	var body struct {
		Items []listingkit.SheinEnrollmentStoreSummary `json:"items"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if len(body.Items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(body.Items))
	}
	if body.Items[0].StoreID != 2002 || body.Items[1].StoreID != 2001 {
		t.Fatalf("store order = [%d %d], want [2002 2001]", body.Items[0].StoreID, body.Items[1].StoreID)
	}
}

func TestListSheinActivityEnrollmentRunsReturnsStoreRuns(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	syncRepo := store.NewMemSheinSyncRepository()
	now := time.Date(2026, 6, 5, 11, 0, 0, 0, time.UTC)
	for _, row := range []*listingkit.SheinActivityEnrollmentRunRecord{
		{
			TenantID:     18,
			StoreID:      2001,
			ActivityType: "PROMOTION",
			ActivityKey:  "PROMOTION:18:2001",
			TriggerMode:  listingkit.SheinEnrollmentRunTriggerModeManualConfirmed,
			Status:       listingkit.SheinEnrollmentRunStatusFailed,
			StartedAt:    sheinTimePtr(now),
		},
		{
			TenantID:     18,
			StoreID:      2001,
			ActivityType: "PROMOTION",
			ActivityKey:  "PROMOTION:18:2001",
			TriggerMode:  listingkit.SheinEnrollmentRunTriggerModeManualConfirmed,
			Status:       listingkit.SheinEnrollmentRunStatusSucceeded,
			StartedAt:    sheinTimePtr(now.Add(time.Minute)),
		},
	} {
		if err := syncRepo.CreateEnrollmentRun(context.Background(), row); err != nil {
			t.Fatalf("seed run: %v", err)
		}
	}

	h, err := NewHandler(
		&stubHandlerCoreService{},
		WithSheinSyncRepository(syncRepo),
		WithSheinSyncServices(&stubSheinSyncHandlerService{}, stubSheinCandidateHandlerService{}, stubSheinEnrollmentHandlerService{}),
	)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.GET("/api/v1/listing-kits/shein-sync/stores/:store_id/enrollment-runs", h.ListSheinActivityEnrollmentRuns)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/shein-sync/stores/2001/enrollment-runs?activity_type=PROMOTION&page=1&page_size=10", nil)
	req.Header.Set("X-Tenant-ID", "18")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", resp.Code, resp.Body.String())
	}

	var body struct {
		Items []listingkit.SheinActivityEnrollmentRunRecord `json:"items"`
		Total int64                                         `json:"total"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.Total != 2 || len(body.Items) != 2 {
		t.Fatalf("total=%d len=%d, want 2", body.Total, len(body.Items))
	}
	if body.Items[0].Status != listingkit.SheinEnrollmentRunStatusSucceeded {
		t.Fatalf("first run status = %q, want succeeded", body.Items[0].Status)
	}
}

func sheinTimePtr(v time.Time) *time.Time {
	return &v
}

func float64Ptr(v float64) *float64 {
	return &v
}
