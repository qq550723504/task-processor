package runner

import (
	"context"
	"fmt"
	"testing"

	"task-processor/internal/app/scheduler"
	"task-processor/internal/listingruntime"

	"github.com/sirupsen/logrus"
)

type stubSchedulerStoreRuntime struct {
	listAutoPricingStoreIDsFunc func(ctx context.Context, platformName string) ([]int64, error)
}

func (s *stubSchedulerStoreRuntime) GetStore(storeID int64) (*listingruntime.StoreInfo, error) {
	return &listingruntime.StoreInfo{ID: storeID}, nil
}

func (s *stubSchedulerStoreRuntime) ListAutoPricingStoreIDs(ctx context.Context, platformName string) ([]int64, error) {
	if s.listAutoPricingStoreIDsFunc == nil {
		return nil, fmt.Errorf("unexpected ListAutoPricingStoreIDs call")
	}
	return s.listAutoPricingStoreIDsFunc(ctx, platformName)
}

func TestResolveStoreIDsForTaskUsesConfiguredWhitelist(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	storeRuntime := &stubSchedulerStoreRuntime{}

	storeIDs := resolveStoreIDsForTask("SHEIN", scheduler.TaskTypePricing, []int64{3, 1, 3, 2}, storeRuntime, logger)
	expected := []int64{1, 2, 3}
	if len(storeIDs) != len(expected) {
		t.Fatalf("expected %d store IDs, got %d", len(expected), len(storeIDs))
	}
	for i := range expected {
		if storeIDs[i] != expected[i] {
			t.Fatalf("expected storeIDs[%d]=%d, got %d", i, expected[i], storeIDs[i])
		}
	}
}

func TestResolveStoreIDsForTaskDiscoversAutoPricingStores(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	calls := 0
	storeRuntime := &stubSchedulerStoreRuntime{
		listAutoPricingStoreIDsFunc: func(_ context.Context, platformName string) ([]int64, error) {
			calls++
			if platformName != "SHEIN" {
				t.Fatalf("expected platform SHEIN, got %s", platformName)
			}
			return []int64{10, 8, 10}, nil
		},
	}

	storeIDs := resolveStoreIDsForTask("SHEIN", scheduler.TaskTypePricing, nil, storeRuntime, logger)
	expected := []int64{8, 10}
	if len(storeIDs) != len(expected) {
		t.Fatalf("expected %d store IDs, got %d", len(expected), len(storeIDs))
	}
	for i := range expected {
		if storeIDs[i] != expected[i] {
			t.Fatalf("expected storeIDs[%d]=%d, got %d", i, expected[i], storeIDs[i])
		}
	}
	if calls != 1 {
		t.Fatalf("expected 1 PageStores call, got %d", calls)
	}
}

func TestResolveStoreIDsForTaskSkipsDynamicDiscoveryForNonPricingTasks(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	storeRuntime := &stubSchedulerStoreRuntime{
		listAutoPricingStoreIDsFunc: func(_ context.Context, _ string) ([]int64, error) {
			t.Fatalf("ListAutoPricingStoreIDs should not be called for non-pricing tasks")
			return nil, nil
		},
	}

	storeIDs := resolveStoreIDsForTask("TEMU", scheduler.TaskTypeInventory, nil, storeRuntime, logger)
	if len(storeIDs) != 0 {
		t.Fatalf("expected no store IDs, got %v", storeIDs)
	}
}
