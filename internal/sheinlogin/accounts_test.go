package sheinlogin

import (
	"context"
	"testing"

	"task-processor/internal/listingadmin"
)

type stubListingAdminAccountStore struct {
	items []listingadmin.Store
}

func (s *stubListingAdminAccountStore) ListStores(_ context.Context, query listingadmin.StoreQuery) (*listingadmin.StorePage, error) {
	items := make([]listingadmin.Store, 0, len(s.items))
	for _, item := range s.items {
		if query.TenantID > 0 && item.TenantID != query.TenantID {
			continue
		}
		if query.Platform != "" && item.Platform != query.Platform {
			continue
		}
		items = append(items, item)
	}
	return &listingadmin.StorePage{Items: items, Total: int64(len(items)), Page: 1, PageSize: len(items)}, nil
}

func (s *stubListingAdminAccountStore) GetStore(_ context.Context, tenantID, id int64) (*listingadmin.Store, error) {
	for _, item := range s.items {
		if item.TenantID == tenantID && item.ID == id {
			store := item
			return &store, nil
		}
	}
	return nil, listingadmin.ErrStoreNotFound
}

func (s *stubListingAdminAccountStore) CreateStore(context.Context, *listingadmin.Store) (*listingadmin.Store, error) {
	panic("unexpected CreateStore")
}
func (s *stubListingAdminAccountStore) UpdateStore(context.Context, *listingadmin.Store) (*listingadmin.Store, error) {
	panic("unexpected UpdateStore")
}
func (s *stubListingAdminAccountStore) UpdateStoreStatus(context.Context, int64, int64, int16, string) (*listingadmin.Store, error) {
	panic("unexpected UpdateStoreStatus")
}
func (s *stubListingAdminAccountStore) DeleteStore(context.Context, int64, int64) error {
	panic("unexpected DeleteStore")
}
func (s *stubListingAdminAccountStore) ListDeletedStores(context.Context, int64) ([]listingadmin.Store, error) {
	panic("unexpected ListDeletedStores")
}
func (s *stubListingAdminAccountStore) RestoreStore(context.Context, int64, int64) (*listingadmin.Store, error) {
	panic("unexpected RestoreStore")
}
func (s *stubListingAdminAccountStore) PermanentlyDeleteStore(context.Context, int64, int64) error {
	panic("unexpected PermanentlyDeleteStore")
}
func (s *stubListingAdminAccountStore) ExtendStoreValidity(context.Context, int64, int64, int) (*listingadmin.Store, error) {
	panic("unexpected ExtendStoreValidity")
}

func TestListingAdminAccountProviderLoadsSheinAccountsFromRepository(t *testing.T) {
	provider := NewListingAdminAccountProvider(&stubListingAdminAccountStore{
		items: []listingadmin.Store{
			{
				ID:       12,
				TenantID: 7,
				Platform: "shein",
				Username: "demo-user",
				Password: "secret",
				LoginURL: "sellerhub.shein.com",
				Proxy:    "http://127.0.0.1:8080",
				Name:     "Demo Shop",
				StoreID:  "SHEIN-12",
			},
			{
				ID:       99,
				TenantID: 7,
				Platform: "temu",
				Username: "ignored",
				Password: "ignored",
			},
		},
	})

	accounts, err := provider.ListAccounts(context.Background(), 7)
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("account count = %d, want 1", len(accounts))
	}
	if accounts[0].StoreID != 12 || accounts[0].TenantID != 7 {
		t.Fatalf("unexpected account identity: %+v", accounts[0])
	}
	if accounts[0].LoginURL != "https://sellerhub.shein.com" {
		t.Fatalf("expected normalized login url, got %q", accounts[0].LoginURL)
	}
}
