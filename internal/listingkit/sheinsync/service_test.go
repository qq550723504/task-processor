package sheinsync

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	sheinproduct "task-processor/internal/shein/api/product"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSyncSheinOnShelfProductsUsesOnShelfRequestAndPersistsRows(t *testing.T) {
	t.Parallel()

	repo := newSheinSyncServiceRepoStub()
	firstPageItems := make([]sheinproduct.ProductListItem, 0, 100)
	firstPageItems = append(firstPageItems, sheinproduct.ProductListItem{
		SpuName:          "spu-1",
		SpuCode:          "SPU001",
		CategoryID:       1001,
		BrandName:        "SHEIN",
		ProductNameMulti: "Product One",
		ShelfStatus:      "ON_SHELF",
		PublishTime:      "2026-06-01 08:30:00",
		FirstShelfTime:   "2026-06-01 09:00:00",
		SkcInfoList: []sheinproduct.SkcInfoItem{
			{SkcName: "skc-1", SkcCode: "SKC001", SaleName: "Red", SupplierCode: "SUP-1", MainImageThumbnailURL: "https://img/1.jpg"},
			{SkcName: "skc-2", SkcCode: "SKC002", SaleName: "Blue", SupplierCode: "SUP-2", MainImageThumbnailURL: "https://img/2.jpg"},
		},
	})
	for i := 0; i < 99; i++ {
		firstPageItems = append(firstPageItems, sheinproduct.ProductListItem{
			SpuName:     fmt.Sprintf("placeholder-%d", i),
			ShelfStatus: "ON_SHELF",
		})
	}
	productAPI := &sheinSyncServiceProductAPIStub{
		listResponses: []*sheinproduct.ProductListResponse{
			makeProductListResponse(firstPageItems, 101),
			makeProductListResponse([]sheinproduct.ProductListItem{
				{
					SpuName:          "spu-2",
					SpuCode:          "SPU002",
					CategoryID:       1002,
					BrandName:        "SHEIN",
					ProductNameMulti: "Product Two",
					ShelfStatus:      "ON_SHELF",
					SkcInfoList: []sheinproduct.SkcInfoItem{
						{SkcName: "skc-3", SkcCode: "SKC003", SaleName: "Green", SupplierCode: "SUP-3", MainImageThumbnailURL: "https://img/3.jpg"},
					},
				},
			}, 101),
		},
		queryPriceResp: makePriceQueryResponse([]sheinproduct.SkcPriceData{
			{
				SkcName: "skc-1",
				SkuInfoList: []sheinproduct.SkuPriceInfo{
					{
						SkuCode: "SKU001",
						PriceInfoList: []sheinproduct.SkuPriceDetail{
							{Currency: "USD", ShopPrice: 34.17},
						},
					},
				},
			},
		}),
		queryInventoryResp: makeInventoryQueryResponse([]sheinproduct.SkcInventory{
			{
				SkcName: "skc-1",
				SkuInfo: []sheinproduct.SkuInventory{
					{
						SkuCode: "SKU001",
						InventoryInfo: []sheinproduct.WarehouseInventory{
							{InventoryQuantity: 999, UsableInventory: 321},
						},
					},
				},
			},
		}),
	}
	costResolver := &sheinSyncServiceCostResolverStub{
		autoCosts: map[string]resolvedSheinCost{
			"spu-1|skc-1": {CostPrice: float64Ptr(11.2), Currency: "USD"},
			"spu-1|skc-2": {CostPrice: float64Ptr(12.3), Currency: "USD"},
			"spu-2|skc-3": {CostPrice: float64Ptr(13.4), Currency: "USD"},
		},
	}

	service := NewSheinSyncService(repo, productAPI, costResolver)

	job, err := service.SyncSheinOnShelfProducts(context.Background(), 11, 22, SheinSyncTriggerModeManual)
	require.NoError(t, err)

	require.Equal(t, 3, job.FetchedCount)
	require.Equal(t, 3, job.InsertedCount)
	require.Equal(t, 0, job.UpdatedCount)
	require.Equal(t, SheinSyncJobStatusSucceeded, job.Status)
	require.Len(t, productAPI.listCalls, 2)
	require.Equal(t, 1, productAPI.listCalls[0].pageNum)
	require.Equal(t, 2, productAPI.listCalls[1].pageNum)
	require.Equal(t, 100, productAPI.listCalls[0].pageSize)
	require.Equal(t, "ON_SHELF", productAPI.listCalls[0].request.ShelfType)
	require.Equal(t, 1, productAPI.listCalls[0].request.SortType)
	require.Equal(t, "ON_SHELF", productAPI.listCalls[1].request.ShelfType)
	require.Equal(t, 1, productAPI.listCalls[1].request.SortType)

	rows, total, err := repo.ListSyncedProducts(context.Background(), &SheinSyncedProductQuery{TenantID: 11, StoreID: 22, Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	require.Len(t, rows, 3)
	require.Equal(t, "skc-1", rows[0].SKCName)
	require.True(t, rows[0].IsActive)
	require.Equal(t, "USD", rows[0].Currency)
	require.NotNil(t, rows[0].PublishTime)
	require.NotNil(t, rows[0].FirstShelfTime)
	require.JSONEq(t, `{"sale_price":34.17,"currency":"USD","sub_site":""}`, rows[0].PriceSnapshot)
	require.JSONEq(t, `{"total":999,"available":321}`, rows[0].InventorySnapshot)
}

func TestSyncSheinOnShelfProductsManualOverrideWinsOverAutoCost(t *testing.T) {
	t.Parallel()

	repo := newSheinSyncServiceRepoStub()
	repo.seedProduct(SheinSyncedProductRecord{
		TenantID:           1,
		StoreID:            101,
		SPUName:            "spu-1",
		SKCName:            "skc-1",
		ManualCostPrice:    float64Ptr(19.8),
		EffectiveCostPrice: float64Ptr(19.8),
		CostPriceSource:    SheinCostPriceSourceManual,
		IsActive:           true,
	})

	productAPI := &sheinSyncServiceProductAPIStub{
		listResponses: []*sheinproduct.ProductListResponse{
			makeProductListResponse([]sheinproduct.ProductListItem{
				{
					SpuName:          "spu-1",
					ProductNameMulti: "Product One",
					ShelfStatus:      "ON_SHELF",
					SkcInfoList: []sheinproduct.SkcInfoItem{
						{SkcName: "skc-1", SupplierCode: "SUP-1"},
					},
				},
			}, 1),
		},
	}
	costResolver := &sheinSyncServiceCostResolverStub{
		autoCosts: map[string]resolvedSheinCost{
			"spu-1|skc-1": {CostPrice: float64Ptr(12.5), Currency: "USD"},
		},
	}

	service := NewSheinSyncService(repo, productAPI, costResolver)

	_, err := service.SyncSheinOnShelfProducts(context.Background(), 1, 101, SheinSyncTriggerModeManual)
	require.NoError(t, err)

	rows, total, err := repo.ListSyncedProducts(context.Background(), &SheinSyncedProductQuery{TenantID: 1, StoreID: 101, Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, rows, 1)
	require.NotNil(t, rows[0].ManualCostPrice)
	require.NotNil(t, rows[0].AutoCostPrice)
	require.NotNil(t, rows[0].EffectiveCostPrice)
	require.Equal(t, 19.8, *rows[0].ManualCostPrice)
	require.Equal(t, 12.5, *rows[0].AutoCostPrice)
	require.Equal(t, 19.8, *rows[0].EffectiveCostPrice)
	require.Equal(t, SheinCostPriceSourceManual, rows[0].CostPriceSource)
}

func TestSyncSheinOnShelfProductsPreservesExistingAutoCostWhenResolverOmitsSKC(t *testing.T) {
	t.Parallel()

	repo := newSheinSyncServiceRepoStub()
	repo.seedProduct(SheinSyncedProductRecord{
		TenantID:           5,
		StoreID:            55,
		SPUName:            "spu-1",
		SKCName:            "skc-1",
		AutoCostPrice:      float64Ptr(16.6),
		EffectiveCostPrice: float64Ptr(16.6),
		CostPriceSource:    SheinCostPriceSourceAuto,
		Currency:           "USD",
		IsActive:           true,
	})

	productAPI := &sheinSyncServiceProductAPIStub{
		listResponses: []*sheinproduct.ProductListResponse{
			makeProductListResponse([]sheinproduct.ProductListItem{
				{
					SpuName:          "spu-1",
					ProductNameMulti: "Product One",
					ShelfStatus:      "ON_SHELF",
					SkcInfoList: []sheinproduct.SkcInfoItem{
						{SkcName: "skc-1", SupplierCode: "SUP-1"},
					},
				},
			}, 1),
		},
	}
	costResolver := &sheinSyncServiceCostResolverStub{
		autoCosts: map[string]resolvedSheinCost{},
	}

	service := NewSheinSyncService(repo, productAPI, costResolver)

	_, err := service.SyncSheinOnShelfProducts(context.Background(), 5, 55, SheinSyncTriggerModeManual)
	require.NoError(t, err)

	rows, total, err := repo.ListSyncedProducts(context.Background(), &SheinSyncedProductQuery{TenantID: 5, StoreID: 55, Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, rows, 1)
	require.NotNil(t, rows[0].AutoCostPrice)
	require.NotNil(t, rows[0].EffectiveCostPrice)
	require.Equal(t, 16.6, *rows[0].AutoCostPrice)
	require.Equal(t, 16.6, *rows[0].EffectiveCostPrice)
	require.Equal(t, "USD", rows[0].Currency)
	require.Equal(t, SheinCostPriceSourceAuto, rows[0].CostPriceSource)
}

func TestSyncSheinOnShelfProductsMarksMissingSKCsInactive(t *testing.T) {
	t.Parallel()

	repo := newSheinSyncServiceRepoStub()
	productAPI := &sheinSyncServiceProductAPIStub{
		listResponses: []*sheinproduct.ProductListResponse{
			makeProductListResponse([]sheinproduct.ProductListItem{
				{
					SpuName:          "spu-1",
					ProductNameMulti: "Product One",
					ShelfStatus:      "ON_SHELF",
					SkcInfoList: []sheinproduct.SkcInfoItem{
						{SkcName: "skc-1"},
						{SkcName: "skc-2"},
					},
				},
			}, 2),
		},
	}
	costResolver := &sheinSyncServiceCostResolverStub{}
	service := NewSheinSyncService(repo, productAPI, costResolver)

	_, err := service.SyncSheinOnShelfProducts(context.Background(), 7, 88, SheinSyncTriggerModeManual)
	require.NoError(t, err)

	productAPI.listResponses = []*sheinproduct.ProductListResponse{
		makeProductListResponse([]sheinproduct.ProductListItem{
			{
				SpuName:          "spu-1",
				ProductNameMulti: "Product One",
				ShelfStatus:      "ON_SHELF",
				SkcInfoList: []sheinproduct.SkcInfoItem{
					{SkcName: "skc-1"},
				},
			},
		}, 1),
	}
	productAPI.listCalls = nil

	job, err := service.SyncSheinOnShelfProducts(context.Background(), 7, 88, SheinSyncTriggerModeSchedule)
	require.NoError(t, err)
	require.Equal(t, SheinSyncTriggerModeSchedule, job.TriggerMode)
	require.Equal(t, 1, job.DeactivatedCount)

	active := true
	rows, total, err := repo.ListSyncedProducts(context.Background(), &SheinSyncedProductQuery{TenantID: 7, StoreID: 88, Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, rows, 2)
	require.Equal(t, "skc-1", rows[0].SKCName)
	require.Equal(t, "skc-2", rows[1].SKCName)
	require.True(t, rows[0].IsActive)
	require.False(t, rows[1].IsActive)

	activeRows, activeTotal, err := repo.ListSyncedProducts(context.Background(), &SheinSyncedProductQuery{TenantID: 7, StoreID: 88, IsActive: &active, Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(1), activeTotal)
	require.Len(t, activeRows, 1)
	require.Equal(t, "skc-1", activeRows[0].SKCName)
}

func TestSyncSheinOnShelfProductsMarksJobFailedWhenListProductsFails(t *testing.T) {
	t.Parallel()

	repo := newSheinSyncServiceRepoStub()
	productAPI := &sheinSyncServiceProductAPIStub{
		listErr: errors.New("shein unavailable"),
	}
	service := NewSheinSyncService(repo, productAPI, &sheinSyncServiceCostResolverStub{})

	job, err := service.SyncSheinOnShelfProducts(context.Background(), 3, 33, SheinSyncTriggerModeManual)
	require.Error(t, err)
	require.Nil(t, job)

	jobs, total, err := repo.ListSyncJobs(context.Background(), &SheinSyncJobQuery{TenantID: 3, StoreID: 33, Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, jobs, 1)
	require.Equal(t, SheinSyncJobStatusFailed, jobs[0].Status)
	require.Contains(t, jobs[0].ErrorSummary, "shein unavailable")
	require.NotNil(t, jobs[0].FinishedAt)
}

func TestSyncSheinOnShelfProductsReturnsClearErrorWhenProductAPIMissing(t *testing.T) {
	t.Parallel()

	service := NewSheinSyncService(newSheinSyncServiceRepoStub(), nil, &sheinSyncServiceCostResolverStub{})

	job, err := service.SyncSheinOnShelfProducts(context.Background(), 6, 66, SheinSyncTriggerModeManual)
	require.Nil(t, job)
	require.Error(t, err)
	require.ErrorContains(t, err, "product API is required")
}

func TestSyncSheinOnShelfProductsReturnsClearErrorWhenListProductsResponseIsNil(t *testing.T) {
	t.Parallel()

	repo := newSheinSyncServiceRepoStub()
	productAPI := &sheinSyncServiceProductAPIStub{
		listResponses: []*sheinproduct.ProductListResponse{nil},
	}
	service := NewSheinSyncService(repo, productAPI, &sheinSyncServiceCostResolverStub{})

	job, err := service.SyncSheinOnShelfProducts(context.Background(), 8, 88, SheinSyncTriggerModeManual)
	require.Nil(t, job)
	require.Error(t, err)
	require.ErrorContains(t, err, "returned nil response")
}

func TestSyncSheinOnShelfProductsReturnsPersistenceErrorWhenFailedJobStateCannotBeSaved(t *testing.T) {
	t.Parallel()

	repo := newSheinSyncServiceRepoStub()
	repo.saveFailedJobErr = errors.New("save failed job state")
	productAPI := &sheinSyncServiceProductAPIStub{
		listErr: errors.New("shein unavailable"),
	}
	service := NewSheinSyncService(repo, productAPI, &sheinSyncServiceCostResolverStub{})

	job, err := service.SyncSheinOnShelfProducts(context.Background(), 4, 44, SheinSyncTriggerModeManual)
	require.Nil(t, job)
	require.Error(t, err)
	require.ErrorContains(t, err, "save failed job state")
}

func TestSheinCostResolverReturnsClearErrorWhenQueryCostPriceResponseIsNil(t *testing.T) {
	t.Parallel()

	resolver := NewSheinCostResolver(&sheinSyncServiceProductAPIStub{})

	_, err := resolver.ResolveAutoCosts(context.Background(), sheinproduct.ProductListItem{
		SpuName: "spu-1",
		SkcInfoList: []sheinproduct.SkcInfoItem{
			{SkcName: "skc-1"},
		},
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "returned nil response")
}

func TestSyncSheinOnShelfProductsResolvesCostsConcurrentlyWithinPage(t *testing.T) {
	t.Parallel()

	repo := newSheinSyncServiceRepoStub()
	productAPI := &sheinSyncServiceProductAPIStub{
		listResponses: []*sheinproduct.ProductListResponse{
			makeProductListResponse([]sheinproduct.ProductListItem{
				{
					SpuName:          "spu-1",
					ProductNameMulti: "Product One",
					ShelfStatus:      "ON_SHELF",
					SkcInfoList: []sheinproduct.SkcInfoItem{
						{SkcName: "skc-1", SupplierCode: "SUP-1"},
					},
				},
				{
					SpuName:          "spu-2",
					ProductNameMulti: "Product Two",
					ShelfStatus:      "ON_SHELF",
					SkcInfoList: []sheinproduct.SkcInfoItem{
						{SkcName: "skc-2", SupplierCode: "SUP-2"},
					},
				},
				{
					SpuName:          "spu-3",
					ProductNameMulti: "Product Three",
					ShelfStatus:      "ON_SHELF",
					SkcInfoList: []sheinproduct.SkcInfoItem{
						{SkcName: "skc-3", SupplierCode: "SUP-3"},
					},
				},
			}, 3),
		},
	}
	costResolver := newBlockingConcurrentCostResolver(3)
	service := NewSheinSyncService(repo, productAPI, costResolver)

	done := make(chan error, 1)
	go func() {
		_, err := service.SyncSheinOnShelfProducts(context.Background(), 12, 34, SheinSyncTriggerModeManual)
		done <- err
	}()

	require.Eventually(t, func() bool {
		return costResolver.startedCount() == 3
	}, time.Second, 10*time.Millisecond)

	costResolver.releaseAll()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("sync did not finish after releasing concurrent cost resolver")
	}
}

type sheinSyncServiceRepoStub struct {
	mu       sync.RWMutex
	nextID   int64
	nextJob  int64
	products map[string]SheinSyncedProductRecord
	jobs     map[int64]SheinSyncJobRecord

	saveFailedJobErr error
}

type blockingConcurrentCostResolver struct {
	mu        sync.Mutex
	started   int
	release   chan struct{}
	autoCosts map[string]resolvedSheinCost
}

func newBlockingConcurrentCostResolver(expected int) *blockingConcurrentCostResolver {
	autoCosts := make(map[string]resolvedSheinCost, expected)
	for i := 1; i <= expected; i++ {
		autoCosts[fmt.Sprintf("spu-%d|skc-%d", i, i)] = resolvedSheinCost{
			CostPrice: float64Ptr(float64(10 + i)),
			Currency:  "USD",
		}
	}
	return &blockingConcurrentCostResolver{
		release:   make(chan struct{}),
		autoCosts: autoCosts,
	}
}

func (r *blockingConcurrentCostResolver) ResolveAutoCosts(_ context.Context, product sheinproduct.ProductListItem) (map[string]resolvedSheinCost, error) {
	r.mu.Lock()
	r.started++
	r.mu.Unlock()

	<-r.release

	resolved := map[string]resolvedSheinCost{}
	for _, skc := range product.SkcInfoList {
		if cost, ok := r.autoCosts[product.SpuName+"|"+skc.SkcName]; ok {
			resolved[skc.SkcName] = cost
		}
	}
	return resolved, nil
}

func (r *blockingConcurrentCostResolver) startedCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.started
}

func (r *blockingConcurrentCostResolver) releaseAll() {
	close(r.release)
}

func newSheinSyncServiceRepoStub() *sheinSyncServiceRepoStub {
	return &sheinSyncServiceRepoStub{
		nextID:   1,
		nextJob:  1,
		products: make(map[string]SheinSyncedProductRecord),
		jobs:     make(map[int64]SheinSyncJobRecord),
	}
}

func (r *sheinSyncServiceRepoStub) seedProduct(record SheinSyncedProductRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if record.ID == 0 {
		record.ID = r.nextID
		r.nextID++
	}
	r.products[r.productKey(record.TenantID, record.StoreID, record.SKCName)] = cloneServiceTestProduct(record)
}

func (r *sheinSyncServiceRepoStub) UpsertSyncedProducts(_ context.Context, records []*SheinSyncedProductRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, record := range records {
		if record == nil {
			continue
		}
		key := r.productKey(record.TenantID, record.StoreID, record.SKCName)
		row := cloneServiceTestProduct(*record)
		if existing, ok := r.products[key]; ok {
			row.ID = existing.ID
		} else {
			row.ID = r.nextID
			r.nextID++
		}
		ApplyEffectiveCostPrice(&row)
		r.products[key] = row
	}
	return nil
}

func (r *sheinSyncServiceRepoStub) ListSyncedProducts(_ context.Context, query *SheinSyncedProductQuery) ([]SheinSyncedProductRecord, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]SheinSyncedProductRecord, 0, len(r.products))
	for _, row := range r.products {
		if query != nil {
			if query.TenantID > 0 && row.TenantID != query.TenantID {
				continue
			}
			if query.StoreID > 0 && row.StoreID != query.StoreID {
				continue
			}
			if query.SKCName != "" && row.SKCName != query.SKCName {
				continue
			}
			if query.IsActive != nil && row.IsActive != *query.IsActive {
				continue
			}
		}
		items = append(items, cloneServiceTestProduct(row))
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].SKCName < items[j].SKCName
	})

	total := int64(len(items))
	page, pageSize := 1, len(items)
	if query != nil {
		if query.Page > 0 {
			page = query.Page
		}
		if query.PageSize > 0 {
			pageSize = query.PageSize
		}
	}
	if pageSize == 0 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start >= len(items) {
		return []SheinSyncedProductRecord{}, total, nil
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end], total, nil
}

func (r *sheinSyncServiceRepoStub) UpdateManualCostPrice(_ context.Context, productID int64, manualCostPrice *float64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for key, row := range r.products {
		if row.ID != productID {
			continue
		}
		row.ManualCostPrice = cloneServiceTestFloat64(manualCostPrice)
		ApplyEffectiveCostPrice(&row)
		r.products[key] = row
		return nil
	}
	return gorm.ErrRecordNotFound
}

func (r *sheinSyncServiceRepoStub) MarkMissingSyncedProductsInactive(_ context.Context, tenantID, storeID int64, activeSKCNames []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	activeSet := make(map[string]struct{}, len(activeSKCNames))
	for _, skcName := range activeSKCNames {
		activeSet[skcName] = struct{}{}
	}
	for key, row := range r.products {
		if row.TenantID != tenantID || row.StoreID != storeID {
			continue
		}
		if _, ok := activeSet[row.SKCName]; ok {
			continue
		}
		row.IsActive = false
		r.products[key] = row
	}
	return nil
}

func (r *sheinSyncServiceRepoStub) SaveSyncJob(_ context.Context, job *SheinSyncJobRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if job == nil {
		return nil
	}
	if job.Status == SheinSyncJobStatusFailed && r.saveFailedJobErr != nil {
		return r.saveFailedJobErr
	}
	row := *job
	if row.ID == 0 {
		row.ID = r.nextJob
		r.nextJob++
	}
	r.jobs[row.ID] = row
	*job = row
	return nil
}

func (r *sheinSyncServiceRepoStub) ListSyncJobs(_ context.Context, query *SheinSyncJobQuery) ([]SheinSyncJobRecord, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]SheinSyncJobRecord, 0, len(r.jobs))
	for _, row := range r.jobs {
		if query != nil {
			if query.TenantID > 0 && row.TenantID != query.TenantID {
				continue
			}
			if query.StoreID > 0 && row.StoreID != query.StoreID {
				continue
			}
			if query.TriggerMode != nil && row.TriggerMode != *query.TriggerMode {
				continue
			}
			if query.Status != nil && row.Status != *query.Status {
				continue
			}
		}
		items = append(items, row)
	}
	return items, int64(len(items)), nil
}

func (r *sheinSyncServiceRepoStub) SaveCandidates(_ context.Context, _ []*SheinActivityCandidateRecord) error {
	return nil
}

func (r *sheinSyncServiceRepoStub) CreateEnrollmentRun(_ context.Context, _ *SheinActivityEnrollmentRunRecord) error {
	return nil
}

func (r *sheinSyncServiceRepoStub) UpdateEnrollmentRun(_ context.Context, _ *SheinActivityEnrollmentRunRecord) error {
	return nil
}

func (r *sheinSyncServiceRepoStub) ListEnrollmentRuns(_ context.Context, _ *SheinEnrollmentRunQuery) ([]SheinActivityEnrollmentRunRecord, int64, error) {
	return nil, 0, nil
}

func (r *sheinSyncServiceRepoStub) SaveEnrollmentItems(_ context.Context, _ []*SheinActivityEnrollmentItemRecord) error {
	return nil
}

func (r *sheinSyncServiceRepoStub) productKey(tenantID, storeID int64, skcName string) string {
	return fmt.Sprintf("%d|%d|%s", tenantID, storeID, skcName)
}

type sheinSyncServiceProductAPIStub struct {
	listResponses      []*sheinproduct.ProductListResponse
	listErr            error
	listCalls          []sheinProductListCall
	queryPriceResp     *sheinproduct.PriceQueryResponse
	queryPriceErr      error
	queryInventoryResp *sheinproduct.InventoryQueryResponse
	queryInventoryErr  error
	queryCostPriceResp *sheinproduct.CostPriceQueryResponse
	queryCostPriceErr  error
}

type sheinProductListCall struct {
	pageNum  int
	pageSize int
	request  *sheinproduct.ProductListRequest
}

func (s *sheinSyncServiceProductAPIStub) GetProduct(string) (*sheinproduct.Product, error) {
	return nil, errors.New("not implemented")
}

func (s *sheinSyncServiceProductAPIStub) UpdateProduct(*sheinproduct.Product) error {
	return errors.New("not implemented")
}

func (s *sheinSyncServiceProductAPIStub) DeleteProduct(string) error {
	return errors.New("not implemented")
}

func (s *sheinSyncServiceProductAPIStub) GetPartInfo(int) (*sheinproduct.PartInfoResponse, error) {
	return nil, errors.New("not implemented")
}

func (s *sheinSyncServiceProductAPIStub) SaveDraftProduct(*sheinproduct.Product) (*sheinproduct.SheinResponse, string, error) {
	return nil, "", errors.New("not implemented")
}

func (s *sheinSyncServiceProductAPIStub) PublishProduct(*sheinproduct.Product) (*sheinproduct.SheinResponse, string, error) {
	return nil, "", errors.New("not implemented")
}

func (s *sheinSyncServiceProductAPIStub) ConfirmPublish(*sheinproduct.Product) (bool, string, error) {
	return false, "", errors.New("not implemented")
}

func (s *sheinSyncServiceProductAPIStub) Record(*sheinproduct.ProductRecordRequest) (*sheinproduct.RecordResponse, error) {
	return nil, errors.New("not implemented")
}

func (s *sheinSyncServiceProductAPIStub) ListProducts(pageNum, pageSize int, request *sheinproduct.ProductListRequest) (*sheinproduct.ProductListResponse, error) {
	s.listCalls = append(s.listCalls, sheinProductListCall{
		pageNum:  pageNum,
		pageSize: pageSize,
		request:  cloneProductListRequest(request),
	})
	if s.listErr != nil {
		return nil, s.listErr
	}
	if len(s.listResponses) == 0 {
		return makeProductListResponse(nil, 0), nil
	}
	index := pageNum - 1
	if index < 0 || index >= len(s.listResponses) {
		return makeProductListResponse(nil, 0), nil
	}
	return s.listResponses[index], nil
}

func (s *sheinSyncServiceProductAPIStub) QueryBrandList() (*sheinproduct.BrandListResponse, error) {
	return nil, errors.New("not implemented")
}

func (s *sheinSyncServiceProductAPIStub) QueryStock(*sheinproduct.StockQueryRequest) (*sheinproduct.StockQueryResponse, error) {
	return nil, errors.New("not implemented")
}

func (s *sheinSyncServiceProductAPIStub) QueryInventory(string) (*sheinproduct.InventoryQueryResponse, error) {
	return s.queryInventoryResp, s.queryInventoryErr
}

func (s *sheinSyncServiceProductAPIStub) UpdateInventory(*sheinproduct.InventoryUpdateRequest) error {
	return errors.New("not implemented")
}

func (s *sheinSyncServiceProductAPIStub) QueryPrice(string) (*sheinproduct.PriceQueryResponse, error) {
	return s.queryPriceResp, s.queryPriceErr
}

func (s *sheinSyncServiceProductAPIStub) QueryCostPrice(string, []string) (*sheinproduct.CostPriceQueryResponse, error) {
	return s.queryCostPriceResp, s.queryCostPriceErr
}

func (s *sheinSyncServiceProductAPIStub) OffShelf(*sheinproduct.ShelfOperateRequest) error {
	return errors.New("not implemented")
}

func (s *sheinSyncServiceProductAPIStub) OnShelf(*sheinproduct.ShelfOperateRequest) error {
	return errors.New("not implemented")
}

type sheinSyncServiceCostResolverStub struct {
	autoCosts map[string]resolvedSheinCost
	err       error
}

func (s *sheinSyncServiceCostResolverStub) ResolveAutoCosts(_ context.Context, product sheinproduct.ProductListItem) (map[string]resolvedSheinCost, error) {
	if s.err != nil {
		return nil, s.err
	}
	resolved := make(map[string]resolvedSheinCost)
	for _, skc := range product.SkcInfoList {
		key := product.SpuName + "|" + skc.SkcName
		if cost, ok := s.autoCosts[key]; ok {
			resolved[skc.SkcName] = cost
		}
	}
	return resolved, nil
}

func makeProductListResponse(items []sheinproduct.ProductListItem, total int) *sheinproduct.ProductListResponse {
	resp := &sheinproduct.ProductListResponse{Code: "0", Msg: "ok"}
	resp.Info.Data = append(resp.Info.Data, items...)
	resp.Info.Meta.Count = total
	return resp
}

func makePriceQueryResponse(items []sheinproduct.SkcPriceData) *sheinproduct.PriceQueryResponse {
	resp := &sheinproduct.PriceQueryResponse{Code: "0", Msg: "ok"}
	resp.Info.Data = append(resp.Info.Data, items...)
	resp.Info.Meta.Count = len(items)
	return resp
}

func makeInventoryQueryResponse(items []sheinproduct.SkcInventory) *sheinproduct.InventoryQueryResponse {
	resp := &sheinproduct.InventoryQueryResponse{Code: "0", Msg: "ok"}
	resp.Info.SkcInfo = append(resp.Info.SkcInfo, items...)
	return resp
}

func cloneProductListRequest(request *sheinproduct.ProductListRequest) *sheinproduct.ProductListRequest {
	if request == nil {
		return nil
	}
	row := *request
	return &row
}

func cloneServiceTestProduct(row SheinSyncedProductRecord) SheinSyncedProductRecord {
	row.PublishTime = cloneServiceTestTime(row.PublishTime)
	row.FirstShelfTime = cloneServiceTestTime(row.FirstShelfTime)
	row.LastSyncAt = cloneServiceTestTime(row.LastSyncAt)
	row.AutoCostPrice = cloneServiceTestFloat64(row.AutoCostPrice)
	row.ManualCostPrice = cloneServiceTestFloat64(row.ManualCostPrice)
	row.EffectiveCostPrice = cloneServiceTestFloat64(row.EffectiveCostPrice)
	return row
}

func cloneServiceTestFloat64(v *float64) *float64 {
	if v == nil {
		return nil
	}
	copied := *v
	return &copied
}

func cloneServiceTestTime(v *time.Time) *time.Time {
	if v == nil {
		return nil
	}
	copied := *v
	return &copied
}
