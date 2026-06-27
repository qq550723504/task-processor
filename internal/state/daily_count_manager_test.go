package state

import (
	"sync"
	"testing"
	"time"

	managementapi "task-processor/internal/listingadmin"
)

type fakeDailyCountProvider struct {
	client *fakeDailyCountClient
}

func (p fakeDailyCountProvider) GetDailyListingCountClient() managementapi.DailyListingCountAPI {
	return p.client
}

type fakeDailyCountClient struct {
	mu    sync.Mutex
	count int64
}

func (c *fakeDailyCountClient) GetDailyListingCount(tenantID, storeID, userID int64, date string) (*managementapi.DailyListingCountRespDTO, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return &managementapi.DailyListingCountRespDTO{TenantID: tenantID, StoreID: storeID, UserID: userID, Date: date, Count: c.count}, nil
}

func (c *fakeDailyCountClient) SetDailyListingCount(req *managementapi.DailyListingCountSetReqDTO) error {
	time.Sleep(20 * time.Millisecond)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.count = req.Count
	return nil
}

func (c *fakeDailyCountClient) TryConsumeDailyQuota(req *managementapi.TryConsumeDailyQuotaReqDTO) (*managementapi.TryConsumeDailyQuotaRespDTO, error) {
	return &managementapi.TryConsumeDailyQuotaRespDTO{Allowed: true, NewCount: c.count + req.Increment}, nil
}

func (c *fakeDailyCountClient) RollbackDailyQuota(req *managementapi.RollbackDailyQuotaReqDTO) (int64, error) {
	return c.count - req.Decrement, nil
}

func (c *fakeDailyCountClient) SetRemainingListingQuota(int64, int64, int) (bool, error) {
	return true, nil
}

func TestDailyCountManagerIncrementCountSerializesPerStoreDate(t *testing.T) {
	client := &fakeDailyCountClient{}
	manager := NewDailyCountManager(fakeDailyCountProvider{client: client})

	var wg sync.WaitGroup
	results := make(chan int64, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- manager.IncrementCount(1, 2, "2026-03-30", 1)
		}()
	}

	wg.Wait()
	close(results)

	var maxResult int64
	for result := range results {
		if result > maxResult {
			maxResult = result
		}
	}

	client.mu.Lock()
	finalCount := client.count
	client.mu.Unlock()

	if finalCount != 2 {
		t.Fatalf("expected final count 2, got %d", finalCount)
	}
	if maxResult != 2 {
		t.Fatalf("expected one increment call to observe count 2, got %d", maxResult)
	}
}
