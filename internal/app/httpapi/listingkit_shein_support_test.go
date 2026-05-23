package httpapi

import (
	"context"
	"testing"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/listingadmin"
	"task-processor/internal/tenantbridge"
)

type staticLegacyTenantResolver struct {
	value int64
}

func (r staticLegacyTenantResolver) ResolveLegacyTenantID(_ context.Context, tenantID string) (int64, bool, error) {
	if tenantID == "373211199677923496" && r.value > 0 {
		return r.value, true, nil
	}
	return 0, false, nil
}

type recordingStoreRepository struct {
	lastTenantID int64
	lastStoreID  int64
	store        *listingadmin.Store
}

func (r *recordingStoreRepository) ListStores(context.Context, listingadmin.StoreQuery) (*listingadmin.StorePage, error) {
	return nil, nil
}

func (r *recordingStoreRepository) GetStore(_ context.Context, tenantID, id int64) (*listingadmin.Store, error) {
	r.lastTenantID = tenantID
	r.lastStoreID = id
	if r.store == nil {
		return nil, listingadmin.ErrStoreNotFound
	}
	return r.store, nil
}

func (r *recordingStoreRepository) CreateStore(context.Context, *listingadmin.Store) (*listingadmin.Store, error) {
	panic("unexpected call")
}

func (r *recordingStoreRepository) UpdateStore(context.Context, *listingadmin.Store) (*listingadmin.Store, error) {
	panic("unexpected call")
}

func (r *recordingStoreRepository) UpdateStoreStatus(context.Context, int64, int64, int16, string) (*listingadmin.Store, error) {
	panic("unexpected call")
}

func (r *recordingStoreRepository) DeleteStore(context.Context, int64, int64) error {
	panic("unexpected call")
}

func (r *recordingStoreRepository) ListDeletedStores(context.Context, int64) ([]listingadmin.Store, error) {
	panic("unexpected call")
}

func (r *recordingStoreRepository) RestoreStore(context.Context, int64, int64) (*listingadmin.Store, error) {
	panic("unexpected call")
}

func (r *recordingStoreRepository) PermanentlyDeleteStore(context.Context, int64, int64) error {
	panic("unexpected call")
}

func (r *recordingStoreRepository) ExtendStoreValidity(context.Context, int64, int64, int) (*listingadmin.Store, error) {
	panic("unexpected call")
}

func TestListingKitSheinRuntimeFactoryResolveStoreConfigUsesLegacyTenantID(t *testing.T) {
	t.Parallel()

	restore := tenantbridge.ConfigureLegacyTenantResolver(staticLegacyTenantResolver{value: 227})
	defer restore()

	repo := &recordingStoreRepository{
		store: &listingadmin.Store{
			ID:       870,
			TenantID: 227,
			StoreID:  "870",
			Name:     "SHEIN Store 870",
			Platform: "shein",
			Region:   "US",
		},
	}
	factory := listingKitSheinRuntimeFactory{repo: repo}
	ctx := openaiclient.WithIdentity(context.Background(), openaiclient.Identity{
		TenantID: "373211199677923496",
		UserID:   "user-1",
	})

	store := factory.resolveStoreConfig(ctx, 870)
	if repo.lastTenantID != 227 {
		t.Fatalf("tenant id = %d, want 227", repo.lastTenantID)
	}
	if repo.lastStoreID != 870 {
		t.Fatalf("store id = %d, want 870", repo.lastStoreID)
	}
	if store == nil {
		t.Fatal("expected store config")
	}
	if store.TenantID != 227 {
		t.Fatalf("store tenant id = %d, want 227", store.TenantID)
	}
}
