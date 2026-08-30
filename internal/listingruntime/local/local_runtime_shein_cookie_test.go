package local

import (
	"context"
	"testing"
)

type localRuntimeSheinCookieProviderStub struct {
	lookup        *SheinCookieLookupResult
	deleted       bool
	getStoreID    int64
	deleteStoreID int64
}

func (p *localRuntimeSheinCookieProviderStub) GetCookie(_ context.Context, storeID int64) (*SheinCookieLookupResult, error) {
	p.getStoreID = storeID
	return p.lookup, nil
}

func (p *localRuntimeSheinCookieProviderStub) DeleteCookie(_ context.Context, storeID int64) (bool, error) {
	p.deleteStoreID = storeID
	return p.deleted, nil
}

func TestLocalRuntimeSheinCookieCompatibility(t *testing.T) {
	provider := &localRuntimeSheinCookieProviderStub{
		lookup:  &SheinCookieLookupResult{CookieJSON: `[{"name":"sid"}]`, TenantID: 24},
		deleted: true,
	}
	runtime := &LocalRuntime{cookieProvider: provider}

	cookie, tenantID, err := runtime.GetSheinCookie(73)
	if err != nil || cookie != `[{"name":"sid"}]` || tenantID != 24 || provider.getStoreID != 73 {
		t.Fatalf("GetSheinCookie() = %q, %d, %v; provider store id = %d", cookie, tenantID, err, provider.getStoreID)
	}

	storeCookie, err := runtime.GetSheinStoreCookie(73)
	if err != nil || storeCookie != cookie {
		t.Fatalf("GetSheinStoreCookie() = %q, %v; want %q, nil", storeCookie, err, cookie)
	}

	deleted, err := runtime.DeleteSheinStoreCookie(73)
	if err != nil || !deleted || provider.deleteStoreID != 73 {
		t.Fatalf("DeleteSheinStoreCookie() = %v, %v; provider store id = %d", deleted, err, provider.deleteStoreID)
	}
}

func TestLocalRuntimeSheinCookieCompatibilityWithoutProvider(t *testing.T) {
	var runtime *LocalRuntime

	cookie, tenantID, err := runtime.GetSheinCookie(73)
	if err != nil || cookie != "" || tenantID != 0 {
		t.Fatalf("GetSheinCookie() = %q, %d, %v; want empty result", cookie, tenantID, err)
	}

	storeCookie, err := runtime.GetSheinStoreCookie(73)
	if err != nil || storeCookie != "" {
		t.Fatalf("GetSheinStoreCookie() = %q, %v; want empty result", storeCookie, err)
	}

	deleted, err := runtime.DeleteSheinStoreCookie(73)
	if err != nil || deleted {
		t.Fatalf("DeleteSheinStoreCookie() = %v, %v; want false, nil", deleted, err)
	}
}
