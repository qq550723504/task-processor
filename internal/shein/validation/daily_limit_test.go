package validation

import (
	"context"
	"testing"

	managementapi "task-processor/internal/listingadmin"
	"task-processor/internal/listingruntime"
	"task-processor/internal/model"
	shein "task-processor/internal/shein"
	sheinctx "task-processor/internal/shein/context"
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

func TestCheckDailyLimitHandlerReturnsPausedHandledErrorWhenLimitReached(t *testing.T) {
	limit := 5
	countClient := &fakeDailyCountClient{reservation: &managementapi.TryConsumeDailyQuotaRespDTO{
		Allowed:      false,
		NewCount:     5,
		Remaining:    0,
		ReachedLimit: true,
	}}
	mem := state.NewMemoryManager(context.Background(), fakeDailyCountProvider{client: countClient})

	ctx := &sheinctx.TaskContext{
		RuntimeState: sheinctx.RuntimeState{
			Task:          &model.Task{ID: 1, TenantID: 1, StoreID: 2},
			MemoryManager: mem,
			StoreInfo: &listingruntime.StoreInfo{
				ID:             2,
				DailyLimit:     &limit,
				DailyLimitType: "SPU",
			},
		},
	}

	err := NewCheckDailyLimitHandler().Handle(ctx)
	if err == nil {
		t.Fatal("expected handled error")
	}

	handledErr, ok := shein.AsTaskHandledError(err)
	if !ok {
		t.Fatalf("expected TaskHandledError, got %T", err)
	}
	if handledErr.TargetStatus() != model.TaskStatusPaused {
		t.Fatalf("target status = %v, want %v", handledErr.TargetStatus(), model.TaskStatusPaused)
	}
	wantMsg := "[DAILY_LIMIT_REACHED] 店铺已达到每日上架限额(5/5)，已暂停上架到当日结束"
	if handledErr.ErrorMessage() != wantMsg {
		t.Fatalf("error message = %q, want %q", handledErr.ErrorMessage(), wantMsg)
	}
}
