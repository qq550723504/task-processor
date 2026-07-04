package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"

	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/store"
	sheinproduct "task-processor/internal/shein/api/product"
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
	sourceCode    string
	sourceCount   int
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

func (s *stubSheinSyncHandlerService) SyncSheinSourceSDSProduct(ctx context.Context, tenantID, storeID int64, sourceCode string) (int, error) {
	s.syncCtx = ctx
	s.syncTenantID = tenantID
	s.syncStoreID = storeID
	s.sourceCode = sourceCode
	return s.sourceCount, s.syncErr
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

func (s *stubSheinSyncHandlerService) ResolveProductAPI(context.Context, int64) (sheinproduct.ProductAPI, error) {
	return nil, nil
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

func (stubSheinEnrollmentHandlerService) StartSheinActivityEnrollment(context.Context, int64, int64, string, string, listingkit.SheinEnrollmentRunTriggerMode, ...int64) (*listingkit.SheinActivityEnrollmentRunRecord, error) {
	return nil, nil
}

func (stubSheinEnrollmentHandlerService) ExecuteSheinActivityEnrollment(context.Context, int64, int64, string, string, listingkit.SheinEnrollmentRunTriggerMode, ...int64) (*listingkit.SheinActivityEnrollmentRunRecord, error) {
	return nil, nil
}

func (stubSheinEnrollmentHandlerService) ExecuteAutoSheinActivityEnrollment(context.Context, int64, int64, string, string) (*listingkit.SheinActivityEnrollmentRunRecord, error) {
	return nil, nil
}

type stubSourceMetadataHandlerCoreService struct {
	stubHandlerCoreService
	ctx   context.Context
	query *listingkit.SheinSourceSDSMetadataQuery
	items []listingkit.SheinSourceSDSMetadataRecord
	err   error
}

func (s *stubSourceMetadataHandlerCoreService) ListSheinSourceSDSMetadata(ctx context.Context, query *listingkit.SheinSourceSDSMetadataQuery) ([]listingkit.SheinSourceSDSMetadataRecord, error) {
	s.ctx = ctx
	s.query = query
	return s.items, s.err
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

type summaryStatsOnlySheinSyncRepository struct {
	listingkit.SheinSyncRepository

	syncedProductCount int
	missingCostCount   int
	pendingReviewCount int
	readyToEnrollCount int

	summaryTenantID     int64
	summaryStoreID      int64
	summaryActivityType string
}

func (r *summaryStatsOnlySheinSyncRepository) CountSheinEnrollmentSummary(ctx context.Context, tenantID, storeID int64, activityType string) (int, int, int, int, error) {
	r.summaryTenantID = tenantID
	r.summaryStoreID = storeID
	r.summaryActivityType = activityType
	return r.syncedProductCount, r.missingCostCount, r.pendingReviewCount, r.readyToEnrollCount, nil
}

func (r *summaryStatsOnlySheinSyncRepository) ListSyncedProducts(context.Context, *listingkit.SheinSyncedProductQuery) ([]listingkit.SheinSyncedProductRecord, int64, error) {
	return nil, 0, errors.New("summary should not list synced products")
}

func (r *summaryStatsOnlySheinSyncRepository) ListCandidates(context.Context, *listingkit.SheinActivityCandidateQuery) ([]listingkit.SheinActivityCandidateRecord, int64, error) {
	return nil, 0, errors.New("summary should not list candidates")
}

func (r *summaryStatsOnlySheinSyncRepository) ListSyncJobs(context.Context, *listingkit.SheinSyncJobQuery) ([]listingkit.SheinSyncJobRecord, int64, error) {
	return nil, 0, nil
}

func (r *summaryStatsOnlySheinSyncRepository) ListEnrollmentRuns(context.Context, *listingkit.SheinEnrollmentRunQuery) ([]listingkit.SheinActivityEnrollmentRunRecord, int64, error) {
	return nil, 0, nil
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

func TestSyncSheinSourceSDSProductReturnsSyncedCount(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	syncSvc := &stubSheinSyncHandlerService{sourceCount: 18}
	h, err := NewHandler(
		&stubHandlerCoreService{},
		WithSheinSyncServices(syncSvc, stubSheinCandidateHandlerService{}, stubSheinEnrollmentHandlerService{}),
	)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/shein-sync/stores/:store_id/source-sds-products/:source_code/sync", h.SyncSheinSourceSDSProduct)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/shein-sync/stores/870/source-sds-products/XB0603003001/sync", nil)
	req.Header.Set("X-Tenant-ID", "227")
	req.Header.Set("X-User-ID", "373211204509761704")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", resp.Code, resp.Body.String())
	}
	if syncSvc.syncTenantID != 227 || syncSvc.syncStoreID != 870 || syncSvc.sourceCode != "XB0603003001" {
		t.Fatalf("sync target = tenant:%d store:%d source:%q, want tenant:227 store:870 source:XB0603003001", syncSvc.syncTenantID, syncSvc.syncStoreID, syncSvc.sourceCode)
	}
	if got := listingkit.TenantIDFromContext(syncSvc.syncCtx); got != "227" {
		t.Fatalf("tenant in ctx = %q, want 227", got)
	}

	var body struct {
		SourceCode  string `json:"source_code"`
		SyncedCount int    `json:"synced_count"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.SourceCode != "XB0603003001" || body.SyncedCount != 18 {
		t.Fatalf("body = %+v, want source code and synced count", body)
	}
}

func TestGetSheinActivityStrategyReturnsConfiguredPromotionStrategy(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	repo := newSheinActivityStrategyTestRepository(t)
	discountRate := 0.22
	stockRatio := 0.45
	_, err := repo.CreateOperationStrategy(context.Background(), &listingadmin.OperationStrategy{
		TenantID:             18,
		StoreID:              2001,
		Name:                 "SHEIN 活动报名",
		Platform:             "SHEIN",
		Status:               0,
		ActivityEnabled:      true,
		ActivityType:         "PROMOTION",
		ActivityPriceMode:    "DISCOUNT",
		ActivityDiscountRate: &discountRate,
		ActivityStockRatio:   &stockRatio,
	})
	if err != nil {
		t.Fatalf("seed strategy: %v", err)
	}
	h, err := NewHandler(&stubHandlerCoreService{}, WithOperationStrategyRepository(repo))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.GET("/api/v1/listing-kits/shein-sync/stores/:store_id/activity-strategy", h.GetSheinActivityStrategy)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/shein-sync/stores/2001/activity-strategy", nil)
	req.Header.Set("X-Tenant-ID", "18")
	req.Header.Set("X-User-ID", "shein-ops")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", resp.Code, resp.Body.String())
	}
	var body struct {
		Configured bool `json:"configured"`
		Strategy   struct {
			StoreID              int64   `json:"store_id"`
			ActivityType         string  `json:"activity_type"`
			ActivityPriceMode    string  `json:"activity_price_mode"`
			ActivityDiscountRate float64 `json:"activity_discount_rate"`
			ActivityStockRatio   float64 `json:"activity_stock_ratio"`
		} `json:"strategy"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if !body.Configured || body.Strategy.StoreID != 2001 || body.Strategy.ActivityType != "PROMOTION" {
		t.Fatalf("body = %+v, want configured promotion strategy", body)
	}
	if body.Strategy.ActivityDiscountRate != 0.22 || body.Strategy.ActivityStockRatio != 0.45 {
		t.Fatalf("strategy rates = discount:%v stock:%v, want 0.22/0.45", body.Strategy.ActivityDiscountRate, body.Strategy.ActivityStockRatio)
	}
}

func TestUpdateSheinActivityStrategyCreatesStorePromotionStrategy(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	repo := newSheinActivityStrategyTestRepository(t)
	h, err := NewHandler(&stubHandlerCoreService{}, WithOperationStrategyRepository(repo))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.PATCH("/api/v1/listing-kits/shein-sync/stores/:store_id/activity-strategy", h.UpdateSheinActivityStrategy)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/listing-kits/shein-sync/stores/2001/activity-strategy", strings.NewReader(`{
		"activity_price_mode":"DISCOUNT",
		"activity_partake_type":"LIMITED",
		"activity_discount_rate":0.18,
		"activity_stock_ratio":0.4,
		"activity_min_profit_rate":0.15,
		"fixed_price_adjustment":1.2
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "18")
	req.Header.Set("X-User-ID", "shein-ops")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", resp.Code, resp.Body.String())
	}
	strategy, err := repo.GetActiveActivityStrategy(context.Background(), 18, 2001, "SHEIN", "PROMOTION")
	if err != nil {
		t.Fatalf("load strategy: %v", err)
	}
	if strategy == nil || strategy.Name != "SHEIN 活动报名" || !strategy.ActivityEnabled || strategy.ActivityType != "PROMOTION" {
		t.Fatalf("strategy = %+v, want active SHEIN promotion strategy", strategy)
	}
	if strategy.ActivityDiscountRate == nil || *strategy.ActivityDiscountRate != 0.18 {
		t.Fatalf("discount rate = %v, want 0.18", strategy.ActivityDiscountRate)
	}
	if strategy.ActivityPartakeType != "LIMITED" {
		t.Fatalf("activity partake type = %q, want LIMITED", strategy.ActivityPartakeType)
	}
	var body struct {
		Strategy struct {
			ActivityPartakeType string `json:"activity_partake_type"`
		} `json:"strategy"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.Strategy.ActivityPartakeType != "LIMITED" {
		t.Fatalf("response activity_partake_type = %q, want LIMITED", body.Strategy.ActivityPartakeType)
	}
}

func TestUpdateSheinActivityStrategyAllowsRegularPartakeWithoutStockRatio(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	repo := newSheinActivityStrategyTestRepository(t)
	h, err := NewHandler(&stubHandlerCoreService{}, WithOperationStrategyRepository(repo))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.PATCH("/api/v1/listing-kits/shein-sync/stores/:store_id/activity-strategy", h.UpdateSheinActivityStrategy)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/listing-kits/shein-sync/stores/2001/activity-strategy", strings.NewReader(`{
		"activity_price_mode":"DISCOUNT",
		"activity_partake_type":"REGULAR",
		"activity_discount_rate":0.18
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "18")
	req.Header.Set("X-User-ID", "shein-ops")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", resp.Code, resp.Body.String())
	}
	strategy, err := repo.GetActiveActivityStrategy(context.Background(), 18, 2001, "SHEIN", "PROMOTION")
	if err != nil {
		t.Fatalf("load strategy: %v", err)
	}
	if strategy == nil || strategy.ActivityStockRatio != nil {
		t.Fatalf("regular strategy stock ratio = %+v, want nil", strategy)
	}
}

func TestSheinActivityStrategyCanBeScopedToTimeLimitedActivity(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	repo := newSheinActivityStrategyTestRepository(t)
	h, err := NewHandler(&stubHandlerCoreService{}, WithOperationStrategyRepository(repo))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.GET("/api/v1/listing-kits/shein-sync/stores/:store_id/activity-strategy", h.GetSheinActivityStrategy)
	router.PATCH("/api/v1/listing-kits/shein-sync/stores/:store_id/activity-strategy", h.UpdateSheinActivityStrategy)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/listing-kits/shein-sync/stores/2001/activity-strategy", strings.NewReader(`{
		"activity_type":"TIME_LIMITED",
		"activity_price_mode":"PROFIT",
		"activity_stock_ratio":0.3,
		"activity_min_profit_rate":0,
		"fixed_price_adjustment":1.1
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "18")
	req.Header.Set("X-User-ID", "shein-ops")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("patch status = %d, want 200 body=%s", resp.Code, resp.Body.String())
	}
	timeLimited, err := repo.GetActiveActivityStrategy(context.Background(), 18, 2001, "SHEIN", "TIME_LIMITED")
	if err != nil {
		t.Fatalf("load time limited strategy: %v", err)
	}
	if timeLimited == nil || timeLimited.ActivityType != "TIME_LIMITED" {
		t.Fatalf("time limited strategy = %+v, want active TIME_LIMITED strategy", timeLimited)
	}
	if timeLimited.ActivityMinProfitRate == nil || *timeLimited.ActivityMinProfitRate != 0 {
		t.Fatalf("time limited min profit = %v, want 0", timeLimited.ActivityMinProfitRate)
	}
	if promotion, err := repo.GetActiveActivityStrategy(context.Background(), 18, 2001, "SHEIN", "PROMOTION"); err != nil || promotion != nil {
		t.Fatalf("promotion strategy = %+v err=%v, want untouched nil", promotion, err)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/shein-sync/stores/2001/activity-strategy?activity_type=TIME_LIMITED", nil)
	getReq.Header.Set("X-Tenant-ID", "18")
	getReq.Header.Set("X-User-ID", "shein-ops")
	getResp := httptest.NewRecorder()
	router.ServeHTTP(getResp, getReq)

	if getResp.Code != http.StatusOK {
		t.Fatalf("get status = %d, want 200 body=%s", getResp.Code, getResp.Body.String())
	}
	var body struct {
		Configured bool `json:"configured"`
		Strategy   struct {
			ActivityType          string  `json:"activity_type"`
			ActivityPriceMode     string  `json:"activity_price_mode"`
			ActivityStockRatio    float64 `json:"activity_stock_ratio"`
			ActivityMinProfitRate float64 `json:"activity_min_profit_rate"`
		} `json:"strategy"`
	}
	if err := json.Unmarshal(getResp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if !body.Configured || body.Strategy.ActivityType != "TIME_LIMITED" || body.Strategy.ActivityPriceMode != "PROFIT" {
		t.Fatalf("body = %+v, want configured TIME_LIMITED PROFIT strategy", body)
	}
	if body.Strategy.ActivityStockRatio != 0.3 || body.Strategy.ActivityMinProfitRate != 0 {
		t.Fatalf("body strategy = %+v, want stock 0.3 profit 0", body.Strategy)
	}
}

func TestUpdateSheinActivityStrategyAllowsZeroProfitFloor(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	repo := newSheinActivityStrategyTestRepository(t)
	h, err := NewHandler(&stubHandlerCoreService{}, WithOperationStrategyRepository(repo))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.PATCH("/api/v1/listing-kits/shein-sync/stores/:store_id/activity-strategy", h.UpdateSheinActivityStrategy)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/listing-kits/shein-sync/stores/2001/activity-strategy", strings.NewReader(`{
		"activity_price_mode":"PROFIT",
		"activity_stock_ratio":0.4,
		"activity_min_profit_rate":0,
		"fixed_price_adjustment":0
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "18")
	req.Header.Set("X-User-ID", "shein-ops")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", resp.Code, resp.Body.String())
	}
	strategy, err := repo.GetActiveActivityStrategy(context.Background(), 18, 2001, "SHEIN", "PROMOTION")
	if err != nil {
		t.Fatalf("load strategy: %v", err)
	}
	if strategy == nil || strategy.ActivityMinProfitRate == nil || *strategy.ActivityMinProfitRate != 0 {
		t.Fatalf("min profit rate = %v, want 0", strategy)
	}
}

func TestUpdateSheinActivityStrategyUpdatesLegacySchemaWithoutUnrelatedColumns(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`
CREATE TABLE listing_operation_strategy (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	tenant_id INTEGER NOT NULL,
	owner_user_id TEXT,
	store_id INTEGER NOT NULL,
	name TEXT NOT NULL,
	platform TEXT NOT NULL,
	status INTEGER NOT NULL DEFAULT 0,
	activity_enabled INTEGER NOT NULL DEFAULT 0,
	activity_type TEXT,
	activity_discount_rate REAL,
	activity_stock_ratio REAL,
	activity_min_profit_rate REAL,
	activity_price_mode TEXT,
	fixed_price_adjustment REAL,
	deleted INTEGER NOT NULL DEFAULT 0
)`).Error; err != nil {
		t.Fatalf("create legacy strategy table: %v", err)
	}
	if err := db.Exec(`
INSERT INTO listing_operation_strategy
	(tenant_id, owner_user_id, store_id, name, platform, status, activity_enabled, activity_type, activity_discount_rate, activity_stock_ratio, activity_price_mode, deleted)
VALUES
	(18, 'shein-ops', 2001, 'SHEIN 活动报名', 'SHEIN', 0, 1, 'PROMOTION', 0.2, 0.5, 'DISCOUNT', 0)
`).Error; err != nil {
		t.Fatalf("seed legacy strategy: %v", err)
	}
	repo := listingadmin.NewGormOperationStrategyRepository(db)
	h, err := NewHandler(&stubHandlerCoreService{}, WithOperationStrategyRepository(repo))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.PATCH("/api/v1/listing-kits/shein-sync/stores/:store_id/activity-strategy", h.UpdateSheinActivityStrategy)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/listing-kits/shein-sync/stores/2001/activity-strategy", strings.NewReader(`{
		"activity_price_mode":"DISCOUNT",
		"activity_discount_rate":0.18,
		"activity_stock_ratio":0.4,
		"fixed_price_adjustment":1.2
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "18")
	req.Header.Set("X-User-ID", "shein-ops")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", resp.Code, resp.Body.String())
	}
	var row struct {
		ActivityDiscountRate float64 `gorm:"column:activity_discount_rate"`
		ActivityStockRatio   float64 `gorm:"column:activity_stock_ratio"`
		FixedPriceAdjustment float64 `gorm:"column:fixed_price_adjustment"`
	}
	if err := db.Table("listing_operation_strategy").Where("tenant_id = ? AND store_id = ?", 18, 2001).Take(&row).Error; err != nil {
		t.Fatalf("load updated strategy: %v", err)
	}
	if row.ActivityDiscountRate != 0.18 || row.ActivityStockRatio != 0.4 || row.FixedPriceAdjustment != 1.2 {
		t.Fatalf("row = %+v, want updated activity fields", row)
	}
}

func TestUpdateSheinActivityStrategyRejectsBothWhenLimitedDiscountIsNotGreater(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	repo := newSheinActivityStrategyTestRepository(t)
	h, err := NewHandler(&stubHandlerCoreService{}, WithOperationStrategyRepository(repo))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.PATCH("/api/v1/listing-kits/shein-sync/stores/:store_id/activity-strategy", h.UpdateSheinActivityStrategy)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/listing-kits/shein-sync/stores/2001/activity-strategy", strings.NewReader(`{
		"activity_price_mode":"DISCOUNT",
		"activity_partake_type":"BOTH",
		"activity_discount_rate":0.2,
		"activity_limited_discount_rate":0.2,
		"activity_stock_ratio":0.4
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "18")
	req.Header.Set("X-User-ID", "shein-ops")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 body=%s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "activity_limited_discount_rate must be greater than activity_discount_rate") {
		t.Fatalf("body = %s, want limited discount validation message", resp.Body.String())
	}
}

func TestUpdateSheinActivityStrategyRejectsBothProfitWhenLimitedMinProfitIsNotLower(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	repo := newSheinActivityStrategyTestRepository(t)
	h, err := NewHandler(&stubHandlerCoreService{}, WithOperationStrategyRepository(repo))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.PATCH("/api/v1/listing-kits/shein-sync/stores/:store_id/activity-strategy", h.UpdateSheinActivityStrategy)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/listing-kits/shein-sync/stores/2001/activity-strategy", strings.NewReader(`{
		"activity_price_mode":"PROFIT",
		"activity_partake_type":"BOTH",
		"activity_min_profit_rate":0.2,
		"activity_limited_min_profit_rate":0.2,
		"activity_stock_ratio":0.4
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "18")
	req.Header.Set("X-User-ID", "shein-ops")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 body=%s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "activity_limited_min_profit_rate must be less than activity_min_profit_rate") {
		t.Fatalf("body = %s, want limited min profit validation message", resp.Body.String())
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

func TestGetSheinEnrollmentStoreSummaryUsesLegacyTenantMapping(t *testing.T) {
	restore := tenantbridge.ConfigureLegacyTenantResolver(stubLegacyTenantResolver{
		mapping: map[string]int64{"373211199677923496": 227},
	})
	t.Cleanup(restore)

	gin.SetMode(gin.TestMode)
	storeRepo := &stubSheinSummaryStoreRepository{
		stores: []listingadmin.Store{
			{
				ID:       870,
				TenantID: 227,
				Name:     "US Store",
				Username: "zone-us",
				Platform: "SHEIN",
				Region:   "US",
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
	router.GET("/api/v1/listing-kits/shein-sync/stores/:store_id/summary", h.GetSheinEnrollmentStoreSummary)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/shein-sync/stores/870/summary?activity_type=PROMOTION", nil)
	req.Header.Set("X-Tenant-ID", "373211199677923496")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", resp.Code, resp.Body.String())
	}

	var body struct {
		Summary listingkit.SheinEnrollmentStoreSummary `json:"summary"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.Summary.StoreID != 870 || body.Summary.ActivityType != "PROMOTION" {
		t.Fatalf("summary = %+v, want mapped store 870 PROMOTION", body.Summary)
	}
}

func TestGetSheinEnrollmentStoreSummaryUsesAggregatedCounts(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	storeRepo := &stubSheinSummaryStoreRepository{
		stores: []listingadmin.Store{
			{
				ID:       870,
				TenantID: 227,
				Name:     "US Store",
				Username: "zone-us",
				Platform: "SHEIN",
				Region:   "US",
			},
		},
	}
	syncRepo := &summaryStatsOnlySheinSyncRepository{
		syncedProductCount: 1000,
		missingCostCount:   123,
		pendingReviewCount: 45,
		readyToEnrollCount: 6,
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
	router.GET("/api/v1/listing-kits/shein-sync/stores/:store_id/summary", h.GetSheinEnrollmentStoreSummary)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/shein-sync/stores/870/summary?activity_type=TIME_LIMITED", nil)
	req.Header.Set("X-Tenant-ID", "227")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", resp.Code, resp.Body.String())
	}

	var body struct {
		Summary listingkit.SheinEnrollmentStoreSummary `json:"summary"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.Summary.SyncedProductCount != 1000 ||
		body.Summary.MissingCostCount != 123 ||
		body.Summary.PendingReviewCount != 45 ||
		body.Summary.ReadyToEnrollCount != 6 {
		t.Fatalf("summary counts = %+v, want aggregated counts", body.Summary)
	}
	if syncRepo.summaryTenantID != 227 || syncRepo.summaryStoreID != 870 || syncRepo.summaryActivityType != "TIME_LIMITED" {
		t.Fatalf("summary query = tenant %d store %d activity %q, want 227/870/TIME_LIMITED", syncRepo.summaryTenantID, syncRepo.summaryStoreID, syncRepo.summaryActivityType)
	}
}

func TestGetSheinEnrollmentStoreSummaryReturnsNotFoundWhenStoreOutsideTenant(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	storeRepo := &stubSheinSummaryStoreRepository{
		stores: []listingadmin.Store{
			{ID: 870, TenantID: 227, Name: "US Store", Platform: "SHEIN"},
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
	router.GET("/api/v1/listing-kits/shein-sync/stores/:store_id/summary", h.GetSheinEnrollmentStoreSummary)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/shein-sync/stores/870/summary?activity_type=PROMOTION", nil)
	req.Header.Set("X-Tenant-ID", "999")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 body=%s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "store_not_found") {
		t.Fatalf("body = %s, want store_not_found", resp.Body.String())
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

func TestListSheinActivityEnrollmentRunItemsReturnsScopedItemDetails(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	syncRepo := store.NewMemSheinSyncRepository()
	targetRun := &listingkit.SheinActivityEnrollmentRunRecord{
		TenantID:     18,
		StoreID:      2001,
		ActivityType: "PROMOTION",
		ActivityKey:  "PROMOTION:18:2001",
		TriggerMode:  listingkit.SheinEnrollmentRunTriggerModeManualConfirmed,
		Status:       listingkit.SheinEnrollmentRunStatusFailed,
	}
	otherStoreRun := &listingkit.SheinActivityEnrollmentRunRecord{
		TenantID:     18,
		StoreID:      2002,
		ActivityType: "PROMOTION",
		ActivityKey:  "PROMOTION:18:2002",
		TriggerMode:  listingkit.SheinEnrollmentRunTriggerModeManualConfirmed,
		Status:       listingkit.SheinEnrollmentRunStatusFailed,
	}
	if err := syncRepo.CreateEnrollmentRun(context.Background(), targetRun); err != nil {
		t.Fatalf("seed target run: %v", err)
	}
	if err := syncRepo.CreateEnrollmentRun(context.Background(), otherStoreRun); err != nil {
		t.Fatalf("seed other store run: %v", err)
	}
	if err := syncRepo.SaveEnrollmentItems(context.Background(), []*listingkit.SheinActivityEnrollmentItemRecord{
		{
			RunID:           targetRun.ID,
			CandidateID:     10,
			StoreID:         2001,
			ActivityKey:     "PROMOTION:18:2001",
			SyncedProductID: 100,
			SKCName:         "sg260618174087119533319",
			Status:          listingkit.SheinEnrollmentItemStatusFailed,
			ErrorMessage:    "current status can not enroll",
			RequestPayload:  `{"raw":true}`,
			ResponsePayload: `{"message":"failed"}`,
		},
		{
			RunID:           otherStoreRun.ID,
			CandidateID:     20,
			StoreID:         2002,
			ActivityKey:     "PROMOTION:18:2002",
			SyncedProductID: 200,
			SKCName:         "sg-other-store",
			Status:          listingkit.SheinEnrollmentItemStatusFailed,
			ErrorMessage:    "must not leak",
		},
	}); err != nil {
		t.Fatalf("seed items: %v", err)
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
	router.GET("/api/v1/listing-kits/shein-sync/stores/:store_id/enrollment-runs/:run_id/items", h.ListSheinActivityEnrollmentRunItems)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/shein-sync/stores/2001/enrollment-runs/1/items?page=1&page_size=10", nil)
	req.Header.Set("X-Tenant-ID", "18")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", resp.Code, resp.Body.String())
	}

	var body struct {
		Items []listingkit.SheinActivityEnrollmentItemRecord `json:"items"`
		Total int64                                          `json:"total"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.Total != 1 || len(body.Items) != 1 {
		t.Fatalf("total=%d len=%d, want 1", body.Total, len(body.Items))
	}
	if body.Items[0].SKCName != "sg260618174087119533319" || body.Items[0].ErrorMessage != "current status can not enroll" {
		t.Fatalf("item = %+v, want scoped failed item", body.Items[0])
	}
	if body.Items[0].RequestPayload != "" || body.Items[0].ResponsePayload != "" {
		t.Fatalf("payloads should be hidden by default: %+v", body.Items[0])
	}
}

func TestListSheinSourceSDSMetadataReturnsHistoricalTaskMetadata(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	core := &stubSourceMetadataHandlerCoreService{
		items: []listingkit.SheinSourceSDSMetadataRecord{{
			SourceCode:   "XB0610007001",
			Title:        "方形双层腰包 -（单图多拼可选）",
			VariantSKU:   "XB0610007001",
			Price:        34.5,
			VariantLabel: "white / 16x23cm",
		}},
	}
	h, err := NewHandler(
		core,
		WithSheinSyncServices(&stubSheinSyncHandlerService{}, stubSheinCandidateHandlerService{}, stubSheinEnrollmentHandlerService{}),
	)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.GET("/api/v1/listing-kits/shein-sync/stores/:store_id/source-sds-metadata", h.ListSheinSourceSDSMetadata)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/shein-sync/stores/870/source-sds-metadata?source_codes=XB0610007001", nil)
	req.Header.Set("X-Tenant-ID", "227")
	req.Header.Set("X-User-ID", "373211204509761704")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", resp.Code, resp.Body.String())
	}
	if core.query == nil || core.query.StoreID != 870 || len(core.query.SourceCodes) != 1 || core.query.SourceCodes[0] != "XB0610007001" {
		t.Fatalf("query = %+v, want store and source code", core.query)
	}
	if got := listingkit.TenantIDFromContext(core.ctx); got != "227" {
		t.Fatalf("tenant in ctx = %q, want 227", got)
	}
	if got := listingkit.RequestUserIDFromContext(core.ctx); got != "373211204509761704" {
		t.Fatalf("user in ctx = %q, want request user", got)
	}

	var body struct {
		Items []listingkit.SheinSourceSDSMetadataRecord `json:"items"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if len(body.Items) != 1 || body.Items[0].Title != "方形双层腰包 -（单图多拼可选）" {
		t.Fatalf("items = %+v, want source title", body.Items)
	}
}

func sheinTimePtr(v time.Time) *time.Time {
	return &v
}

func float64Ptr(v float64) *float64 {
	return &v
}

func newSheinActivityStrategyTestRepository(t *testing.T) *listingadmin.GormOperationStrategyRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&sheinActivityStrategyTestRow{}); err != nil {
		t.Fatalf("migrate listing_operation_strategy: %v", err)
	}
	return listingadmin.NewGormOperationStrategyRepository(db)
}

type sheinActivityStrategyTestRow struct {
	ID                           int64      `gorm:"column:id;primaryKey;autoIncrement"`
	TenantID                     int64      `gorm:"column:tenant_id;not null;index"`
	OwnerUserID                  string     `gorm:"column:owner_user_id;type:varchar(128);index"`
	StoreID                      int64      `gorm:"column:store_id;not null;index"`
	Name                         string     `gorm:"column:name;not null"`
	Platform                     string     `gorm:"column:platform;not null;index"`
	Status                       int16      `gorm:"column:status;not null;default:0;index"`
	StockChangeThreshold         int        `gorm:"column:stock_change_threshold"`
	StockChangeAction            string     `gorm:"column:stock_change_action"`
	OutOfStockAction             string     `gorm:"column:out_of_stock_action"`
	MinProfitRate                float64    `gorm:"column:min_profit_rate"`
	LowProfitAction              string     `gorm:"column:low_profit_action"`
	PriceUpdateMultiplier        float64    `gorm:"column:price_update_multiplier"`
	FixedPriceAdjustment         float64    `gorm:"column:fixed_price_adjustment"`
	StockUpdateRatio             float64    `gorm:"column:stock_update_ratio"`
	Remark                       string     `gorm:"column:remark"`
	ActivityEnabled              int16      `gorm:"column:activity_enabled;not null;default:0"`
	ActivityType                 string     `gorm:"column:activity_type"`
	ActivityDiscountRate         float64    `gorm:"column:activity_discount_rate"`
	ActivityStockRatio           float64    `gorm:"column:activity_stock_ratio"`
	PromotionRatio               float64    `gorm:"column:promotion_ratio"`
	ActivityMinProfitRate        float64    `gorm:"column:activity_min_profit_rate"`
	ActivityLimitedMinProfitRate float64    `gorm:"column:activity_limited_min_profit_rate"`
	ActivityPriceMode            string     `gorm:"column:activity_price_mode"`
	ActivityPartakeType          string     `gorm:"column:activity_partake_type"`
	TimeLimitedDiscountRate      float64    `gorm:"column:time_limited_discount_rate"`
	TimeLimitedMinProfitRate     float64    `gorm:"column:time_limited_min_profit_rate"`
	TimeLimitedPriceMode         string     `gorm:"column:time_limited_price_mode"`
	TimeLimitedUserLimit         bool       `gorm:"column:time_limited_user_limit"`
	TimeLimitedUserLimitNum      int        `gorm:"column:time_limited_user_limit_num"`
	TimeLimitedStockLimit        bool       `gorm:"column:time_limited_stock_limit"`
	TimeLimitedStockLimitPercent int        `gorm:"column:time_limited_stock_limit_percent"`
	PriceIncreaseThreshold       float64    `gorm:"column:price_increase_threshold"`
	PriceDecreaseThreshold       float64    `gorm:"column:price_decrease_threshold"`
	PriceIncreaseAction          string     `gorm:"column:price_increase_action"`
	PriceDecreaseAction          string     `gorm:"column:price_decrease_action"`
	RestoreStockAmount           int        `gorm:"column:restore_stock_amount"`
	Creator                      string     `gorm:"column:creator"`
	CreatedBy                    string     `gorm:"column:created_by;type:varchar(128)"`
	CreateTime                   *time.Time `gorm:"column:create_time;autoCreateTime"`
	Updater                      string     `gorm:"column:updater"`
	UpdatedBy                    string     `gorm:"column:updated_by;type:varchar(128)"`
	UpdateTime                   *time.Time `gorm:"column:update_time;autoUpdateTime"`
	Deleted                      int16      `gorm:"column:deleted;not null;default:0;index"`
}

func (sheinActivityStrategyTestRow) TableName() string { return "listing_operation_strategy" }
