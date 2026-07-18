package listingkit

import (
	"context"
	"testing"

	sheinproduct "task-processor/internal/shein/api/product"
)

type rejectingSheinSyncService struct {
	syncCalls       int
	sourceSyncCalls int
	productAPICalls int
}

type sourceSDSCostGroupSheinSyncService struct {
	*rejectingSheinSyncService
	listQuery *SheinSourceSDSCostGroupQuery
}

func (s *sourceSDSCostGroupSheinSyncService) ListSDSCostGroups(context.Context, *SheinSDSCostGroupQuery) ([]SheinSDSCostGroupRecord, int64, error) {
	return nil, 0, nil
}

func (s *sourceSDSCostGroupSheinSyncService) ListSourceSDSCostGroups(_ context.Context, query *SheinSourceSDSCostGroupQuery) ([]SheinSourceSDSCostGroupRecord, int64, error) {
	s.listQuery = query
	return []SheinSourceSDSCostGroupRecord{{GroupKey: "source:SDS-869"}}, 1, nil
}

func (s *sourceSDSCostGroupSheinSyncService) UpdateSDSCostGroupManualCost(context.Context, int64, int64, string, string, *float64) (*SheinSDSCostGroupRecord, error) {
	return nil, nil
}

func (s *rejectingSheinSyncService) SyncSheinOnShelfProducts(context.Context, int64, int64, SheinSyncTriggerMode) (*SheinSyncJobRecord, error) {
	s.syncCalls++
	return nil, nil
}

func (s *rejectingSheinSyncService) SyncSheinSourceSDSProduct(context.Context, int64, int64, string) (int, error) {
	s.sourceSyncCalls++
	return 0, nil
}

func (*rejectingSheinSyncService) ListSyncedProducts(context.Context, *SheinSyncedProductQuery) ([]SheinSyncedProductRecord, int64, error) {
	return nil, 0, nil
}

func (*rejectingSheinSyncService) UpdateManualCostPrice(context.Context, int64, *float64) error {
	return nil
}

func (s *rejectingSheinSyncService) ResolveProductAPI(context.Context, int64) (sheinproduct.ProductAPI, error) {
	s.productAPICalls++
	return nil, nil
}

func TestStoreValidatedSheinSyncServiceRejectsBeforeDelegating(t *testing.T) {
	t.Parallel()

	delegate := &rejectingSheinSyncService{}
	service := NewStoreValidatedSheinSyncService(
		delegate,
		rejectingStoreAccessValidator{err: NewStoreAccessError(StoreAccessDisabled, "store is disabled")},
	)

	if _, err := service.SyncSheinOnShelfProducts(context.Background(), 227, 869, SheinSyncTriggerModeManual); StoreAccessErrorCode(err) != StoreAccessDisabled {
		t.Fatalf("sync error = %v, want disabled store error", err)
	}
	if _, err := service.SyncSheinSourceSDSProduct(context.Background(), 227, 869, "SDS-869"); StoreAccessErrorCode(err) != StoreAccessDisabled {
		t.Fatalf("source sync error = %v, want disabled store error", err)
	}
	if _, err := service.ResolveProductAPI(context.Background(), 869); StoreAccessErrorCode(err) != StoreAccessUnavailable {
		t.Fatalf("product api error = %v, want unavailable store error without a tenant context", err)
	}
	if delegate.syncCalls != 0 || delegate.sourceSyncCalls != 0 || delegate.productAPICalls != 0 {
		t.Fatalf("delegate calls = sync:%d source:%d product_api:%d, want all 0", delegate.syncCalls, delegate.sourceSyncCalls, delegate.productAPICalls)
	}
}

func TestStoreValidatedSheinSyncServiceRetainsSourceSDSCostGroupCapability(t *testing.T) {
	t.Parallel()

	delegate := &sourceSDSCostGroupSheinSyncService{rejectingSheinSyncService: &rejectingSheinSyncService{}}
	service := NewStoreValidatedSheinSyncService(delegate, rejectingStoreAccessValidator{})
	costGroups, ok := service.(interface {
		ListSDSCostGroups(context.Context, *SheinSDSCostGroupQuery) ([]SheinSDSCostGroupRecord, int64, error)
		ListSourceSDSCostGroups(context.Context, *SheinSourceSDSCostGroupQuery) ([]SheinSourceSDSCostGroupRecord, int64, error)
		UpdateSDSCostGroupManualCost(context.Context, int64, int64, string, string, *float64) (*SheinSDSCostGroupRecord, error)
	})
	if !ok {
		t.Fatal("store-validated SHEIN sync service does not expose SDS cost group operations")
	}

	query := &SheinSourceSDSCostGroupQuery{TenantID: 227, StoreID: 869, Page: 1, PageSize: 100}
	items, total, err := costGroups.ListSourceSDSCostGroups(context.Background(), query)
	if err != nil {
		t.Fatalf("list source SDS cost groups: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].GroupKey != "source:SDS-869" {
		t.Fatalf("source SDS cost groups = (%+v, %d), want one source:SDS-869 row", items, total)
	}
	if delegate.listQuery != query {
		t.Fatal("source SDS cost-group query was not delegated")
	}
}
