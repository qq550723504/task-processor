package sheinlogin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"task-processor/internal/infra/clients/management"
	managementapi "task-processor/internal/infra/clients/management/api"
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

func TestMapStoreToAccountFiltersAndMapsSheinStore(t *testing.T) {
	store := &managementapi.StoreRespDTO{
		ID:       12,
		TenantID: 34,
		Platform: "SHEIN",
		Username: "demo-user",
		Password: "secret",
		LoginUrl: "sellerhub.shein.com",
		Proxy:    "http://127.0.0.1:8080",
		Name:     "Demo Shop",
		StoreID:  "SHEIN-12",
	}
	account, ok := mapStoreToAccount(store)
	if !ok {
		t.Fatal("expected store to map to shein account")
	}
	if account.StoreID != 12 || account.TenantID != 34 {
		t.Fatalf("unexpected account identity: %+v", account)
	}
	if account.Username != "demo-user" || account.Proxy == "" || account.ShopName != "Demo Shop" {
		t.Fatalf("unexpected account fields: %+v", account)
	}
	if account.LoginURL != "https://sellerhub.shein.com" {
		t.Fatalf("expected normalized login url, got %q", account.LoginURL)
	}

	store.Platform = "TEMU"
	if _, ok := mapStoreToAccount(store); ok {
		t.Fatal("expected non-shein store to be filtered out")
	}
}

func TestManagementAccountProviderCachesByTenant(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rpc-api/listing/store/page" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var req struct {
			TenantID int64  `json:"tenantId"`
			Platform string `json:"platform"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Platform != "SHEIN" {
			t.Fatalf("unexpected platform: %s", req.Platform)
		}
		var item *managementapi.StoreRespDTO
		switch req.TenantID {
		case 7:
			item = &managementapi.StoreRespDTO{ID: 101, TenantID: 7, Platform: "SHEIN", Username: "u1", Password: "p1", StoreID: "A"}
		case 8:
			item = &managementapi.StoreRespDTO{ID: 102, TenantID: 8, Platform: "SHEIN", Username: "u2", Password: "p2", StoreID: "B"}
		default:
			t.Fatalf("unexpected tenant id: %d", req.TenantID)
		}
		_ = json.NewEncoder(w).Encode(managementapi.CommonResult[managementapi.PageResult[*managementapi.StoreRespDTO]]{
			Code: 0,
			Data: managementapi.PageResult[*managementapi.StoreRespDTO]{
				List:     []*managementapi.StoreRespDTO{item},
				Total:    1,
				PageNo:   1,
				PageSize: 100,
			},
		})
	}))
	defer server.Close()

	clientManager := management.NewClientManager(nil)
	clientManager.SetBaseURL(server.URL)
	clientManager.GetClient()
	clientManager.SetUserToken("token", "7")

	accountProvider := NewManagementAccountProvider(clientManager)
	tenant7, err := accountProvider.ListAccounts(context.Background(), 7)
	if err != nil {
		t.Fatalf("list tenant 7: %v", err)
	}
	if len(tenant7) != 1 || tenant7[0].TenantID != 7 {
		t.Fatalf("unexpected tenant 7 accounts: %+v", tenant7)
	}

	tenant8, err := accountProvider.ListAccounts(context.Background(), 8)
	if err != nil {
		t.Fatalf("list tenant 8: %v", err)
	}
	if len(tenant8) != 1 || tenant8[0].TenantID != 8 {
		t.Fatalf("unexpected tenant 8 accounts: %+v", tenant8)
	}

	accountProvider.mu.Lock()
	accountProvider.cache[7] = tenantAccountCache{
		items: []Account{{StoreID: 999, TenantID: 7, Username: "cached", Platform: "SHEIN"}},
		until: time.Now().Add(5 * time.Second),
	}
	accountProvider.mu.Unlock()

	tenant8Again, err := accountProvider.ListAccounts(context.Background(), 8)
	if err != nil {
		t.Fatalf("list tenant 8 again: %v", err)
	}
	if len(tenant8Again) != 1 || tenant8Again[0].TenantID != 8 || tenant8Again[0].StoreID != 102 {
		t.Fatalf("tenant 8 cache contaminated: %+v", tenant8Again)
	}
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
