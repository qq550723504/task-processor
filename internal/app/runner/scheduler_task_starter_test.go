package runner

import (
	"fmt"
	"testing"

	"task-processor/internal/app/scheduler"
	managementapi "task-processor/internal/infra/clients/management/api"

	"github.com/sirupsen/logrus"
)

type stubSchedulerStoreClient struct {
	pageStoresFunc func(req *managementapi.StorePageReqDTO) (*managementapi.PageResult[*managementapi.StoreRespDTO], error)
}

func (s *stubSchedulerStoreClient) GetStore(storeID int64) (*managementapi.StoreRespDTO, error) {
	return &managementapi.StoreRespDTO{ID: storeID}, nil
}

func (s *stubSchedulerStoreClient) PageStores(req *managementapi.StorePageReqDTO) (*managementapi.PageResult[*managementapi.StoreRespDTO], error) {
	if s.pageStoresFunc == nil {
		return nil, fmt.Errorf("unexpected PageStores call")
	}
	return s.pageStoresFunc(req)
}

func TestResolveStoreIDsForTaskUsesConfiguredWhitelist(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	storeClient := &stubSchedulerStoreClient{}

	storeIDs := resolveStoreIDsForTask("SHEIN", scheduler.TaskTypePricing, []int64{3, 1, 3, 2}, storeClient, logger)
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
	storeClient := &stubSchedulerStoreClient{
		pageStoresFunc: func(req *managementapi.StorePageReqDTO) (*managementapi.PageResult[*managementapi.StoreRespDTO], error) {
			calls++
			if req.Platform != "" {
				t.Fatalf("expected empty platform filter, got %s", req.Platform)
			}
			if req.EnableAutoPrice == nil || !*req.EnableAutoPrice {
				t.Fatalf("expected enableAutoPrice=true")
			}

			switch req.PageNo {
			case 1:
				return &managementapi.PageResult[*managementapi.StoreRespDTO]{
					List: []*managementapi.StoreRespDTO{
						{ID: 10, Platform: "SHEIN"},
						{ID: 8, Platform: "shein"},
						{ID: 10, Platform: "SHEIN"},
						{ID: 99, Platform: "TEMU"},
					},
					PageNo:   1,
					PageSize: req.PageSize,
					Total:    4,
				}, nil
			case 2:
				return &managementapi.PageResult[*managementapi.StoreRespDTO]{
					List: []*managementapi.StoreRespDTO{},
					PageNo:   2,
					PageSize: req.PageSize,
				}, nil
			default:
				t.Fatalf("unexpected page number %d", req.PageNo)
				return nil, nil
			}
		},
	}

	storeIDs := resolveStoreIDsForTask("SHEIN", scheduler.TaskTypePricing, nil, storeClient, logger)
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
	storeClient := &stubSchedulerStoreClient{
		pageStoresFunc: func(req *managementapi.StorePageReqDTO) (*managementapi.PageResult[*managementapi.StoreRespDTO], error) {
			t.Fatalf("PageStores should not be called for non-pricing tasks")
			return nil, nil
		},
	}

	storeIDs := resolveStoreIDsForTask("TEMU", scheduler.TaskTypeInventory, nil, storeClient, logger)
	if len(storeIDs) != 0 {
		t.Fatalf("expected no store IDs, got %v", storeIDs)
	}
}
