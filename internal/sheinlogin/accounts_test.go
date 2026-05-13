package sheinlogin

import (
	"testing"

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
