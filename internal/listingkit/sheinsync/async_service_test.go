package sheinsync

import (
	"context"
	"testing"
	"time"

	sheinproduct "task-processor/internal/shein/api/product"

	"github.com/stretchr/testify/require"
)

func TestAsyncSheinSyncServiceReturnsPendingJobBeforeBackgroundSyncCompletes(t *testing.T) {
	t.Parallel()

	repo := newSheinSyncServiceRepoStub()
	productAPI := &blockingSheinSyncProductAPIStub{
		release: make(chan struct{}),
		response: makeProductListResponse([]sheinproduct.ProductListItem{
			{
				SpuName:          "spu-1",
				ProductNameMulti: "Product One",
				ShelfStatus:      "ON_SHELF",
				SkcInfoList: []sheinproduct.SkcInfoItem{
					{SkcName: "skc-1", SupplierCode: "SUP-1"},
				},
			},
		}, 1),
	}
	service := NewAsyncSheinSyncService(repo, productAPI, &sheinSyncServiceCostResolverStub{})

	started := time.Now()
	job, err := service.SyncSheinOnShelfProducts(context.Background(), 7, 88, SheinSyncTriggerModeManual)
	require.NoError(t, err)
	require.Less(t, time.Since(started), 200*time.Millisecond)
	require.NotNil(t, job)
	require.NotZero(t, job.ID)
	require.Equal(t, SheinSyncJobStatusPending, job.Status)
	require.Nil(t, job.StartedAt)
	require.Nil(t, job.FinishedAt)

	jobs, total, err := repo.ListSyncJobs(context.Background(), &SheinSyncJobQuery{TenantID: 7, StoreID: 88, Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, jobs, 1)
	require.Equal(t, job.ID, jobs[0].ID)

	close(productAPI.release)

	require.Eventually(t, func() bool {
		rows, _, err := repo.ListSyncedProducts(context.Background(), &SheinSyncedProductQuery{
			TenantID: 7,
			StoreID:  88,
			Page:     1,
			PageSize: 10,
		})
		if err != nil || len(rows) != 1 {
			return false
		}
		jobs, _, err := repo.ListSyncJobs(context.Background(), &SheinSyncJobQuery{TenantID: 7, StoreID: 88, Page: 1, PageSize: 10})
		return err == nil && len(jobs) == 1 && jobs[0].Status == SheinSyncJobStatusSucceeded
	}, 3*time.Second, 20*time.Millisecond)
}

func TestAsyncSheinSyncServiceWithBuilderResolvesCostsUsingRuntimeProductAPI(t *testing.T) {
	t.Parallel()

	repo := newSheinSyncServiceRepoStub()
	productAPI := &blockingSheinSyncProductAPIStub{
		release: make(chan struct{}),
		response: makeProductListResponse([]sheinproduct.ProductListItem{
			{
				SpuName:          "spu-1",
				ProductNameMulti: "Product One",
				ShelfStatus:      "ON_SHELF",
				SkcInfoList: []sheinproduct.SkcInfoItem{
					{SkcName: "skc-1", SupplierCode: "SUP-1"},
				},
			},
		}, 1),
	}
	service := NewAsyncSheinSyncServiceWithBuilder(repo, sheinSyncProductAPIBuilderStub{productAPI: productAPI}, nil)

	job, err := service.SyncSheinOnShelfProducts(context.Background(), 7, 88, SheinSyncTriggerModeManual)
	require.NoError(t, err)
	require.NotNil(t, job)

	close(productAPI.release)

	require.Eventually(t, func() bool {
		jobs, _, err := repo.ListSyncJobs(context.Background(), &SheinSyncJobQuery{TenantID: 7, StoreID: 88, Page: 1, PageSize: 10})
		if err != nil || len(jobs) != 1 {
			return false
		}
		if jobs[0].Status != SheinSyncJobStatusSucceeded {
			return false
		}

		rows, _, err := repo.ListSyncedProducts(context.Background(), &SheinSyncedProductQuery{
			TenantID: 7,
			StoreID:  88,
			Page:     1,
			PageSize: 10,
		})
		return err == nil && len(rows) == 1 && rows[0].SupplyPrice != nil && *rows[0].SupplyPrice == 10.5
	}, 3*time.Second, 20*time.Millisecond)
}

func TestAsyncSheinSyncServiceResolveProductAPIDelegatesToRuntimeService(t *testing.T) {
	t.Parallel()

	repo := newSheinSyncServiceRepoStub()
	productAPI := &blockingSheinSyncProductAPIStub{}
	service := NewAsyncSheinSyncService(repo, productAPI, nil)

	resolved, err := service.ResolveProductAPI(context.Background(), 66)
	require.NoError(t, err)
	require.Same(t, productAPI, resolved)
}

type blockingSheinSyncProductAPIStub struct {
	release  chan struct{}
	response *sheinproduct.ProductListResponse
}

type sheinSyncProductAPIBuilderStub struct {
	productAPI sheinproduct.ProductAPI
	fallback   string
}

func (s sheinSyncProductAPIBuilderStub) BuildProductAPI(_ context.Context, _ int64) (sheinproduct.ProductAPI, string) {
	return s.productAPI, s.fallback
}

func (s *blockingSheinSyncProductAPIStub) GetProduct(string) (*sheinproduct.Product, error) {
	return nil, nil
}

func (s *blockingSheinSyncProductAPIStub) UpdateProduct(*sheinproduct.Product) error {
	return nil
}

func (s *blockingSheinSyncProductAPIStub) DeleteProduct(string) error {
	return nil
}

func (s *blockingSheinSyncProductAPIStub) GetPartInfo(int) (*sheinproduct.PartInfoResponse, error) {
	return nil, nil
}

func (s *blockingSheinSyncProductAPIStub) SaveDraftProduct(*sheinproduct.Product) (*sheinproduct.SheinResponse, string, error) {
	return nil, "", nil
}

func (s *blockingSheinSyncProductAPIStub) PublishProduct(*sheinproduct.Product) (*sheinproduct.SheinResponse, string, error) {
	return nil, "", nil
}

func (s *blockingSheinSyncProductAPIStub) ConfirmPublish(*sheinproduct.Product) (bool, string, error) {
	return false, "", nil
}

func (s *blockingSheinSyncProductAPIStub) Record(*sheinproduct.ProductRecordRequest) (*sheinproduct.RecordResponse, error) {
	return nil, nil
}

func (s *blockingSheinSyncProductAPIStub) ListProducts(pageNum, pageSize int, request *sheinproduct.ProductListRequest) (*sheinproduct.ProductListResponse, error) {
	<-s.release
	if pageNum > 1 {
		return makeProductListResponse(nil, 1), nil
	}
	return s.response, nil
}

func (s *blockingSheinSyncProductAPIStub) QueryBrandList() (*sheinproduct.BrandListResponse, error) {
	return nil, nil
}

func (s *blockingSheinSyncProductAPIStub) QueryProductNameLengthConfig(int) ([]sheinproduct.NameLengthConfigItem, error) {
	return nil, nil
}

func (s *blockingSheinSyncProductAPIStub) QueryLanguageList() ([]sheinproduct.LanguageListItem, error) {
	return nil, nil
}

func (s *blockingSheinSyncProductAPIStub) QuerySiteList() ([]sheinproduct.SiteListGroup, error) {
	return nil, nil
}

func (s *blockingSheinSyncProductAPIStub) QueryStock(*sheinproduct.StockQueryRequest) (*sheinproduct.StockQueryResponse, error) {
	return nil, nil
}

func (s *blockingSheinSyncProductAPIStub) QueryInventory(string) (*sheinproduct.InventoryQueryResponse, error) {
	return nil, nil
}

func (s *blockingSheinSyncProductAPIStub) UpdateInventory(*sheinproduct.InventoryUpdateRequest) error {
	return nil
}

func (s *blockingSheinSyncProductAPIStub) QueryPrice(string) (*sheinproduct.PriceQueryResponse, error) {
	return nil, nil
}

func (s *blockingSheinSyncProductAPIStub) QueryCostPrice(string, []string) (*sheinproduct.CostPriceQueryResponse, error) {
	return &sheinproduct.CostPriceQueryResponse{
		Info: sheinproduct.CostPriceInfo{
			Data: []sheinproduct.SkcCostData{
				{
					SkcName: "skc-1",
					SkuCostInfoList: []sheinproduct.SkuCostInfo{
						{
							CostPriceInfo: sheinproduct.CostPrice{
								CostPrice: "10.5",
								Currency:  "USD",
							},
						},
					},
				},
			},
		},
	}, nil
}

func (s *blockingSheinSyncProductAPIStub) OffShelf(*sheinproduct.ShelfOperateRequest) error {
	return nil
}

func (s *blockingSheinSyncProductAPIStub) OnShelf(*sheinproduct.ShelfOperateRequest) error {
	return nil
}
