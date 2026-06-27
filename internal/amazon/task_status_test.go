package amazon

import (
	"testing"

	amazonModel "task-processor/internal/amazon/model"
	managementapi "task-processor/internal/listingadmin"
	"task-processor/internal/model"
	"task-processor/internal/state"

	"github.com/sirupsen/logrus"
)

type fakeDailyCountProvider struct {
	client *fakeDailyCountClient
}

func (p fakeDailyCountProvider) GetDailyListingCountClient() managementapi.DailyListingCountAPI {
	return p.client
}

type fakeDailyCountClient struct {
	count int64
}

func (c *fakeDailyCountClient) GetDailyListingCount(tenantID, storeID, userID int64, date string) (*managementapi.DailyListingCountRespDTO, error) {
	return &managementapi.DailyListingCountRespDTO{TenantID: tenantID, StoreID: storeID, UserID: userID, Date: date, Count: c.count}, nil
}

func (c *fakeDailyCountClient) SetDailyListingCount(req *managementapi.DailyListingCountSetReqDTO) error {
	c.count = req.Count
	return nil
}

func (c *fakeDailyCountClient) TryConsumeDailyQuota(req *managementapi.TryConsumeDailyQuotaReqDTO) (*managementapi.TryConsumeDailyQuotaRespDTO, error) {
	next := c.count + req.Increment
	allowed := next <= req.Limit
	if allowed {
		c.count = next
	}
	return &managementapi.TryConsumeDailyQuotaRespDTO{
		Allowed:      allowed,
		NewCount:     c.count,
		Remaining:    req.Limit - c.count,
		ReachedLimit: c.count >= req.Limit,
	}, nil
}

func (c *fakeDailyCountClient) RollbackDailyQuota(req *managementapi.RollbackDailyQuotaReqDTO) (int64, error) {
	c.count -= req.Decrement
	if c.count < 0 {
		c.count = 0
	}
	return c.count, nil
}

func (c *fakeDailyCountClient) SetRemainingListingQuota(int64, int64, int) (bool, error) {
	return true, nil
}

func TestParseTaskStatusMetadata(t *testing.T) {
	reasonCode, stage := parseTaskStatusMetadata("NONRETRYABLE: [stage:check_daily_limit] [DAILY_LIMIT_REACHED] limit reached")
	if reasonCode != amazonTaskReasonDailyLimitReached {
		t.Fatalf("reasonCode = %q, want %q", reasonCode, amazonTaskReasonDailyLimitReached)
	}
	if stage != "check_daily_limit" {
		t.Fatalf("stage = %q, want check_daily_limit", stage)
	}
}

func TestShouldPauseAmazonTask(t *testing.T) {
	if !shouldPauseAmazonTask(amazonTaskReasonAuthExpired) {
		t.Fatal("auth expired should pause task")
	}
	if !shouldPauseAmazonTask(amazonTaskReasonDailyLimitReached) {
		t.Fatal("daily limit should pause task")
	}
	if shouldPauseAmazonTask("VALIDATION_FAILED") {
		t.Fatal("validation failure should not pause task")
	}
}

func TestIsAmazonRetryableTaskError(t *testing.T) {
	if isAmazonRetryableTaskError(assertError("NONRETRYABLE: [stage:validate_input] [VALIDATION_FAILED] bad input")) {
		t.Fatal("nonretryable error should not be retried")
	}
	if !isAmazonRetryableTaskError(assertError("[stage:create_listing] [RATE_LIMIT] slow down")) {
		t.Fatal("rate limit error should remain retryable")
	}
}

func TestRecordAmazonDailyListingCountWithoutDailyLimit(t *testing.T) {
	countClient := &fakeDailyCountClient{}

	recordAmazonDailyListingCount(
		&model.Task{
			TenantID: 1,
			StoreID:  2,
		},
		&amazonModel.TaskContext{
			StoreInfo: &managementapi.StoreRespDTO{
				ID:             2,
				DailyLimitType: "SPU",
			},
		},
		&state.MemoryManager{
			DailyCountManager: state.NewDailyCountManager(fakeDailyCountProvider{client: countClient}),
		},
		logrus.New(),
	)

	if countClient.count != 1 {
		t.Fatalf("expected daily listing count 1 without daily limit, got %d", countClient.count)
	}
}

type assertError string

func (e assertError) Error() string { return string(e) }
