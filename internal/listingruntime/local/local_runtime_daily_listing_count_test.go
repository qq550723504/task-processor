package local

import (
	"context"
	"sync"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"

	"task-processor/internal/listingadmin"
)

func TestLocalRuntimeDailyListingCountClientUsesResourcesWithoutCompatibilityProvider(t *testing.T) {
	runtime, _ := newLocalRuntimeDailyListingCountTestRuntime(t)
	client := runtime.GetDailyListingCountClient()
	if client == nil {
		t.Fatal("GetDailyListingCountClient() returned nil")
	}

	request := &listingadmin.DailyListingCountSetReqDTO{TenantID: 1, StoreID: 2, UserID: 3, Date: "2026-08-30", Count: 4}
	if err := client.SetDailyListingCount(request); err != nil {
		t.Fatalf("SetDailyListingCount() error = %v", err)
	}
	count, err := client.GetDailyListingCount(1, 2, 3, "2026-08-30")
	if err != nil {
		t.Fatalf("GetDailyListingCount() error = %v", err)
	}
	if count == nil || count.Count != 4 || count.TenantID != 1 || count.StoreID != 2 || count.UserID != 3 || count.Date != "2026-08-30" {
		t.Fatalf("GetDailyListingCount() = %#v; want persisted resource count", count)
	}
}

func TestLocalRuntimeDailyListingQuotaClientUsesResourcesWithoutCompatibilityProvider(t *testing.T) {
	runtime, _ := newLocalRuntimeDailyListingCountTestRuntime(t)
	client := runtime.GetDailyListingCountClient()
	if client == nil {
		t.Fatal("GetDailyListingCountClient() returned nil")
	}

	first, err := client.TryConsumeDailyQuota(&listingadmin.TryConsumeDailyQuotaReqDTO{TenantID: 1, StoreID: 2, UserID: 3, Date: "2026-08-30", Increment: 2, Limit: 3})
	if err != nil {
		t.Fatalf("TryConsumeDailyQuota(first) error = %v", err)
	}
	if first == nil || !first.Allowed || first.NewCount != 2 || first.Remaining != 1 || first.ReachedLimit {
		t.Fatalf("TryConsumeDailyQuota(first) = %#v; want allowed count 2 with one remaining", first)
	}

	second, err := client.TryConsumeDailyQuota(&listingadmin.TryConsumeDailyQuotaReqDTO{TenantID: 1, StoreID: 2, UserID: 3, Date: "2026-08-30", Increment: 2, Limit: 3})
	if err != nil {
		t.Fatalf("TryConsumeDailyQuota(second) error = %v", err)
	}
	if second == nil || second.Allowed || second.NewCount != 2 || second.Remaining != 1 || second.ReachedLimit {
		t.Fatalf("TryConsumeDailyQuota(second) = %#v; want rejected quota request", second)
	}

	count, err := client.RollbackDailyQuota(&listingadmin.RollbackDailyQuotaReqDTO{TenantID: 1, StoreID: 2, UserID: 3, Date: "2026-08-30", Decrement: 1})
	if err != nil {
		t.Fatalf("RollbackDailyQuota() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("RollbackDailyQuota() = %d; want 1", count)
	}
}

func TestLocalRuntimeRemainingListingQuotaClientUsesResourcesWithoutCompatibilityProvider(t *testing.T) {
	runtime, server := newLocalRuntimeDailyListingCountTestRuntime(t)
	client := runtime.GetDailyListingCountClient()
	if client == nil {
		t.Fatal("GetDailyListingCountClient() returned nil")
	}

	updated, err := client.SetRemainingListingQuota(1, 2, 7)
	if err != nil {
		t.Fatalf("SetRemainingListingQuota() error = %v", err)
	}
	if !updated {
		t.Fatal("SetRemainingListingQuota() = false; want resource-backed quota update")
	}
	if stored, err := server.Get("listing:remaining:quota:1:2"); err != nil || stored != "7" {
		t.Fatalf("remaining quota value = %q, %v; want 7", stored, err)
	}
}

func TestLocalDataProviderDailyListingCountCompatibility(t *testing.T) {
	_, server := newLocalRuntimeDailyListingCountTestRuntime(t)
	provider := NewLocalDataProviderFromResources(NewRuntimeResources(nil, goredis.NewClient(&goredis.Options{Addr: server.Addr()})))
	t.Cleanup(func() { _ = provider.Close() })

	if err := provider.SetDailyListingCount(&listingadmin.DailyListingCountSetReqDTO{TenantID: 9, StoreID: 8, UserID: 7, Date: "2026-08-30", Count: 6}); err != nil {
		t.Fatalf("SetDailyListingCount() error = %v", err)
	}
	count, err := provider.GetDailyListingCount(9, 8, 7, "2026-08-30")
	if err != nil {
		t.Fatalf("GetDailyListingCount() error = %v", err)
	}
	if count == nil || count.Count != 6 {
		t.Fatalf("GetDailyListingCount() = %#v; want compatibility count 6", count)
	}
}

func TestLocalDailyListingQuotaConsumptionIsAtomic(t *testing.T) {
	server := miniredis.RunT(t)
	redisClient := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	barrier := newDailyListingCountGetBarrierHook()
	redisClient.AddHook(barrier)
	resources := NewRuntimeResources(nil, redisClient)
	t.Cleanup(func() { _ = resources.Close() })
	client := (&LocalRuntime{resources: resources}).GetDailyListingCountClient()

	results := make(chan *listingadmin.TryConsumeDailyQuotaRespDTO, 2)
	errs := make(chan error, 2)
	var workers sync.WaitGroup
	for range 2 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			result, err := client.TryConsumeDailyQuota(&listingadmin.TryConsumeDailyQuotaReqDTO{TenantID: 1, StoreID: 2, UserID: 3, Date: "2026-08-30", Increment: 1, Limit: 1})
			results <- result
			errs <- err
		}()
	}
	done := make(chan struct{})
	go func() {
		workers.Wait()
		close(done)
	}()

	select {
	case <-barrier.ready:
		close(barrier.release)
	case <-done:
		close(barrier.release)
	case <-time.After(time.Second):
		t.Fatal("quota requests did not complete")
	}
	<-done
	close(results)
	close(errs)

	allowed := 0
	for result := range results {
		if result != nil && result.Allowed {
			allowed++
		}
	}
	for err := range errs {
		if err != nil {
			t.Fatalf("TryConsumeDailyQuota() error = %v", err)
		}
	}
	if allowed != 1 {
		t.Fatalf("allowed quota consumptions = %d; want 1", allowed)
	}
	count, err := client.GetDailyListingCount(1, 2, 3, "2026-08-30")
	if err != nil {
		t.Fatalf("GetDailyListingCount() error = %v", err)
	}
	if count == nil || count.Count != 1 {
		t.Fatalf("GetDailyListingCount() = %#v; want count 1", count)
	}
	if ttl := server.TTL("listing:daily:count:1:2:2026-08-30"); ttl <= 0 {
		t.Fatalf("daily listing count TTL = %v; want positive expiration", ttl)
	}
}

func newLocalRuntimeDailyListingCountTestRuntime(t *testing.T) (*LocalRuntime, *miniredis.Miniredis) {
	t.Helper()
	server := miniredis.RunT(t)
	redisClient := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	resources := NewRuntimeResources(nil, redisClient)
	t.Cleanup(func() { _ = resources.Close() })
	return &LocalRuntime{resources: resources}, server
}

type dailyListingCountGetBarrierHook struct {
	mu      sync.Mutex
	gets    int
	ready   chan struct{}
	release chan struct{}
}

func newDailyListingCountGetBarrierHook() *dailyListingCountGetBarrierHook {
	return &dailyListingCountGetBarrierHook{ready: make(chan struct{}), release: make(chan struct{})}
}

func (h *dailyListingCountGetBarrierHook) DialHook(next goredis.DialHook) goredis.DialHook {
	return next
}

func (h *dailyListingCountGetBarrierHook) ProcessHook(next goredis.ProcessHook) goredis.ProcessHook {
	return func(ctx context.Context, cmd goredis.Cmder) error {
		err := next(ctx, cmd)
		if err != nil || cmd.Name() != "get" {
			return err
		}
		h.mu.Lock()
		h.gets++
		if h.gets == 2 {
			close(h.ready)
		}
		h.mu.Unlock()
		<-h.release
		return err
	}
}

func (h *dailyListingCountGetBarrierHook) ProcessPipelineHook(next goredis.ProcessPipelineHook) goredis.ProcessPipelineHook {
	return next
}

var _ goredis.Hook = (*dailyListingCountGetBarrierHook)(nil)
