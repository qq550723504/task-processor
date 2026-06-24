package listingkit

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	sheinproduct "task-processor/internal/shein/api/product"
)

func TestSDSRetirementConfirmRunExecutesSelectedItemsAfterRefresh(t *testing.T) {
	repo := &sdsRetirementExecutionRepoStub{
		storedRun: &SDSRetirementRunRecord{
			ID:       "run-1",
			TenantID: "18",
			Platform: "shein",
			StoreID:  177,
			Status:   SDSRetirementRunStatusReady,
		},
		storedItems: []SDSRetirementItemRecord{
			{
				ID:              "item-1",
				RunID:           "run-1",
				TenantID:        "18",
				Platform:        "shein",
				StoreID:         177,
				SyncedProductID: 11,
				SPUName:         "SPU-1",
				SKCName:         "SKC-1",
				BusinessModel:   2,
				Selected:        true,
				SiteSelection:   `[{"site_abbr":"US","store_type":1}]`,
				Status:          SDSRetirementItemStatusSelected,
			},
			{
				ID:       "item-2",
				RunID:    "run-1",
				TenantID: "18",
				Platform: "shein",
				StoreID:  177,
				SPUName:  "SPU-2",
				SKCName:  "SKC-2",
				Selected: false,
				Status:   SDSRetirementItemStatusPending,
			},
		},
	}
	productAPI := &sdsRetirementProductAPIStub{}
	shein := &sdsRetirementExecutionSheinSyncStub{
		supportsImmediateRefresh: true,
		productAPI:               productAPI,
	}
	productAPI.events = &shein.events
	service := NewSDSRetirementService(repo, nil, shein)

	detail, err := service.ConfirmSDSRetirementRun(WithTenantID(context.Background(), "18"), "run-1", &ConfirmSDSRetirementRunRequest{
		ConfirmedBy: "operator",
	})
	if err != nil {
		t.Fatalf("ConfirmSDSRetirementRun() error = %v", err)
	}
	if shein.syncTenantID != 18 || shein.syncStoreID != 177 || shein.syncTrigger != SheinSyncTriggerModeManual {
		t.Fatalf("sync call = tenant %d store %d trigger %q", shein.syncTenantID, shein.syncStoreID, shein.syncTrigger)
	}
	if got := strings.Join(append([]string(nil), shein.events...), ","); got != "sync,resolve,offshelf" {
		t.Fatalf("events = %q, want sync,resolve,offshelf", got)
	}
	if shein.resolvedStoreID != 177 {
		t.Fatalf("resolved store id = %d, want 177", shein.resolvedStoreID)
	}
	if len(productAPI.offShelfRequests) != 1 {
		t.Fatalf("offshelf calls = %d, want 1", len(productAPI.offShelfRequests))
	}
	req := productAPI.offShelfRequests[0]
	if req.SpuName != "SPU-1" || len(req.SkcSiteInfos) != 1 {
		t.Fatalf("request = %+v", req)
	}
	info := req.SkcSiteInfos[0]
	if info.BusinessModel != 2 || info.SkcName != "SKC-1" || len(info.OffSubSites) != 1 || info.OffSubSites[0].SiteAbbr != "US" {
		t.Fatalf("skc site info = %+v", info)
	}
	if detail.Run.Status != SDSRetirementRunStatusSucceeded || detail.Run.ConfirmedBy != "operator" || detail.Run.ConfirmedAt == nil {
		t.Fatalf("run = %#v", detail.Run)
	}
	if detail.Items[0].Status != SDSRetirementItemStatusSucceeded || detail.Items[0].FinishedAt == nil {
		t.Fatalf("item[0] = %#v", detail.Items[0])
	}
	if detail.Items[1].Status != SDSRetirementItemStatusSkipped {
		t.Fatalf("item[1] = %#v", detail.Items[1])
	}
	if repo.markedOffShelfIDs[0] != 11 {
		t.Fatalf("marked offshelf ids = %#v", repo.markedOffShelfIDs)
	}
	if repo.savedRun == nil || repo.savedRun.Status != SDSRetirementRunStatusSucceeded {
		t.Fatalf("saved run = %#v", repo.savedRun)
	}
	if repo.savedItems[0].RequestSnapshot == "" {
		t.Fatalf("saved items = %#v", repo.savedItems)
	}
	var snapshot sheinproduct.ShelfOperateRequest
	if err := json.Unmarshal([]byte(repo.savedItems[0].RequestSnapshot), &snapshot); err != nil {
		t.Fatalf("unmarshal request snapshot: %v", err)
	}
	if len(snapshot.SkcSiteInfos) != 1 || snapshot.SkcSiteInfos[0].BusinessModel != 2 {
		t.Fatalf("snapshot = %+v", snapshot)
	}
}

func TestSDSRetirementRetryRunExecutesOnlyFailedItems(t *testing.T) {
	repo := &sdsRetirementExecutionRepoStub{
		storedRun: &SDSRetirementRunRecord{
			ID:       "run-2",
			TenantID: "18",
			Platform: "shein",
			StoreID:  177,
			Status:   SDSRetirementRunStatusFailed,
		},
		storedItems: []SDSRetirementItemRecord{
			{
				ID:              "item-failed",
				RunID:           "run-2",
				TenantID:        "18",
				Platform:        "shein",
				StoreID:         177,
				SyncedProductID: 21,
				SPUName:         "SPU-1",
				SKCName:         "SKC-1",
				BusinessModel:   4,
				Selected:        true,
				SiteSelection:   `[{"site_abbr":"US","store_type":1}]`,
				Status:          SDSRetirementItemStatusFailed,
				Error:           "previous failure",
			},
			{
				ID:            "item-succeeded",
				RunID:         "run-2",
				TenantID:      "18",
				Platform:      "shein",
				StoreID:       177,
				SPUName:       "SPU-2",
				SKCName:       "SKC-2",
				BusinessModel: 4,
				Selected:      true,
				SiteSelection: `[{"site_abbr":"US","store_type":1}]`,
				Status:        SDSRetirementItemStatusSucceeded,
			},
		},
	}
	productAPI := &sdsRetirementProductAPIStub{}
	shein := &sdsRetirementExecutionSheinSyncStub{
		supportsImmediateRefresh: true,
		productAPI:               productAPI,
	}
	productAPI.events = &shein.events
	service := NewSDSRetirementService(repo, nil, shein)

	detail, err := service.RetrySDSRetirementRun(WithTenantID(context.Background(), "18"), "run-2")
	if err != nil {
		t.Fatalf("RetrySDSRetirementRun() error = %v", err)
	}
	if len(productAPI.offShelfRequests) != 1 || productAPI.offShelfRequests[0].SpuName != "SPU-1" {
		t.Fatalf("offshelf requests = %#v", productAPI.offShelfRequests)
	}
	if detail.Run.Status != SDSRetirementRunStatusSucceeded {
		t.Fatalf("run = %#v", detail.Run)
	}
	if detail.Items[0].Status != SDSRetirementItemStatusSucceeded || detail.Items[1].Status != SDSRetirementItemStatusSucceeded {
		t.Fatalf("items = %#v", detail.Items)
	}
}

func TestSDSRetirementConfirmRunRequiresTenantScope(t *testing.T) {
	service := NewSDSRetirementService(&sdsRetirementExecutionRepoStub{}, nil, &sdsRetirementExecutionSheinSyncStub{})
	_, err := service.ConfirmSDSRetirementRun(context.Background(), "run-1", &ConfirmSDSRetirementRunRequest{ConfirmedBy: "operator"})
	if err == nil || !strings.Contains(err.Error(), "tenant scope is required") {
		t.Fatalf("ConfirmSDSRetirementRun() error = %v, want tenant scope error", err)
	}
}

func TestSDSRetirementConfirmRunRequiresExplicitRequest(t *testing.T) {
	service := NewSDSRetirementService(&sdsRetirementExecutionRepoStub{}, nil, &sdsRetirementExecutionSheinSyncStub{})
	_, err := service.ConfirmSDSRetirementRun(WithTenantID(context.Background(), "18"), "run-1", nil)
	if err == nil || !strings.Contains(err.Error(), "confirm request is required") {
		t.Fatalf("ConfirmSDSRetirementRun() error = %v, want explicit request error", err)
	}
}

func TestSDSRetirementConfirmRunRejectsAsyncOnlyRefresh(t *testing.T) {
	repo := &sdsRetirementExecutionRepoStub{
		storedRun: &SDSRetirementRunRecord{
			ID:       "run-1",
			TenantID: "18",
			Platform: "shein",
			StoreID:  177,
			Status:   SDSRetirementRunStatusReady,
		},
		storedItems: []SDSRetirementItemRecord{{
			ID:            "item-1",
			RunID:         "run-1",
			TenantID:      "18",
			Platform:      "shein",
			StoreID:       177,
			SPUName:       "SPU-1",
			SKCName:       "SKC-1",
			BusinessModel: 2,
			Selected:      true,
			SiteSelection: `[{"site_abbr":"US","store_type":1}]`,
			Status:        SDSRetirementItemStatusSelected,
		}},
	}
	service := NewSDSRetirementService(repo, nil, &sdsRetirementExecutionSheinSyncStub{
		supportsImmediateRefresh: false,
		productAPI:               &sdsRetirementProductAPIStub{},
	})
	_, err := service.ConfirmSDSRetirementRun(WithTenantID(context.Background(), "18"), "run-1", &ConfirmSDSRetirementRunRequest{ConfirmedBy: "operator"})
	if err == nil || !strings.Contains(err.Error(), "cannot guarantee refreshed SHEIN preview data") {
		t.Fatalf("ConfirmSDSRetirementRun() error = %v, want async refresh safety error", err)
	}
	if repo.savedRun != nil {
		t.Fatalf("saved run = %#v, want none", repo.savedRun)
	}
}

type sdsRetirementExecutionRepoStub struct {
	storedRun          *SDSRetirementRunRecord
	storedItems        []SDSRetirementItemRecord
	savedRun           *SDSRetirementRunRecord
	savedItems         []SDSRetirementItemRecord
	markedOffShelfIDs  []int64
	markedOffShelfTime []time.Time
}

func (s *sdsRetirementExecutionRepoStub) CreateTask(context.Context, *Task) error { return nil }
func (s *sdsRetirementExecutionRepoStub) GetTask(context.Context, string) (*Task, error) {
	return nil, nil
}
func (s *sdsRetirementExecutionRepoStub) ListTasks(context.Context, *TaskListQuery) ([]Task, int64, error) {
	return nil, 0, nil
}
func (s *sdsRetirementExecutionRepoStub) MarkProcessing(context.Context, string) error { return nil }
func (s *sdsRetirementExecutionRepoStub) MarkCompleted(context.Context, string, *ListingKitResult) error {
	return nil
}
func (s *sdsRetirementExecutionRepoStub) MarkNeedsReview(context.Context, string, *ListingKitResult, string) error {
	return nil
}
func (s *sdsRetirementExecutionRepoStub) MarkFailed(context.Context, string, string) error {
	return nil
}
func (s *sdsRetirementExecutionRepoStub) MarkBlockedRetryable(context.Context, string, *RetryableBlock, string) error {
	return nil
}
func (s *sdsRetirementExecutionRepoStub) ListRecoverableTasks(context.Context, *RecoverableTaskQuery) ([]Task, error) {
	return nil, nil
}
func (s *sdsRetirementExecutionRepoStub) RecoverBlockedTaskNow(context.Context, string, time.Time) error {
	return nil
}
func (s *sdsRetirementExecutionRepoStub) BulkRecoverBlockedTasks(context.Context, *RecoverBlockedTasksQuery) (int64, error) {
	return 0, nil
}
func (s *sdsRetirementExecutionRepoStub) PrepareRetry(context.Context, string) error { return nil }
func (s *sdsRetirementExecutionRepoStub) IncrementRetryCount(context.Context, string) error {
	return nil
}
func (s *sdsRetirementExecutionRepoStub) SaveTaskResult(context.Context, string, *ListingKitResult) error {
	return nil
}
func (s *sdsRetirementExecutionRepoStub) CreateSDSRetirementRun(context.Context, *SDSRetirementRunRecord, []SDSRetirementItemRecord) error {
	return nil
}
func (s *sdsRetirementExecutionRepoStub) GetSDSRetirementRun(context.Context, string) (*SDSRetirementRunRecord, []SDSRetirementItemRecord, error) {
	if s.storedRun == nil {
		return nil, nil, nil
	}
	clonedRun := *s.storedRun
	return &clonedRun, append([]SDSRetirementItemRecord(nil), s.storedItems...), nil
}
func (s *sdsRetirementExecutionRepoStub) UpdateSDSRetirementItems(context.Context, string, []SDSRetirementItemSelectionUpdate) error {
	return nil
}
func (s *sdsRetirementExecutionRepoStub) SaveSDSRetirementExecution(_ context.Context, run *SDSRetirementRunRecord, items []SDSRetirementItemRecord) error {
	clonedRun := *run
	s.savedRun = &clonedRun
	s.savedItems = append([]SDSRetirementItemRecord(nil), items...)
	return nil
}
func (s *sdsRetirementExecutionRepoStub) MarkSyncedProductOffShelf(_ context.Context, syncedProductID int64, now time.Time) error {
	s.markedOffShelfIDs = append(s.markedOffShelfIDs, syncedProductID)
	s.markedOffShelfTime = append(s.markedOffShelfTime, now)
	return nil
}

type sdsRetirementExecutionSheinSyncStub struct {
	syncTenantID             int64
	syncStoreID              int64
	syncTrigger              SheinSyncTriggerMode
	supportsImmediateRefresh bool
	productAPI               sheinproduct.ProductAPI
	resolvedStoreID          int64
	resolveErr               error
	syncErr                  error
	events                   []string
}

func (s *sdsRetirementExecutionSheinSyncStub) SyncSheinOnShelfProducts(_ context.Context, tenantID, storeID int64, triggerMode SheinSyncTriggerMode) (*SheinSyncJobRecord, error) {
	s.syncTenantID = tenantID
	s.syncStoreID = storeID
	s.syncTrigger = triggerMode
	s.events = append(s.events, "sync")
	if s.syncErr != nil {
		return nil, s.syncErr
	}
	return &SheinSyncJobRecord{}, nil
}

func (s *sdsRetirementExecutionSheinSyncStub) ListSyncedProducts(context.Context, *SheinSyncedProductQuery) ([]SheinSyncedProductRecord, int64, error) {
	return nil, 0, nil
}

func (s *sdsRetirementExecutionSheinSyncStub) UpdateManualCostPrice(context.Context, int64, *float64) error {
	return nil
}

func (s *sdsRetirementExecutionSheinSyncStub) SupportsImmediateRefresh() bool {
	return s.supportsImmediateRefresh
}

func (s *sdsRetirementExecutionSheinSyncStub) ResolveProductAPI(_ context.Context, storeID int64) (sheinproduct.ProductAPI, error) {
	s.resolvedStoreID = storeID
	s.events = append(s.events, "resolve")
	if s.resolveErr != nil {
		return nil, s.resolveErr
	}
	return s.productAPI, nil
}

type sdsRetirementProductAPIStub struct {
	offShelfRequests []*sheinproduct.ShelfOperateRequest
	offShelfErr      error
	events           *[]string
}

func (s *sdsRetirementProductAPIStub) GetProduct(string) (*sheinproduct.Product, error) {
	return nil, nil
}
func (s *sdsRetirementProductAPIStub) UpdateProduct(*sheinproduct.Product) error { return nil }
func (s *sdsRetirementProductAPIStub) DeleteProduct(string) error                { return nil }
func (s *sdsRetirementProductAPIStub) GetPartInfo(int) (*sheinproduct.PartInfoResponse, error) {
	return nil, nil
}
func (s *sdsRetirementProductAPIStub) SaveDraftProduct(*sheinproduct.Product) (*sheinproduct.SheinResponse, string, error) {
	return nil, "", nil
}
func (s *sdsRetirementProductAPIStub) PublishProduct(*sheinproduct.Product) (*sheinproduct.SheinResponse, string, error) {
	return nil, "", nil
}
func (s *sdsRetirementProductAPIStub) ConfirmPublish(*sheinproduct.Product) (bool, string, error) {
	return false, "", nil
}
func (s *sdsRetirementProductAPIStub) Record(*sheinproduct.ProductRecordRequest) (*sheinproduct.RecordResponse, error) {
	return nil, nil
}
func (s *sdsRetirementProductAPIStub) ListProducts(int, int, *sheinproduct.ProductListRequest) (*sheinproduct.ProductListResponse, error) {
	return nil, nil
}
func (s *sdsRetirementProductAPIStub) QueryBrandList() (*sheinproduct.BrandListResponse, error) {
	return nil, nil
}
func (s *sdsRetirementProductAPIStub) QueryStock(*sheinproduct.StockQueryRequest) (*sheinproduct.StockQueryResponse, error) {
	return nil, nil
}
func (s *sdsRetirementProductAPIStub) QueryInventory(string) (*sheinproduct.InventoryQueryResponse, error) {
	return nil, nil
}
func (s *sdsRetirementProductAPIStub) UpdateInventory(*sheinproduct.InventoryUpdateRequest) error {
	return nil
}
func (s *sdsRetirementProductAPIStub) QueryPrice(string) (*sheinproduct.PriceQueryResponse, error) {
	return nil, nil
}
func (s *sdsRetirementProductAPIStub) QueryCostPrice(string, []string) (*sheinproduct.CostPriceQueryResponse, error) {
	return nil, nil
}
func (s *sdsRetirementProductAPIStub) OffShelf(request *sheinproduct.ShelfOperateRequest) error {
	if s.events != nil {
		*s.events = append(*s.events, "offshelf")
	}
	if s.offShelfErr != nil {
		return s.offShelfErr
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		return err
	}
	var cloned sheinproduct.ShelfOperateRequest
	if err := json.Unmarshal(encoded, &cloned); err != nil {
		return err
	}
	s.offShelfRequests = append(s.offShelfRequests, &cloned)
	return nil
}
func (s *sdsRetirementProductAPIStub) OnShelf(*sheinproduct.ShelfOperateRequest) error { return nil }

func (s *sdsRetirementSheinSyncStub) ResolveProductAPI(context.Context, int64) (sheinproduct.ProductAPI, error) {
	return &sdsRetirementProductAPIStub{}, nil
}

var _ SheinSyncService = (*sdsRetirementExecutionSheinSyncStub)(nil)
var _ sheinproduct.ProductAPI = (*sdsRetirementProductAPIStub)(nil)
