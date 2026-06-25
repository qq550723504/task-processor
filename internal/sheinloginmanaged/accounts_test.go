package sheinloginmanaged

import (
	"context"
	"testing"
	"time"

	"task-processor/internal/ports/managementapi"
	"task-processor/internal/sheinlogin"
)

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
	account, ok := MapStoreToAccountForTest(store)
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
	if _, ok := MapStoreToAccountForTest(store); ok {
		t.Fatal("expected non-shein store to be filtered out")
	}
}

func TestManagementAccountProviderCachesByTenant(t *testing.T) {
	accountProvider := NewAccountProviderWithStoreClientFactory(func(tenantID int64) managementapi.StoreAPI {
		switch tenantID {
		case 7:
			return stubStoreAPI{page: &managementapi.PageResult[*managementapi.StoreRespDTO]{
				List:     []*managementapi.StoreRespDTO{{ID: 101, TenantID: 7, Platform: "SHEIN", Username: "u1", Password: "p1", StoreID: "A"}},
				Total:    1,
				PageNo:   1,
				PageSize: 100,
			}}
		case 8:
			return stubStoreAPI{page: &managementapi.PageResult[*managementapi.StoreRespDTO]{
				List:     []*managementapi.StoreRespDTO{{ID: 102, TenantID: 8, Platform: "SHEIN", Username: "u2", Password: "p2", StoreID: "B"}},
				Total:    1,
				PageNo:   1,
				PageSize: 100,
			}}
		default:
			t.Fatalf("unexpected tenant id: %d", tenantID)
			return nil
		}
	})
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
		items: []sheinlogin.Account{{StoreID: 999, TenantID: 7, Username: "cached", Platform: "SHEIN"}},
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

type stubStoreAPI struct {
	page *managementapi.PageResult[*managementapi.StoreRespDTO]
}

func (s stubStoreAPI) GetStore(int64) (*managementapi.StoreRespDTO, error) { return nil, nil }

func (s stubStoreAPI) PageStores(req *managementapi.StorePageReqDTO) (*managementapi.PageResult[*managementapi.StoreRespDTO], error) {
	if req == nil || req.Platform != "SHEIN" {
		return nil, nil
	}
	return s.page, nil
}

func (s stubStoreAPI) GetStoreCookie(int64) (string, error) { return "", nil }

func (s stubStoreAPI) UpdateStoreId(*managementapi.StoreIdUpdateReqDTO) (bool, error) {
	return false, nil
}

func (s stubStoreAPI) UpdateStoreStatus(*managementapi.StoreStatusUpdateReqDTO) (bool, error) {
	return false, nil
}

func (s stubStoreAPI) DeleteStoreCookie(int64) (bool, error) { return false, nil }

func (s stubStoreAPI) SetStorePauseStatus(int64, bool, string) (bool, error) {
	return false, nil
}

func (s stubStoreAPI) GetStorePauseStatus(int64) (bool, error) { return false, nil }

func (s stubStoreAPI) GetStorePauseStatusDetail(int64) (*managementapi.StorePauseStatusRespDTO, error) {
	return nil, nil
}
