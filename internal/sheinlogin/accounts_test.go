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
