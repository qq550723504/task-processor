package managedclient

import (
	"testing"

	"task-processor/internal/listingruntime"
)

func TestNewAPIClientWithStoreInfoAppliesProxyAndLoginURL(t *testing.T) {
	storeInfo := &listingruntime.StoreInfo{
		ID:       869,
		TenantID: 227,
		LoginURL: "sso.geiwohuo.com",
		Proxy:    "http://10.42.0.1:31069",
	}

	apiClient := NewAPIClientWithStoreInfo(869, nil, nil, storeInfo)

	if got := apiClient.GetProxyURL(); got != storeInfo.Proxy {
		t.Fatalf("proxy URL = %q, want %q", got, storeInfo.Proxy)
	}
	if got := apiClient.GetBaseURL(); got != "https://sso.geiwohuo.com" {
		t.Fatalf("base URL = %q, want %q", got, "https://sso.geiwohuo.com")
	}
	if got := apiClient.GetTenantID(); got != storeInfo.TenantID {
		t.Fatalf("tenant ID = %d, want %d", got, storeInfo.TenantID)
	}
	if got := apiClient.GetCookieManager().GetResolvedTenantID(); got != storeInfo.TenantID {
		t.Fatalf("cookie manager tenant ID = %d, want %d", got, storeInfo.TenantID)
	}
}

func TestNewAPIClientWithStoreInfoCanIgnoreStoreProxy(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_SHEIN_IGNORE_STORE_PROXY", "1")
	storeInfo := &listingruntime.StoreInfo{
		ID:       869,
		TenantID: 227,
		Proxy:    "http://10.42.0.1:31069",
	}

	apiClient := NewAPIClientWithStoreInfo(869, nil, nil, storeInfo)

	if got := apiClient.GetProxyURL(); got != "" {
		t.Fatalf("proxy URL = %q, want empty when proxy ignored", got)
	}
}
