package publish

import (
	"testing"

	"task-processor/internal/listingadmin"
	"task-processor/internal/listingruntime"
	"task-processor/internal/model"
	sheinproduct "task-processor/internal/shein/api/product"
	"task-processor/internal/state"
)

type fakeDailyCountProvider struct {
	client *fakeDailyCountClient
}

func (p fakeDailyCountProvider) GetDailyListingCountClient() listingadmin.DailyListingCountAPI {
	return p.client
}

type fakeDailyCountClient struct {
	count int64
}

func (c *fakeDailyCountClient) GetDailyListingCount(tenantID, storeID, userID int64, date string) (*listingadmin.DailyListingCountRespDTO, error) {
	return &listingadmin.DailyListingCountRespDTO{TenantID: tenantID, StoreID: storeID, UserID: userID, Date: date, Count: c.count}, nil
}

func (c *fakeDailyCountClient) SetDailyListingCount(req *listingadmin.DailyListingCountSetReqDTO) error {
	c.count = req.Count
	return nil
}

func (c *fakeDailyCountClient) TryConsumeDailyQuota(req *listingadmin.TryConsumeDailyQuotaReqDTO) (*listingadmin.TryConsumeDailyQuotaRespDTO, error) {
	return &listingadmin.TryConsumeDailyQuotaRespDTO{Allowed: true, NewCount: c.count + req.Increment}, nil
}

func (c *fakeDailyCountClient) RollbackDailyQuota(req *listingadmin.RollbackDailyQuotaReqDTO) (int64, error) {
	return c.count - req.Decrement, nil
}

func (c *fakeDailyCountClient) SetRemainingListingQuota(int64, int64, int) (bool, error) {
	return true, nil
}

func TestSavePublishResultCalculateIncrementUsesStoreLimitType(t *testing.T) {
	handler := NewSavePublishResultHandler()
	limit := 100
	input := &PublishResultInput{
		StoreInfo: &listingruntime.StoreInfo{
			DailyLimit:     &limit,
			DailyLimitType: "SKU",
		},
		SheinResponse: &sheinproduct.SheinResponse{
			Info: sheinproduct.ResponseInfo{
				SKCList: []sheinproduct.ResponseSKC{
					{
						SKUList: []sheinproduct.ResponseSKU{
							{SKUCode: "sku-1"},
							{SKUCode: "sku-2"},
						},
					},
					{
						SKUList: []sheinproduct.ResponseSKU{
							{SKUCode: "sku-3"},
						},
					},
				},
			},
		},
	}

	got := handler.calculateIncrement(input)
	if got != 3 {
		t.Fatalf("expected SKU increment 3, got %d", got)
	}
}

func TestSavePublishResultRecordDailyListingCountWithoutDailyLimit(t *testing.T) {
	countClient := &fakeDailyCountClient{}

	handler := NewSavePublishResultHandler()
	input := &PublishResultInput{
		Task: &model.Task{
			TenantID: 1,
			StoreID:  2,
		},
		MemoryManager: &state.MemoryManager{
			DailyCountManager: state.NewDailyCountManager(fakeDailyCountProvider{client: countClient}),
		},
		StoreInfo: &listingruntime.StoreInfo{
			ID:             2,
			DailyLimit:     nil,
			DailyLimitType: "SKU",
		},
		SheinResponse: &sheinproduct.SheinResponse{
			Info: sheinproduct.ResponseInfo{
				SKCList: []sheinproduct.ResponseSKC{
					{
						SKUList: []sheinproduct.ResponseSKU{
							{SKUCode: "sku-1"},
							{SKUCode: "sku-2"},
						},
					},
				},
			},
		},
	}

	handler.recordDailyListingCount(nil, input)

	if countClient.count != 2 {
		t.Fatalf("expected daily listing count 2 without daily limit, got %d", countClient.count)
	}
}
