package pipeline

import (
	"context"
	"strings"
	"testing"

	amazonmodel "task-processor/internal/amazon/model"
	managementapi "task-processor/internal/listingadmin"
	"task-processor/internal/state"
)

type fakeDailyCountProvider struct {
	client *fakeDailyCountClient
}

func (p fakeDailyCountProvider) GetDailyListingCountClient() managementapi.DailyListingCountAPI {
	return p.client
}

type fakeDailyCountClient struct {
	reservation *managementapi.TryConsumeDailyQuotaRespDTO
}

func (c *fakeDailyCountClient) GetDailyListingCount(tenantID, storeID, userID int64, date string) (*managementapi.DailyListingCountRespDTO, error) {
	return &managementapi.DailyListingCountRespDTO{TenantID: tenantID, StoreID: storeID, UserID: userID, Date: date}, nil
}

func (c *fakeDailyCountClient) SetDailyListingCount(*managementapi.DailyListingCountSetReqDTO) error {
	return nil
}

func (c *fakeDailyCountClient) TryConsumeDailyQuota(*managementapi.TryConsumeDailyQuotaReqDTO) (*managementapi.TryConsumeDailyQuotaRespDTO, error) {
	return c.reservation, nil
}

func (c *fakeDailyCountClient) RollbackDailyQuota(*managementapi.RollbackDailyQuotaReqDTO) (int64, error) {
	return 0, nil
}

func (c *fakeDailyCountClient) SetRemainingListingQuota(int64, int64, int) (bool, error) {
	return true, nil
}

func TestDailyLimitHandlerReservesQuotaWhenUnderLimit(t *testing.T) {
	limit := 5
	countClient := &fakeDailyCountClient{reservation: &managementapi.TryConsumeDailyQuotaRespDTO{
		Allowed:      true,
		NewCount:     3,
		Remaining:    2,
		ReachedLimit: false,
	}}

	taskContext := &amazonmodel.TaskContext{
		Data: map[string]any{
			"tenantId": int64(1),
			"storeId":  int64(2),
		},
		StoreInfo: &managementapi.StoreRespDTO{
			ID:             2,
			DailyLimit:     &limit,
			DailyLimitType: "SPU",
		},
	}

	handler := NewDailyLimitHandler(&amazonmodel.Services{
		MemoryManager: state.NewMemoryManager(context.Background(), fakeDailyCountProvider{client: countClient}),
	})
	if err := handler.Handle(context.Background(), taskContext); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if !taskContext.DailyQuotaReserved {
		t.Fatal("expected daily quota reservation to be recorded")
	}
	if taskContext.DailyQuotaIncrement != 1 {
		t.Fatalf("expected increment 1, got %d", taskContext.DailyQuotaIncrement)
	}
}

func TestDailyLimitHandlerReturnsNonRetryableWhenLimitReached(t *testing.T) {
	limit := 5
	countClient := &fakeDailyCountClient{reservation: &managementapi.TryConsumeDailyQuotaRespDTO{
		Allowed:      false,
		NewCount:     5,
		Remaining:    0,
		ReachedLimit: true,
	}}

	taskContext := &amazonmodel.TaskContext{
		Data: map[string]any{
			"tenantId": int64(1),
			"storeId":  int64(2),
		},
		StoreInfo: &managementapi.StoreRespDTO{
			ID:             2,
			DailyLimit:     &limit,
			DailyLimitType: "SPU",
		},
	}

	handler := NewDailyLimitHandler(&amazonmodel.Services{
		MemoryManager: state.NewMemoryManager(context.Background(), fakeDailyCountProvider{client: countClient}),
	})
	err := handler.Handle(context.Background(), taskContext)
	if err == nil {
		t.Fatal("expected nonretryable error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "nonretryable:") {
		t.Fatalf("expected nonretryable error, got %v", err)
	}
}
