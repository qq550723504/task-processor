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
