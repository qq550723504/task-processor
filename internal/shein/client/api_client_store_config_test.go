package client

import (
	"testing"
)

func TestNewAPIClientWithStoreConfigSeedsCookieManagerTenant(t *testing.T) {
	storeInfo := &StoreConfig{
		ID:       869,
		TenantID: 227,
	}

	apiClient := NewAPIClientWithStoreConfig(869, storeInfo, nil)

	if got := apiClient.GetCookieManager().GetResolvedTenantID(); got != storeInfo.TenantID {
		t.Fatalf("cookie manager tenant ID = %d, want %d", got, storeInfo.TenantID)
	}
}

func TestNewAPIClientWithStoreConfigCanIgnoreStoreProxy(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_SHEIN_IGNORE_STORE_PROXY", "1")
	storeInfo := &StoreConfig{
		ID:       869,
		TenantID: 227,
		Proxy:    "http://10.42.0.1:31069",
	}

	apiClient := NewAPIClientWithStoreConfig(869, storeInfo, nil)

	if got := apiClient.GetProxyURL(); got != "" {
		t.Fatalf("proxy URL = %q, want empty when proxy ignored", got)
	}
}
