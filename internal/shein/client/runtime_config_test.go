package client

import (
	"context"
	"testing"
)

type stubCookieProvider struct {
	result *CookieLookupResult
	err    error
}

func (p stubCookieProvider) GetCookie(_ context.Context, _ int64) (*CookieLookupResult, error) {
	return p.result, p.err
}

func TestNewAPIClientWithStoreConfigAppliesRuntimeStoreFields(t *testing.T) {
	storeInfo := &StoreConfig{
		ID:       869,
		TenantID: 227,
		LoginURL: "sso.geiwohuo.com",
		Proxy:    "http://10.42.0.1:31069",
	}

	apiClient := NewAPIClientWithStoreConfig(869, storeInfo, nil)

	if got := apiClient.GetProxyURL(); got != storeInfo.Proxy {
		t.Fatalf("proxy URL = %q, want %q", got, storeInfo.Proxy)
	}
	if got := apiClient.GetBaseURL(); got != "https://sso.geiwohuo.com" {
		t.Fatalf("base URL = %q, want %q", got, "https://sso.geiwohuo.com")
	}
	if got := apiClient.GetTenantID(); got != storeInfo.TenantID {
		t.Fatalf("tenant ID = %d, want %d", got, storeInfo.TenantID)
	}
}

func TestNewAPIClientWithStoreConfigLoadsCookiesFromProvider(t *testing.T) {
	storeInfo := &StoreConfig{
		ID:       869,
		TenantID: 227,
	}
	provider := stubCookieProvider{
		result: &CookieLookupResult{
			TenantID:   227,
			CookieJSON: `[{"name":"sid","value":"abc","domain":".shein.com","path":"/"}]`,
		},
	}

	apiClient := NewAPIClientWithStoreConfig(869, storeInfo, provider)

	if !apiClient.HasCookies() {
		t.Fatal("expected cookies to be loaded from provider")
	}
	if got := apiClient.GetCookieManager().GetResolvedTenantID(); got != 227 {
		t.Fatalf("cookie manager tenant ID = %d, want 227", got)
	}
}

func TestNewAPIClientWithStoreConfigLoadsWrappedCookiePayloadFromProvider(t *testing.T) {
	storeInfo := &StoreConfig{
		ID:       870,
		TenantID: 227,
	}
	provider := stubCookieProvider{
		result: &CookieLookupResult{
			TenantID: 227,
			CookieJSON: `{
				"cookies": [
					{"name":"sid","value":"abc","domain":"sso.geiwohuo.com","path":"/","sameSite":"Lax"}
				]
			}`,
		},
	}

	apiClient := NewAPIClientWithStoreConfig(870, storeInfo, provider)

	if !apiClient.HasCookies() {
		t.Fatal("expected wrapped cookies to be loaded from provider")
	}
	if got := apiClient.GetCookieCount(); got != 1 {
		t.Fatalf("cookie count = %d, want 1", got)
	}
}
